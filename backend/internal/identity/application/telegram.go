package application

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrTelegramUnavailable means the bot token is not configured on this
	// server, so initData cannot be verified.
	ErrTelegramUnavailable = errors.New("telegram sign-in is not available")
	// ErrTelegramAuthInvalid covers a missing, malformed, forged or expired
	// launch payload. The client cannot tell these apart, and should not.
	ErrTelegramAuthInvalid = errors.New("this telegram sign-in could not be verified; reopen KWIK from Telegram")
	// ErrTelegramLinkPasswordRequired is returned for accounts that have no
	// local password (created through Telegram or Google). The session alone is
	// not enough to attach another identity to them.
	ErrTelegramLinkPasswordRequired = errors.New("this account has no password set; sign in another way to connect telegram")
	// ErrTelegramAlreadyLinked means this KWIK account has a different Telegram
	// identity attached already.
	ErrTelegramAlreadyLinked = errors.New("a telegram account is already connected")
	// ErrTelegramLinkConflict means this Telegram account belongs to another
	// KWIK user.
	ErrTelegramLinkConflict = errors.New("that telegram account is already connected to another user")
)

// TelegramInitDataVerifier validates a raw WebApp initData string. The
// implementation lives in infrastructure/telegram and never trusts
// initDataUnsafe.
type TelegramInitDataVerifier interface {
	Verify(initData string) (*domain.TelegramProfile, error)
}

// SetTelegramAuth enables Telegram Mini App authentication. A nil verifier
// leaves the feature off and every Telegram endpoint reports it as unavailable.
func (s *IdentityService) SetTelegramAuth(verifier TelegramInitDataVerifier, identities domain.OAuthIdentityRepository) {
	s.telegramVerifier = verifier
	if identities != nil {
		s.oauthIdentities = identities
	}
}

// TelegramEnabled reports whether initData can be verified on this instance.
func (s *IdentityService) TelegramEnabled() bool {
	return s.telegramVerifier != nil && s.oauthIdentities != nil
}

