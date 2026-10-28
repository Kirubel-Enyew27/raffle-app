package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

// defaultRefreshExpiry mirrors config's JWT_REFRESH_EXPIRY default and keeps
// callers that only know about access tokens (older tests, embedders) working.
const defaultRefreshExpiry = 7 * 24 * time.Hour

// rotationGrace is how long an already-exchanged refresh token may still be
// presented. Two tabs refreshing at the same instant both send the same token;
// the second one receives the session the first one got instead of a forced
// re-login. Past the window the token is simply dead.
const rotationGrace = time.Minute

type IdentityService struct {
	userRepo         domain.UserRepository
	auditService     *auditapp.AuditService
	jwtSecret        []byte
	jwtExpiry        time.Duration
	jwtRefreshExpiry time.Duration
	tokenStore       domain.RefreshTokenStore
	// now is the clock used for rotation windows; tests swap it to avoid
	// sleeping through a real grace period.
	now func() time.Time
}

// Session is the credential pair returned by login and refresh. AccessToken is
// short lived; RefreshToken is only accepted by RefreshSession.
type Session struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	User         *domain.User
}

func NewIdentityService(userRepo domain.UserRepository, auditService *auditapp.AuditService, jwtSecret []byte, jwtExpiry, refreshExpiry time.Duration, tokenStore domain.RefreshTokenStore) *IdentityService {
	if refreshExpiry <= 0 {
		refreshExpiry = defaultRefreshExpiry
	}
	return &IdentityService{
		userRepo:         userRepo,
		auditService:     auditService,
		jwtSecret:        jwtSecret,
		jwtExpiry:        jwtExpiry,
		jwtRefreshExpiry: refreshExpiry,
		tokenStore:       tokenStore,
		now:              time.Now,
	}
}

// issueSession mints a fresh access/refresh pair for an already authenticated
// user. Both tokens are signed with the same secret but carry a distinct type
// claim, so a refresh token can never authorise an API call.
func (s *IdentityService) issueSession(user *domain.User) (*Session, error) {
	if user == nil {
		return nil, apperrors.ErrUnauthorized
	}
	access, err := appjwt.GenerateTokenHMAC(user.ID, user.Email, user.Role, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		return nil, apperrors.ErrInternal
	}
	refresh, err := appjwt.GenerateRefreshTokenHMAC(user.ID, user.Email, user.Role, s.jwtSecret, s.jwtRefreshExpiry)
	if err != nil {
		return nil, apperrors.ErrInternal
	}
	return &Session{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.jwtExpiry.Seconds()),
		User:         user,
	}, nil
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s *IdentityService) Register(ctx context.Context, id, email, password, fullName, phone string) (*domain.User, error) {
	if (email == "" && phone == "") || password == "" {
		return nil, apperrors.ErrValidationFailed
	}

	if id == "" {
		id = generateUUID()
	}

	if email != "" {
		exists, err := s.userRepo.ExistsByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperrors.ErrConflict
		}
	}

	if phone != "" {
		exists, err := s.userRepo.ExistsByPhone(ctx, phone)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperrors.ErrConflict
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.ErrInternal
	}

	user := &domain.User{
		ID:           id,
		Email:        email,
		Phone:        phone,
		FullName:     fullName,
		PasswordHash: string(hashed),
		Role:         "user",
		IsBanned:     false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Record audit log
	_ = s.auditService.Record(ctx, &user.ID, "user", "register", "user", &user.ID, "", nil, nil)

	return user, nil
}

func (s *IdentityService) Login(ctx context.Context, identifier, password string) (*Session, error) {
	if identifier == "" || password == "" {
		return nil, apperrors.ErrValidationFailed
	}

	// Try email first, then phone
	user, err := s.userRepo.FindByEmail(ctx, identifier)
	if err != nil {
		user, err = s.userRepo.FindByPhone(ctx, identifier)
		if err != nil {
			return nil, errors.New("invalid credentials")
		}
	}

	if user.IsBanned {
		return nil, errors.New("user is banned")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	session, err := s.issueSession(user)
	if err != nil {
		return nil, err
	}

	// Record audit log
	_ = s.auditService.Record(ctx, &user.ID, user.Role, "login", "auth", &user.ID, "", nil, nil)

	return session, nil
}

// loadSessionUser re-reads the account behind a token so a banned or deleted
// user cannot keep renewing a session. A storage failure is reported as
// internal (transient) rather than unauthorised: telling the client its session
// is dead over a database blip is exactly how players get thrown out of a match.
func (s *IdentityService) loadSessionUser(ctx context.Context, userID string) (*domain.User, error) {
	if userID == "" {
		return nil, apperrors.ErrUnauthorized
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrUnauthorized
		}
		return nil, apperrors.ErrInternal
	}
	if user == nil || user.IsBanned {
		return nil, apperrors.ErrUnauthorized
	}
	return user, nil
}

