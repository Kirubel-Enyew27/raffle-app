package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

const (
	// oauthStateTTL bounds how long a user may sit on Google's consent screen.
	oauthStateTTL = 10 * time.Minute
	// loginHandoffTTL is deliberately short: the SPA redeems the session
	// immediately after the redirect.
	loginHandoffTTL = 60 * time.Second
)

var (
	// ErrGoogleUnavailable means credentials are not configured on this server.
	ErrGoogleUnavailable = errors.New("google sign-in is not available")
	// ErrGoogleDeclined means the user cancelled at Google.
	ErrGoogleDeclined = errors.New("google sign-in was cancelled")
	// ErrOAuthStateInvalid covers unknown, expired and replayed state values.
	ErrOAuthStateInvalid = errors.New("this sign-in attempt has expired, please try again")
	// ErrGoogleAuthFailed hides exchange/validation detail from the client.
	ErrGoogleAuthFailed = errors.New("google sign-in could not be completed")
	// ErrGoogleEmailUnverified means Google did not vouch for the address.
	ErrGoogleEmailUnverified = errors.New("the google account has no verified email address")
	// ErrGoogleAccountExists is returned when an existing local account owns
	// the same email. Linking is never automatic: the user must sign in with
	// the credentials they already have and connect Google from their profile.
	ErrGoogleAccountExists = errors.New("an account with this email already exists")
	// ErrInvalidPassword means the password given to start an account link did
	// not match the signed-in user's password.
	ErrInvalidPassword = errors.New("the password you entered is incorrect")
	// ErrGoogleLinkEmailMismatch means the Google account owns a different
	// address than the local account, so it is not proof of ownership.
	ErrGoogleLinkEmailMismatch = errors.New("the google account uses a different email address")
	// ErrGoogleLinkEmailRequired means the local account has no email to match.
	ErrGoogleLinkEmailRequired = errors.New("add an email address to your account before connecting google")
	// ErrGoogleAlreadyLinked means this account already has a Google identity.
	ErrGoogleAlreadyLinked = errors.New("a google account is already connected")
	// ErrGoogleLinkConflict means that Google account belongs to another user.
	ErrGoogleLinkConflict = errors.New("that google account is already connected to another user")
)

// GoogleCallbackResult describes what a completed callback did. On failure the
// result is still returned when the state was valid, so the caller knows which
// page to send the user back to.
type GoogleCallbackResult struct {
	// HandoffCode is the one-time code the SPA exchanges for a session. Empty
	// for link attempts, which never mint a session.
	HandoffCode string
	RedirectTo  string
	// LinkMode is true when this callback belongs to an account-linking
	// attempt started from a signed-in session.
	LinkMode bool
}

// SetGoogleOAuth enables Google sign-in. Passing a nil provider leaves the
// feature off and every Google endpoint reports that it is unavailable.
func (s *IdentityService) SetGoogleOAuth(
	provider domain.GoogleProvider,
	identities domain.OAuthIdentityRepository,
	states domain.OAuthStateStore,
	handoffs domain.LoginHandoffStore,
) {
	s.googleProvider = provider
	s.oauthIdentities = identities
	s.oauthStates = states
	s.loginHandoffs = handoffs
}

// GoogleEnabled reports whether Google sign-in can be used on this instance.
func (s *IdentityService) GoogleEnabled() bool {
	return s.googleProvider != nil && s.oauthIdentities != nil && s.oauthStates != nil && s.loginHandoffs != nil
}

