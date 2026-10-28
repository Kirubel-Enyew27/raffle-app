package application

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	auditdomain "github.com/raffle-app/backend/internal/audit/domain"
	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
)

type mockUserRepo struct {
	users   map[string]*domain.User
	findErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*domain.User)}
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	user, ok := m.users[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) SoftDelete(ctx context.Context, id string) error {
	delete(m.users, id)
	return nil
}

func (m *mockUserRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if u, ok := m.users[userID]; ok {
		u.PasswordHash = passwordHash
		return nil
	}
	return errors.New("user not found")
}

func (m *mockUserRepo) FindByName(ctx context.Context, name string) (*domain.User, error) {
	for _, u := range m.users {
		if u.FullName == name {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockUserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Phone == phone {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockUserRepo) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	for _, u := range m.users {
		if u.Phone == phone {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	for _, u := range m.users {
		if u.Email == email {
			return true, nil
		}
	}
	return false, nil
}

type mockAuditRepo struct {
	logs []auditdomain.AuditLog
}

func (m *mockAuditRepo) Create(ctx context.Context, log *auditdomain.AuditLog) error {
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) FindByID(ctx context.Context, id string) (*auditdomain.AuditLog, error) {
	return nil, nil
}

func (m *mockAuditRepo) FindByFilter(ctx context.Context, filter auditdomain.AuditLogFilter) ([]auditdomain.AuditLog, int, error) {
	return nil, 0, nil
}

func (m *mockAuditRepo) Count(ctx context.Context, filter auditdomain.AuditLogFilter) (int, error) {
	return 0, nil
}

func (m *mockAuditRepo) DeleteOlderThan(ctx context.Context, cutoffDate time.Time) (int64, error) {
	return 0, nil
}

func generateTestSecret() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}

func TestRegister(t *testing.T) {
	userRepo := newMockUserRepo()
	auditRepo := &mockAuditRepo{}
	auditService := auditapp.NewAuditService(auditRepo)
	secret := generateTestSecret()

	svc := NewIdentityService(userRepo, auditService, secret, 15*time.Minute, 7*24*time.Hour, newFakeTokenStore())

	user, err := svc.Register(context.Background(), "user-123", "test@example.com", "password123", "Test User", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}

	// Verify Audit Log
	if len(auditRepo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditRepo.logs))
	}
	log := auditRepo.logs[0]
	if log.Action != "register" {
		t.Errorf("expected action 'register', got %s", log.Action)
	}
	if *log.ActorID != "user-123" {
		t.Errorf("expected actor id 'user-123', got %s", *log.ActorID)
	}
}

func TestLogin(t *testing.T) {
	userRepo := newMockUserRepo()
	auditRepo := &mockAuditRepo{}
	auditService := auditapp.NewAuditService(auditRepo)
	secret := generateTestSecret()

	svc := NewIdentityService(userRepo, auditService, secret, 15*time.Minute, 7*24*time.Hour, newFakeTokenStore())

	// Register user first
	_, err := svc.Register(context.Background(), "user-123", "test@example.com", "password123", "Test User", "")
	if err != nil {
		t.Fatal(err)
	}

	// Clear register audit log
	auditRepo.logs = nil

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if session.AccessToken == "" {
		t.Error("expected non-empty token")
	}
	if session.RefreshToken == "" {
		t.Error("expected non-empty refresh token so a live session can renew itself")
	}
	if session.User.ID != "user-123" {
		t.Errorf("expected user id user-123, got %s", session.User.ID)
	}

	// Verify Audit Log
	if len(auditRepo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditRepo.logs))
	}
	log := auditRepo.logs[0]
	if log.Action != "login" {
		t.Errorf("expected action 'login', got %s", log.Action)
	}
	if *log.ActorID != "user-123" {
		t.Errorf("expected actor id 'user-123', got %s", *log.ActorID)
	}
}