// RefreshSession exchanges a valid refresh token for a new token pair. The user
// row is re-read so a banned or deleted account cannot renew its session; every
// other credential failure (bad signature, wrong token type, expiry, revoked) is
// reported as unauthorised for the client to re-authenticate.
//
// Each refresh token is single use: exchanging it retires it and records the new
// pair for a short grace window, so duplicated requests are answered instead of
// being treated as failures.
func (s *IdentityService) RefreshSession(ctx context.Context, refreshToken string) (*Session, error) {
	if refreshToken == "" {
		return nil, apperrors.ErrUnauthorized
	}

	claims, err := appjwt.ParseRefreshTokenHMAC(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	tokenID := domain.SessionTokenID(refreshToken, claims.ID)
	now := s.now()

	// One read decides the token's fate. A retry or a second tab may present the
	// token its sibling just exchanged: answer with that same session instead of
	// logging the player out mid-match.
	record, err := s.tokenStore.Get(ctx, tokenID)
	if err != nil {
		return nil, apperrors.ErrInternal
	}
	if record != nil && record.Revoked {
		return nil, apperrors.ErrUnauthorized
	}
	reused := record.Reusable(now)
	if record != nil && reused == nil {
		// Already exchanged and the grace window has closed: the token is spent.
		return nil, apperrors.ErrUnauthorized
	}

	user, err := s.loadSessionUser(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	if reused != nil {
		return &Session{
			AccessToken:  reused.AccessToken,
			RefreshToken: reused.RefreshToken,
			ExpiresIn:    reused.ExpiresIn,
			User:         user,
		}, nil
	}

	session, err := s.issueSession(user)
	if err != nil {
		return nil, err
	}

	until := now.Add(s.jwtRefreshExpiry)
	if claims.ExpiresAt != nil {
		until = claims.ExpiresAt.Time
	}
	if err := s.tokenStore.RecordExchange(ctx, tokenID, until, domain.RotatedSession{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		ExpiresIn:    session.ExpiresIn,
	}, now.Add(rotationGrace)); err != nil {
		// The new pair is discarded: leaving an exchange unrecorded would keep
		// the presented token replayable.
		return nil, apperrors.ErrInternal
	}

	_ = s.auditService.Record(ctx, &user.ID, user.Role, "token_refresh", "auth", &user.ID, "", nil, nil)
	return session, nil
}

// Logout revokes a refresh token so it can never be exchanged again. Logging out
// is idempotent: an unknown, malformed or already-expired token is not an error,
// because the client is dropping it either way. Only a storage failure is
// returned to the caller.
func (s *IdentityService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	claims, err := appjwt.ParseRefreshTokenHMAC(refreshToken, s.jwtSecret)
	if err != nil {
		return nil
	}

	until := s.now().Add(s.jwtRefreshExpiry)
	if claims.ExpiresAt != nil {
		until = claims.ExpiresAt.Time
	}
	if err := s.tokenStore.Revoke(ctx, domain.SessionTokenID(refreshToken, claims.ID), until); err != nil {
		return apperrors.ErrInternal
	}

	_ = s.auditService.Record(ctx, &claims.UserID, claims.Role, "logout", "auth", &claims.UserID, "", nil, nil)
	return nil
}

func (s *IdentityService) UpdateProfile(ctx context.Context, userID, fullName, phone string, avatarURL ...string) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if fullName != "" {
		user.FullName = fullName
	}
	if phone != "" {
		user.Phone = phone
	}
	if len(avatarURL) > 0 && avatarURL[0] != "" {
		user.AvatarURL = avatarURL[0]
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	_ = s.auditService.Record(ctx, &userID, user.Role, "profile_update", "user", &userID, "", nil, nil)
	return user, nil
}

func (s *IdentityService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if userID == "" || oldPassword == "" || newPassword == "" {
		return apperrors.ErrValidationFailed
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		// If FindByID is not fully implemented in DB repo, handle it or query from email if needed.
		// Wait, userRepo.FindByID returns nil, nil in current boilerplate! Let's verify that.
		// Let's implement FindByID in postgres.go of identity repo.
		return err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword))
	if err != nil {
		return errors.New("invalid old password")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperrors.ErrInternal
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, string(hashed)); err != nil {
		return err
	}

	// Record audit log
	oldVal := "password_masked"
	newVal := "password_changed"
	_ = s.auditService.Record(ctx, &userID, user.Role, "password_change", "user", &userID, "", &oldVal, &newVal)

	return nil
}