// LoginWithTelegram verifies the launch payload and signs the Telegram user in.
//
// The Telegram user id is the only identity key. An account is provisioned the
// first time that id is seen; an existing account is never adopted based on a
// display name, username or email. Ownership of an existing KWIK account is
// proven only in the explicit link flow (LinkTelegram).
func (s *IdentityService) LoginWithTelegram(ctx context.Context, initData string) (*Session, error) {
	if !s.TelegramEnabled() {
		return nil, ErrTelegramUnavailable
	}

	profile, err := s.telegramVerifier.Verify(initData)
	if err != nil || profile == nil || profile.ID <= 0 {
		return nil, ErrTelegramAuthInvalid
	}

	subject := strconv.FormatInt(profile.ID, 10)
	linked, err := s.oauthIdentities.FindByProviderUserID(ctx, domain.ProviderTelegram, subject)
	switch {
	case err == nil && linked != nil:
		user, fetchErr := s.userRepo.FindByID(ctx, linked.UserID)
		if fetchErr != nil {
			if errors.Is(fetchErr, apperrors.ErrNotFound) {
				return nil, apperrors.ErrUnauthorized
			}
			return nil, apperrors.ErrInternal
		}
		if user == nil || user.IsBanned || user.DeletedAt != nil {
			return nil, apperrors.ErrForbidden
		}
		_ = s.auditService.Record(ctx, &user.ID, user.Role, "telegram_login", "auth", &user.ID, "", nil, nil)
		return s.issueSession(user)
	case err != nil && !errors.Is(err, apperrors.ErrNotFound):
		return nil, apperrors.ErrInternal
	}

	now := time.Now()
	user := &domain.User{
		ID:        generateUUID(),
		FullName:  profile.DisplayName(),
		AvatarURL: profile.PhotoURL,
		// Provision exactly like registration: least privilege. Nothing from
		// Telegram can grant admin rights.
		Role:      "user",
		IsBanned:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	identity := &domain.OAuthIdentity{
		ID:             generateUUID(),
		UserID:         user.ID,
		Provider:       domain.ProviderTelegram,
		ProviderUserID: subject,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.oauthIdentities.CreateUserWithIdentity(ctx, user, identity); err != nil {
		if errors.Is(err, apperrors.ErrConflict) {
			// Two launches for the same Telegram user raced. The winner's
			// account is the authoritative one.
			winner, findErr := s.oauthIdentities.FindByProviderUserID(ctx, domain.ProviderTelegram, subject)
			if findErr == nil && winner != nil {
				linkedUser, fetchErr := s.userRepo.FindByID(ctx, winner.UserID)
				if fetchErr == nil && linkedUser != nil {
					return s.issueSession(linkedUser)
				}
			}
			return nil, apperrors.ErrInternal
		}
		return nil, err
	}

	_ = s.auditService.Record(ctx, &user.ID, user.Role, "telegram_register", "auth", &user.ID, "", nil, nil)
	return s.issueSession(user)
}

// TelegramLinkStatus reports whether the signed-in account has Telegram
// connected.
func (s *IdentityService) TelegramLinkStatus(ctx context.Context, userID string) (bool, error) {
	if !s.TelegramEnabled() {
		return false, ErrTelegramUnavailable
	}
	if userID == "" {
		return false, apperrors.ErrUnauthorized
	}
	identity, err := s.oauthIdentities.FindByUserID(ctx, domain.ProviderTelegram, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return false, nil
		}
		return false, apperrors.ErrInternal
	}
	return identity != nil, nil
}

// LinkTelegram attaches the verified Telegram identity to the signed-in KWIK
// account. The session proves control of KWIK and the fresh initData proves
// control of Telegram; the current password is required as well so a stolen
// session cannot silently add a permanent way back in. Accounts created through
// a social provider have no password to confirm with, so they cannot use this
// flow (they already sign in through their provider).
func (s *IdentityService) LinkTelegram(ctx context.Context, userID, initData, password string) error {
	if !s.TelegramEnabled() {
		return ErrTelegramUnavailable
	}
	if userID == "" {
		return apperrors.ErrUnauthorized
	}
	if strings.TrimSpace(password) == "" {
		return apperrors.ErrValidationFailed
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return apperrors.ErrUnauthorized
		}
		return apperrors.ErrInternal
	}
	if user == nil || user.IsBanned || user.DeletedAt != nil {
		return apperrors.ErrForbidden
	}
	if user.PasswordHash == "" {
		return ErrTelegramLinkPasswordRequired
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return ErrInvalidPassword
	}

	profile, err := s.telegramVerifier.Verify(initData)
	if err != nil || profile == nil || profile.ID <= 0 {
		return ErrTelegramAuthInvalid
	}
	subject := strconv.FormatInt(profile.ID, 10)

	current, err := s.oauthIdentities.FindByUserID(ctx, domain.ProviderTelegram, userID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return apperrors.ErrInternal
	}
	if current != nil {
		if current.ProviderUserID == subject {
			return nil
		}
		return ErrTelegramAlreadyLinked
	}

	other, err := s.oauthIdentities.FindByProviderUserID(ctx, domain.ProviderTelegram, subject)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return apperrors.ErrInternal
	}
	if other != nil {
		if other.UserID == userID {
			return nil
		}
		return ErrTelegramLinkConflict
	}

	now := time.Now()
	identity := &domain.OAuthIdentity{
		ID:             generateUUID(),
		UserID:         userID,
		Provider:       domain.ProviderTelegram,
		ProviderUserID: subject,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.oauthIdentities.Create(ctx, identity); err != nil {
		if errors.Is(err, apperrors.ErrConflict) {
			return ErrTelegramAlreadyLinked
		}
		return err
	}

	_ = s.auditService.Record(ctx, &userID, user.Role, "telegram_link", "auth", &userID, "", nil, nil)
	return nil
}
