package application

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

type fakeTelegramVerifier struct {
	profiles map[string]*domain.TelegramProfile
	err      error
}

func (v *fakeTelegramVerifier) Verify(initData string) (*domain.TelegramProfile, error) {
	if v.err != nil {
		return nil, v.err
	}
	profile, ok := v.profiles[initData]
	if !ok {
		return nil, errors.New("invalid init data")
	}
	return profile, nil
}

type telegramFixture struct {
	svc        *IdentityService
	verifier   *fakeTelegramVerifier
	identities *googleIdentityRepo
	users      *googleUserRepo
}

func newTelegramFixture(t *testing.T) *telegramFixture {
	t.Helper()
	users := newGoogleUserRepo()
	identities := newGoogleIdentityRepo(users)
	verifier := &fakeTelegramVerifier{profiles: map[string]*domain.TelegramProfile{
		"valid-telegram-init": {
			ID:        900001,
			FirstName: "Ada",
			LastName:  "Lovelace",
			Username:  "ada",
		},
		"second-telegram-init": {
			ID:        900002,
			FirstName: "Grace",
			Username:  "grace",
		},
	}}

	svc := NewIdentityService(users, auditapp.NewAuditService(&mockAuditRepo{}), generateTestSecret(), 15*time.Minute, time.Hour, newFakeTokenStore())
	svc.SetTelegramAuth(verifier, identities)
	return &telegramFixture{svc: svc, verifier: verifier, identities: identities, users: users}
}

func (f *telegramFixture) link(t *testing.T, userID string, telegramID int64) *domain.OAuthIdentity {
	t.Helper()
	identity := &domain.OAuthIdentity{
		ID:             "identity-" + strconv.FormatInt(telegramID, 10),
		UserID:         userID,
		Provider:       domain.ProviderTelegram,
		ProviderUserID: strconv.FormatInt(telegramID, 10),
	}
	f.identities.identities[identity.Provider+":"+identity.ProviderUserID] = identity
	return identity
}

func TestTelegramLoginProvisionsAccountOnFirstLaunch(t *testing.T) {
	f := newTelegramFixture(t)

	session, err := f.svc.LoginWithTelegram(context.Background(), "valid-telegram-init")
	if err != nil {
		t.Fatalf("telegram login: %v", err)
	}
	if session.User.Role != "user" {
		t.Fatalf("telegram must never grant elevated roles, got %q", session.User.Role)
	}
	if session.User.FullName != "Ada Lovelace" {
		t.Errorf("unexpected full name %q", session.User.FullName)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatal("expected a full KWIK session")
	}

	identity, err := f.identities.FindByProviderUserID(context.Background(), domain.ProviderTelegram, "900001")
	if err != nil || identity == nil {
		t.Fatalf("expected the telegram identity to be stored: %v", err)
	}
	if identity.UserID != session.User.ID {
		t.Errorf("identity points at %s, expected %s", identity.UserID, session.User.ID)
	}
}

func TestTelegramLoginReusesTheSameAccount(t *testing.T) {
	f := newTelegramFixture(t)

	first, err := f.svc.LoginWithTelegram(context.Background(), "valid-telegram-init")
	if err != nil {
		t.Fatalf("first login: %v", err)
	}
	second, err := f.svc.LoginWithTelegram(context.Background(), "valid-telegram-init")
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	if second.User.ID != first.User.ID {
		t.Fatalf("a second launch created a duplicate account: %s vs %s", second.User.ID, first.User.ID)
	}
	if len(f.users.users) != 1 {
		t.Fatalf("expected exactly one account, got %d", len(f.users.users))
	}
}

func TestTelegramLoginKeepsExistingRoleAndWalletIdentity(t *testing.T) {
	f := newTelegramFixture(t)

	// An administrator who linked Telegram earlier keeps the admin role. The
	// role comes from the database row, never from Telegram data.
	f.users.users["admin-1"] = &domain.User{ID: "admin-1", FullName: "Admin", Role: "admin"}
	f.link(t, "admin-1", 900001)

	session, err := f.svc.LoginWithTelegram(context.Background(), "valid-telegram-init")
	if err != nil {
		t.Fatalf("telegram login: %v", err)
	}
	if session.User.ID != "admin-1" || session.User.Role != "admin" {
		t.Fatalf("expected the existing admin account, got %+v", session.User)
	}
}

func TestTelegramLoginRejectsBannedAndUnknownIdentities(t *testing.T) {
	f := newTelegramFixture(t)
	f.users.users["banned-1"] = &domain.User{ID: "banned-1", Role: "user", IsBanned: true}
	f.link(t, "banned-1", 900001)

	if _, err := f.svc.LoginWithTelegram(context.Background(), "valid-telegram-init"); !errors.Is(err, apperrors.ErrForbidden) {
		t.Fatalf("expected a banned account to be rejected, got %v", err)
	}

	if _, err := f.svc.LoginWithTelegram(context.Background(), "forged"); !errors.Is(err, ErrTelegramAuthInvalid) {
		t.Fatalf("expected invalid initData to be rejected, got %v", err)
	}
}