func TestChangePassword(t *testing.T) {
	userRepo := newMockUserRepo()
	auditRepo := &mockAuditRepo{}
	auditService := auditapp.NewAuditService(auditRepo)
	secret := generateTestSecret()

	svc := NewIdentityService(userRepo, auditService, secret, 15*time.Minute, 7*24*time.Hour, newFakeTokenStore())

	// Register user first
	_, err := svc.Register(context.Background(), "user-123", "test@example.com", "password123", "Test User", "")
	if err != nil {
		t.Fatal(err)
	}

	// Clear register audit log
	auditRepo.logs = nil

	err = svc.ChangePassword(context.Background(), "user-123", "password123", "newpassword123")
	if err != nil {
		t.Fatalf("unexpected change password error: %v", err)
	}

	// Verify Audit Log
	if len(auditRepo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditRepo.logs))
	}
	log := auditRepo.logs[0]
	if log.Action != "password_change" {
		t.Errorf("expected action 'password_change', got %s", log.Action)
	}
	if *log.ActorID != "user-123" {
		t.Errorf("expected actor id 'user-123', got %s", *log.ActorID)
	}
}

// newAuthTestService registers one active user and returns the service, the
// signing secret and the fake repo so tests can mutate the account.
func newAuthTestService(t *testing.T, accessExpiry, refreshExpiry time.Duration) (*IdentityService, []byte, *mockUserRepo) {
	t.Helper()
	svc, secret, repo, _ := newAuthTestServiceWithStore(t, accessExpiry, refreshExpiry)
	return svc, secret, repo
}

// newAuthTestServiceWithStore also exposes the token store so revocation tests
// can inspect it and simulate a storage outage.
func newAuthTestServiceWithStore(t *testing.T, accessExpiry, refreshExpiry time.Duration) (*IdentityService, []byte, *mockUserRepo, *fakeTokenStore) {
	t.Helper()
	userRepo := newMockUserRepo()
	auditService := auditapp.NewAuditService(&mockAuditRepo{})
	secret := generateTestSecret()
	store := newFakeTokenStore()
	svc := NewIdentityService(userRepo, auditService, secret, accessExpiry, refreshExpiry, store)
	// Follow the store's clock so rotation windows can be advanced instantly.
	svc.now = func() time.Time { return store.now }
	if _, err := svc.Register(context.Background(), "user-123", "test@example.com", "password123", "Test User", ""); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return svc, secret, userRepo, store
}

func TestLoginIssuesTypedTokenPair(t *testing.T) {
	svc, secret, _ := newAuthTestService(t, 15*time.Minute, 7*24*time.Hour)

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	accessClaims, err := appjwt.ParseTokenHMAC(session.AccessToken, secret)
	if err != nil {
		t.Fatalf("access token should be usable for API calls: %v", err)
	}
	if accessClaims.UserID != "user-123" {
		t.Errorf("expected access token for user-123, got %s", accessClaims.UserID)
	}

	refreshClaims, err := appjwt.ParseRefreshTokenHMAC(session.RefreshToken, secret)
	if err != nil {
		t.Fatalf("refresh token should be a valid refresh token: %v", err)
	}
	if refreshClaims.UserID != "user-123" {
		t.Errorf("expected refresh token for user-123, got %s", refreshClaims.UserID)
	}

	// The refresh token must never be accepted as an access token.
	if _, err := appjwt.ParseTokenHMAC(session.RefreshToken, secret); !errors.Is(err, appjwt.ErrRefreshTokenUsedAsAccess) {
		t.Fatalf("expected refresh token to be rejected as access token, got %v", err)
	}
}

func TestRefreshSessionRenewsExpiredAccessToken(t *testing.T) {
	// A one-second access token that has just expired is the same state a player
	// is in after 15 minutes of continuous play.
	svc, secret, _ := newAuthTestService(t, time.Second, time.Hour)

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := appjwt.ParseTokenHMAC(session.AccessToken, secret); err == nil {
		t.Fatal("expected the access token to be expired for this test to be meaningful")
	}

	renewed, err := svc.RefreshSession(context.Background(), session.RefreshToken)
	if err != nil {
		t.Fatalf("refresh should succeed while the refresh token is valid: %v", err)
	}
	if renewed.AccessToken == "" {
		t.Fatal("expected a new access token")
	}
	if renewed.User == nil || renewed.User.ID != "user-123" {
		t.Fatalf("expected the refreshed session to belong to user-123, got %+v", renewed.User)
	}

	claims, err := appjwt.ParseTokenHMAC(renewed.AccessToken, secret)
	if err != nil {
		t.Fatalf("renewed access token should be valid: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected renewed token for user-123, got %s", claims.UserID)
	}
}