// beginGoogleAuthorization creates a single-use authorization attempt and
// returns the URL to send the browser to. The PKCE verifier and nonce stay in
// Redis; the browser only receives the opaque state value.
func (s *IdentityService) beginGoogleAuthorization(ctx context.Context, attempt domain.OAuthAttempt) (string, error) {
	state, err := randomURLToken(32)
	if err != nil {
		return "", apperrors.ErrInternal
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		return "", apperrors.ErrInternal
	}
	verifier, err := randomURLToken(32)
	if err != nil {
		return "", apperrors.ErrInternal
	}

	attempt.CodeVerifier = verifier
	attempt.Nonce = nonce
	if err := s.oauthStates.Save(ctx, state, attempt, oauthStateTTL); err != nil {
		// Without the attempt stored, the callback can never be validated.
		// Failing closed is the only safe answer.
		return "", apperrors.ErrInternal
	}

	return s.googleProvider.AuthorizationURL(state, nonce, pkceChallengeFor(verifier)), nil
}

// StartGoogleLogin starts a sign-in (or first-time registration) with Google.
func (s *IdentityService) StartGoogleLogin(ctx context.Context, redirectTo string) (string, error) {
	if !s.GoogleEnabled() {
		return "", ErrGoogleUnavailable
	}
	return s.beginGoogleAuthorization(ctx, domain.OAuthAttempt{
		RedirectTo: SafeRedirectPath(redirectTo),
	})
}

// StartGoogleLink starts connecting a Google account to the signed-in user.
//
// The caller must be authenticated *and* re-enter the current password. The
// state record is bound to that user id, so the callback can only ever affect
// the account that started the attempt: a link attempt cannot be replayed into
// someone else's account, and a sign-in attempt can never turn into a link.
func (s *IdentityService) StartGoogleLink(ctx context.Context, userID, password, redirectTo string) (string, error) {
	if !s.GoogleEnabled() {
		return "", ErrGoogleUnavailable
	}
	if userID == "" {
		return "", apperrors.ErrUnauthorized
	}
	if password == "" {
		return "", apperrors.ErrValidationFailed
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return "", apperrors.ErrUnauthorized
		}
		return "", apperrors.ErrInternal
	}
	if user == nil || user.IsBanned || user.DeletedAt != nil {
		return "", apperrors.ErrForbidden
	}
	// Accounts created through Google have no password to confirm with; they
	// are already connected anyway.
	if user.PasswordHash == "" {
		return "", ErrGoogleAlreadyLinked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidPassword
	}

	// Cheap checks before sending the user to Google, so an impossible link
	// fails immediately instead of after a round trip.
	if strings.TrimSpace(user.Email) == "" {
		return "", ErrGoogleLinkEmailRequired
	}
	linked, err := s.oauthIdentities.FindByUserID(ctx, domain.ProviderGoogle, userID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return "", apperrors.ErrInternal
	}
	if linked != nil {
		return "", ErrGoogleAlreadyLinked
	}

	return s.beginGoogleAuthorization(ctx, domain.OAuthAttempt{
		RedirectTo: SafeRedirectPath(redirectTo),
		LinkUserID: userID,
	})
}

// GoogleLinkStatus reports whether the account has a Google identity attached.
func (s *IdentityService) GoogleLinkStatus(ctx context.Context, userID string) (bool, error) {
	if !s.GoogleEnabled() {
		return false, ErrGoogleUnavailable
	}
	if userID == "" {
		return false, apperrors.ErrUnauthorized
	}

	identity, err := s.oauthIdentities.FindByUserID(ctx, domain.ProviderGoogle, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return false, nil
		}
		return false, apperrors.ErrInternal
	}
	return identity != nil, nil
}

