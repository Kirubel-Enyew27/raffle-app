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
)

type mockUserRepo struct {
	users map[string]*domain.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*domain.User)}
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, errors.New("user not found")
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

	svc := NewIdentityService(userRepo, auditService, secret, 15*time.Minute)

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

	svc := NewIdentityService(userRepo, auditService, secret, 15*time.Minute)

	// Register user first
	_, err := svc.Register(context.Background(), "user-123", "test@example.com", "password123", "Test User", "")
	if err != nil {
		t.Fatal(err)
	}

	// Clear register audit log
	auditRepo.logs = nil

	token, user, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if user.ID != "user-123" {
		t.Errorf("expected user id user-123, got %s", user.ID)
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

	svc := NewIdentityService(userRepo, auditService, secret, 15*time.Minute)

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