func TestRefreshSessionRejectsAccessToken(t *testing.T) {
	svc, _, _ := newAuthTestService(t, 15*time.Minute, time.Hour)

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	if _, err := svc.RefreshSession(context.Background(), session.AccessToken); !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized when refreshing with an access token, got %v", err)
	}
}

func TestRefreshSessionRejectsExpiredRefreshToken(t *testing.T) {
	svc, secret, _ := newAuthTestService(t, 15*time.Minute, time.Hour)

	expired, err := appjwt.GenerateRefreshTokenHMAC("user-123", "test@example.com", "user", secret, -time.Minute)
	if err != nil {
		t.Fatalf("failed to build expired refresh token: %v", err)
	}

	if _, err := svc.RefreshSession(context.Background(), expired); !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for an expired refresh token, got %v", err)
	}
}

func TestRefreshSessionRejectsBannedUser(t *testing.T) {
	svc, _, userRepo := newAuthTestService(t, 15*time.Minute, time.Hour)

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	userRepo.users["user-123"].IsBanned = true
	if _, err := svc.RefreshSession(context.Background(), session.RefreshToken); !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for a banned user, got %v", err)
	}
}

func TestRefreshSessionRejectsDeletedUser(t *testing.T) {
	svc, _, userRepo := newAuthTestService(t, 15*time.Minute, time.Hour)

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	delete(userRepo.users, "user-123")
	if _, err := svc.RefreshSession(context.Background(), session.RefreshToken); !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for a deleted user, got %v", err)
	}
}

func TestRefreshSessionRejectsInvalidTokens(t *testing.T) {
	svc, _, _ := newAuthTestService(t, 15*time.Minute, time.Hour)

	foreign, err := appjwt.GenerateRefreshTokenHMAC("user-123", "test@example.com", "user", []byte("some-other-secret"), time.Hour)
	if err != nil {
		t.Fatalf("failed to build foreign token: %v", err)
	}

	for name, token := range map[string]string{
		"empty":        "",
		"garbage":      "not-a-jwt",
		"wrong secret": foreign,
	} {
		if _, err := svc.RefreshSession(context.Background(), token); !errors.Is(err, apperrors.ErrUnauthorized) {
			t.Errorf("%s: expected ErrUnauthorized, got %v", name, err)
		}
	}
}

func TestRefreshSessionReportsDatabaseOutageAsInternal(t *testing.T) {
	svc, _, userRepo := newAuthTestService(t, 15*time.Minute, time.Hour)

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	userRepo.findErr = errors.New("connection refused")
	_, err = svc.RefreshSession(context.Background(), session.RefreshToken)
	if !errors.Is(err, apperrors.ErrInternal) {
		t.Fatalf("expected a transient internal error, got %v", err)
	}
	if errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatal("a database outage must never be reported as an expired session")
	}

	// Once the database recovers the same session renews normally.
	userRepo.findErr = nil
	if _, err := svc.RefreshSession(context.Background(), session.RefreshToken); err != nil {
		t.Fatalf("session should survive the outage: %v", err)
	}
}

func TestRefreshSessionRotatesRefreshToken(t *testing.T) {
	svc, _, _ := newAuthTestService(t, 15*time.Minute, time.Hour)

	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	renewed, err := svc.RefreshSession(context.Background(), session.RefreshToken)
	if err != nil {
		t.Fatalf("unexpected refresh error: %v", err)
	}
	if renewed.RefreshToken == "" {
		t.Fatal("expected a rotated refresh token")
	}

	// The rotated token must keep working so a long match is never interrupted.
	if _, err := svc.RefreshSession(context.Background(), renewed.RefreshToken); err != nil {
		t.Fatalf("rotated refresh token should be usable: %v", err)
	}
}