// CompleteGoogleCallback validates the callback, then either signs the user in
// or — for a link attempt — attaches Google to the account that started it.
func (s *IdentityService) CompleteGoogleCallback(ctx context.Context, state, code, oauthError string) (*GoogleCallbackResult, error) {
	if !s.GoogleEnabled() {
		return nil, ErrGoogleUnavailable
	}
	if state == "" {
		return nil, ErrOAuthStateInvalid
	}

	attempt, err := s.oauthStates.Consume(ctx, state)
	if err != nil {
		return nil, apperrors.ErrInternal
	}
	if attempt == nil {
		// Unknown, expired or already used: a replayed callback lands here.
		return nil, ErrOAuthStateInvalid
	}

	result := &GoogleCallbackResult{
		RedirectTo: attempt.RedirectTo,
		LinkMode:   attempt.LinkUserID != "",
	}

	if oauthError != "" {
		// The state was still consumed above so it cannot be reused later.
		return result, ErrGoogleDeclined
	}
	if code == "" {
		return result, ErrGoogleAuthFailed
	}

	profile, err := s.googleProvider.Exchange(ctx, code, attempt.CodeVerifier, attempt.Nonce)
	if err != nil {
		return result, ErrGoogleAuthFailed
	}

	if result.LinkMode {
		// No session is minted: the user is already signed in, and this flow
		// only adds an authentication method to their account.
		if err := s.linkGoogleIdentity(ctx, attempt.LinkUserID, profile); err != nil {
			return result, err
		}
		return result, nil
	}

	user, err := s.loginWithGoogleProfile(ctx, profile)
	if err != nil {
		return result, err
	}

	session, err := s.issueSession(user)
	if err != nil {
		return result, err
	}

	handoff, err := randomURLToken(32)
	if err != nil {
		return result, apperrors.ErrInternal
	}
	err = s.loginHandoffs.Save(ctx, handoff, domain.LoginHandoff{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		ExpiresIn:    session.ExpiresIn,
		User:         session.User,
		RedirectTo:   attempt.RedirectTo,
	}, loginHandoffTTL)
	if err != nil {
		return result, apperrors.ErrInternal
	}

	result.HandoffCode = handoff
	return result, nil
}

// ExchangeGoogleHandoff redeems the handoff code for the session it holds. The
// code is consumed atomically, so it works exactly once.
func (s *IdentityService) ExchangeGoogleHandoff(ctx context.Context, code string) (*Session, string, error) {
	if !s.GoogleEnabled() || code == "" {
		return nil, "", ErrOAuthStateInvalid
	}

	handoff, err := s.loginHandoffs.Consume(ctx, code)
	if err != nil {
		return nil, "", apperrors.ErrInternal
	}
	if handoff == nil || handoff.AccessToken == "" || handoff.User == nil {
		return nil, "", ErrOAuthStateInvalid
	}

	return &Session{
		AccessToken:  handoff.AccessToken,
		RefreshToken: handoff.RefreshToken,
		ExpiresIn:    handoff.ExpiresIn,
		User:         handoff.User,
	}, handoff.RedirectTo, nil
}