func TestTelegramLoginDistinguishesTelegramUsers(t *testing.T) {
	f := newTelegramFixture(t)

	one, err := f.svc.LoginWithTelegram(context.Background(), "valid-telegram-init")
	if err != nil {
		t.Fatal(err)
	}
	two, err := f.svc.LoginWithTelegram(context.Background(), "second-telegram-init")
	if err != nil {
		t.Fatal(err)
	}
	if one.User.ID == two.User.ID {
		t.Fatal("two telegram users must not share an account")
	}
}

func TestTelegramLoginUnavailableWithoutConfiguration(t *testing.T) {
	users := newGoogleUserRepo()
	svc := NewIdentityService(users, auditapp.NewAuditService(&mockAuditRepo{}), generateTestSecret(), time.Minute, time.Hour, newFakeTokenStore())
	if _, err := svc.LoginWithTelegram(context.Background(), "valid-telegram-init"); !errors.Is(err, ErrTelegramUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestLinkTelegramRequiresPasswordAndAttachesIdentity(t *testing.T) {
	f := newTelegramFixture(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	f.users.users["user-1"] = &domain.User{ID: "user-1", Email: "player@example.com", PasswordHash: string(hash), Role: "user"}

	if err := f.svc.LinkTelegram(context.Background(), "user-1", "valid-telegram-init", ""); !errors.Is(err, apperrors.ErrValidationFailed) {
		t.Fatalf("expected a password requirement, got %v", err)
	}
	if err := f.svc.LinkTelegram(context.Background(), "user-1", "valid-telegram-init", "wrong"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected the wrong password to be rejected, got %v", err)
	}
	if err := f.svc.LinkTelegram(context.Background(), "user-1", "forged", "password123"); !errors.Is(err, ErrTelegramAuthInvalid) {
		t.Fatalf("expected forged initData to be rejected, got %v", err)
	}

	if err := f.svc.LinkTelegram(context.Background(), "user-1", "valid-telegram-init", "password123"); err != nil {
		t.Fatalf("link: %v", err)
	}

	linked, err := f.svc.TelegramLinkStatus(context.Background(), "user-1")
	if err != nil || !linked {
		t.Fatalf("expected telegram to be linked, got %v %v", linked, err)
	}

	// Linking the same account again is idempotent.
	if err := f.svc.LinkTelegram(context.Background(), "user-1", "valid-telegram-init", "password123"); err != nil {
		t.Fatalf("expected idempotent link, got %v", err)
	}

	// Signing in through Telegram now reaches the existing account, with its
	// wallet and history untouched.
	session, err := f.svc.LoginWithTelegram(context.Background(), "valid-telegram-init")
	if err != nil {
		t.Fatalf("telegram login after link: %v", err)
	}
	if session.User.ID != "user-1" {
		t.Fatalf("expected user-1, got %s", session.User.ID)
	}
}

func TestLinkTelegramRefusesIdentitiesOwnedBySomeoneElse(t *testing.T) {
	f := newTelegramFixture(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	f.users.users["user-1"] = &domain.User{ID: "user-1", PasswordHash: string(hash), Role: "user"}
	f.users.users["user-2"] = &domain.User{ID: "user-2", PasswordHash: string(hash), Role: "user"}
	f.link(t, "user-2", 900001)

	if err := f.svc.LinkTelegram(context.Background(), "user-1", "valid-telegram-init", "password123"); !errors.Is(err, ErrTelegramLinkConflict) {
		t.Fatalf("expected a link conflict, got %v", err)
	}
}

func TestLinkTelegramKeepsTheFirstIdentity(t *testing.T) {
	f := newTelegramFixture(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	f.users.users["user-1"] = &domain.User{ID: "user-1", PasswordHash: string(hash), Role: "user"}
	f.link(t, "user-1", 900002)

	if err := f.svc.LinkTelegram(context.Background(), "user-1", "valid-telegram-init", "password123"); !errors.Is(err, ErrTelegramAlreadyLinked) {
		t.Fatalf("expected the account to keep its existing telegram identity, got %v", err)
	}
}

func TestLinkTelegramRequiresPasswordOnSocialAccounts(t *testing.T) {
	f := newTelegramFixture(t)
	f.users.users["social-1"] = &domain.User{ID: "social-1", Role: "user"}

	if err := f.svc.LinkTelegram(context.Background(), "social-1", "valid-telegram-init", "anything"); !errors.Is(err, ErrTelegramLinkPasswordRequired) {
		t.Fatalf("expected password-required, got %v", err)
	}
}