// loginWithGoogleProfile maps a verified Google identity onto a local account.
//
// Google's `sub` is the only identity key. An email match against an existing
// password account is deliberately NOT used to link: whoever controls the
// Google account would otherwise inherit the local account (and its wallet) on
// the strength of an email string alone. Ownership is proven in the explicit
// link flow instead (StartGoogleLink).
func (s *IdentityService) loginWithGoogleProfile(ctx context.Context, profile *domain.GoogleProfile) (*domain.User, error) {
	if profile == nil || profile.Subject == "" {
		return nil, ErrGoogleAuthFailed
	}
	if profile.Email == "" || !profile.EmailVerified {
		return nil, ErrGoogleEmailUnverified
	}

	linked, err := s.oauthIdentities.FindByProviderUserID(ctx, domain.ProviderGoogle, profile.Subject)
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
		_ = s.auditService.Record(ctx, &user.ID, user.Role, "google_login", "auth", &user.ID, "", nil, nil)
		return user, nil
	case err != nil && !errors.Is(err, apperrors.ErrNotFound):
		return nil, apperrors.ErrInternal
	}

	// First Google sign-in for this subject. Refuse to adopt an email that
	// already belongs to an account with another sign-in method.
	existing, err := s.userRepo.FindByEmail(ctx, profile.Email)
	if err == nil && existing != nil {
		_ = s.auditService.Record(ctx, &existing.ID, existing.Role, "google_link_conflict", "auth", &existing.ID, "", nil, nil)
		return nil, ErrGoogleAccountExists
	}
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return nil, apperrors.ErrInternal
	}

	user := &domain.User{
		ID:        generateUUID(),
		Email:     profile.Email,
		FullName:  profile.Name,
		AvatarURL: profile.Picture,
		// Role assignment follows normal registration: the least privileged
		// role, never anything derived from the Google profile.
		Role:      "user",
		IsBanned:  false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	identity := &domain.OAuthIdentity{
		ID:             generateUUID(),
		UserID:         user.ID,
		Provider:       domain.ProviderGoogle,
		ProviderUserID: profile.Subject,
		ProviderEmail:  profile.Email,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.oauthIdentities.CreateUserWithIdentity(ctx, user, identity); err != nil {
		if errors.Is(err, apperrors.ErrConflict) {
			// Another request for the same Google account won the race.
			if winner, findErr := s.oauthIdentities.FindByProviderUserID(ctx, domain.ProviderGoogle, profile.Subject); findErr == nil && winner != nil {
				if linkedUser, fetchErr := s.userRepo.FindByID(ctx, winner.UserID); fetchErr == nil && linkedUser != nil {
					return linkedUser, nil
				}
			}
			return nil, ErrGoogleAccountExists
		}
		return nil, err
	}

	_ = s.auditService.Record(ctx, &user.ID, user.Role, "google_register", "auth", &user.ID, "", nil, nil)
	return user, nil
}

// linkGoogleIdentity attaches a verified Google identity to an account that
// already exists and whose ownership was proven when the attempt was started.
//
// The Google account must own the same verified address as the local account.
// That is what stops a stolen session from attaching an attacker-controlled
// mailbox and keeping access after a password change.
func (s *IdentityService) linkGoogleIdentity(ctx context.Context, userID string, profile *domain.GoogleProfile) error {
	if profile == nil || profile.Subject == "" {
		return ErrGoogleAuthFailed
	}
	if profile.Email == "" || !profile.EmailVerified {
		return ErrGoogleEmailUnverified
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

	accountEmail := strings.ToLower(strings.TrimSpace(user.Email))
	if accountEmail == "" {
		return ErrGoogleLinkEmailRequired
	}
	if accountEmail != profile.Email {
		return ErrGoogleLinkEmailMismatch
	}

	current, err := s.oauthIdentities.FindByUserID(ctx, domain.ProviderGoogle, userID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return apperrors.ErrInternal
	}
	if current != nil {
		if current.ProviderUserID == profile.Subject {
			// Already connected to this same Google account: idempotent.
			return nil
		}
		return ErrGoogleAlreadyLinked
	}

	other, err := s.oauthIdentities.FindByProviderUserID(ctx, domain.ProviderGoogle, profile.Subject)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return apperrors.ErrInternal
	}
	if other != nil {
		if other.UserID == userID {
			return nil
		}
		return ErrGoogleLinkConflict
	}

	now := time.Now()
	identity := &domain.OAuthIdentity{
		ID:             generateUUID(),
		UserID:         userID,
		Provider:       domain.ProviderGoogle,
		ProviderUserID: profile.Subject,
		ProviderEmail:  profile.Email,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.oauthIdentities.Create(ctx, identity); err != nil {
		if errors.Is(err, apperrors.ErrConflict) {
			// Lost a race with another attempt; report it rather than guessing.
			return ErrGoogleAlreadyLinked
		}
		return err
	}

	_ = s.auditService.Record(ctx, &userID, user.Role, "google_link", "auth", &userID, "", nil, nil)
	return nil
}

// SafeRedirectPath keeps only same-origin relative paths, so a crafted
// `redirect` parameter cannot turn the post-login navigation into an open
// redirect.
func SafeRedirectPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return ""
	}
	// "/\evil.example" is treated as protocol-relative by some browsers.
	if strings.HasPrefix(value, `/\`) {
		return ""
	}
	return value
}

func randomURLToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallengeFor derives the S256 code challenge for a verifier.
func pkceChallengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
