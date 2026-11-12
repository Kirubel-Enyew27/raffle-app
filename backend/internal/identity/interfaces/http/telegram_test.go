package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	identityapp "github.com/raffle-app/backend/internal/identity/application"
	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/middleware"
)

type stubTelegramVerifier struct{}

func (stubTelegramVerifier) Verify(initData string) (*domain.TelegramProfile, error) {
	if initData != "valid-init-data" {
		return nil, apperrors.ErrUnauthorized
	}
	return &domain.TelegramProfile{ID: 555001, FirstName: "Tigist", LastName: "B."}, nil
}

type memoryIdentityRepo struct {
	users      *fakeUserRepo
	identities map[string]*domain.OAuthIdentity
}

func newMemoryIdentityRepo(users *fakeUserRepo) *memoryIdentityRepo {
	return &memoryIdentityRepo{users: users, identities: map[string]*domain.OAuthIdentity{}}
}

func (r *memoryIdentityRepo) FindByProviderUserID(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error) {
	if identity, ok := r.identities[provider+":"+subject]; ok {
		return identity, nil
	}
	return nil, apperrors.ErrNotFound
}

func (r *memoryIdentityRepo) FindByUserID(ctx context.Context, provider, userID string) (*domain.OAuthIdentity, error) {
	for _, identity := range r.identities {
		if identity.Provider == provider && identity.UserID == userID {
			return identity, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (r *memoryIdentityRepo) CreateUserWithIdentity(ctx context.Context, user *domain.User, identity *domain.OAuthIdentity) error {
	r.users.users[user.ID] = user
	r.identities[identity.Provider+":"+identity.ProviderUserID] = identity
	return nil
}

func (r *memoryIdentityRepo) Create(ctx context.Context, identity *domain.OAuthIdentity) error {
	r.identities[identity.Provider+":"+identity.ProviderUserID] = identity
	return nil
}

func newTelegramRouter(t *testing.T) (*gin.Engine, *fakeUserRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// AuthMiddleware verifies with this secret; the service signs with it.
	viper.Set("JWT_SECRET", "telegram-handler-test-secret-123456789")

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserRepo{users: map[string]*domain.User{
		"user-1": {ID: "user-1", Email: "player@example.com", PasswordHash: string(hash), Role: "user"},
	}}
	svc := identityapp.NewIdentityService(
		users,
		auditapp.NewAuditService(fakeAuditRepo{}),
		[]byte("telegram-handler-test-secret-123456789"),
		15*time.Minute,
		time.Hour,
		newFakeTokenStore(),
	)
	svc.SetTelegramAuth(stubTelegramVerifier{}, newMemoryIdentityRepo(users))

	r := gin.New()
	api := r.Group("/api/v1")
	RegisterIdentityRoutes(api, NewIdentityHandler(svc))
	RegisterTelegramAuthRoutes(api, NewTelegramAuthHandler(svc), nil)
	api.GET("/protected", middleware.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": "success"})
	})
	return r, users
}

func TestTelegramLoginCreatesSessionAndReusesAccount(t *testing.T) {
	r, _ := newTelegramRouter(t)

	first := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram", "", map[string]string{"init_data": "valid-init-data"})
	if first.Code != http.StatusOK {
		t.Fatalf("telegram login failed: %d %s", first.Code, first.Body.String())
	}
	var session sessionPayload
	if err := json.Unmarshal(first.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.Data.Token == "" || session.Data.RefreshToken == "" || session.Data.User.ID == "" {
		t.Fatalf("incomplete session: %+v", session.Data)
	}

	if w := doJSON(t, r, http.MethodGet, "/api/v1/protected", session.Data.Token, nil); w.Code != http.StatusOK {
		t.Fatalf("telegram session should authorise API calls: %d", w.Code)
	}

	second := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram", "", map[string]string{"init_data": "valid-init-data"})
	if second.Code != http.StatusOK {
		t.Fatalf("second telegram login failed: %d", second.Code)
	}
	var again sessionPayload
	if err := json.Unmarshal(second.Body.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again.Data.User.ID != session.Data.User.ID {
		t.Fatalf("duplicate account on second launch: %s vs %s", again.Data.User.ID, session.Data.User.ID)
	}
}

func TestTelegramLoginRejectsInvalidAndMissingInitData(t *testing.T) {
	r, _ := newTelegramRouter(t)

	forged := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram", "", map[string]string{"init_data": "forged"})
	if forged.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for forged initData, got %d", forged.Code)
	}
	var payload errorPayload
	_ = json.Unmarshal(forged.Body.Bytes(), &payload)
	if payload.Code != "TELEGRAM_AUTH_INVALID" {
		t.Errorf("unexpected error code %q", payload.Code)
	}

	missing := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram", "", map[string]string{})
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing initData, got %d", missing.Code)
	}
}

func TestTelegramLinkRequiresAuthenticationAndPassword(t *testing.T) {
	r, _ := newTelegramRouter(t)

	if w := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram/link", "", map[string]string{
		"init_data": "valid-init-data",
		"password":  "password123",
	}); w.Code != http.StatusUnauthorized {
		t.Fatalf("linking must require a KWIK session, got %d", w.Code)
	}

	login := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram", "", map[string]string{"init_data": "valid-init-data"})
	var session sessionPayload
	_ = json.Unmarshal(login.Body.Bytes(), &session)

	// A Telegram-created account has no password, so it cannot begin a link.
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram/link", session.Data.Token, map[string]string{
		"init_data": "valid-init-data",
		"password":  "password123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected PASSWORD_REQUIRED for a social account, got %d %s", w.Code, w.Body.String())
	}

	status := doJSON(t, r, http.MethodGet, "/api/v1/auth/telegram/link", session.Data.Token, nil)
	if status.Code != http.StatusOK {
		t.Fatalf("link status failed: %d", status.Code)
	}
	var statusPayload struct {
		Data struct {
			Linked bool `json:"linked"`
		} `json:"data"`
	}
	_ = json.Unmarshal(status.Body.Bytes(), &statusPayload)
	if !statusPayload.Data.Linked {
		t.Fatal("expected the provisioned account to be linked")
	}
}

func TestTelegramLinkConnectsPasswordAccount(t *testing.T) {
	r, users := newTelegramRouter(t)
	// Sign in as the existing password account using the normal flow.
	login := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"identifier": "player@example.com",
		"password":   "password123",
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", login.Code, login.Body.String())
	}
	var session sessionPayload
	_ = json.Unmarshal(login.Body.Bytes(), &session)

	bad := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram/link", session.Data.Token, map[string]string{
		"init_data": "valid-init-data",
		"password":  "wrong",
	})
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("expected the password check to fail, got %d", bad.Code)
	}

	ok := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram/link", session.Data.Token, map[string]string{
		"init_data": "valid-init-data",
		"password":  "password123",
	})
	if ok.Code != http.StatusOK {
		t.Fatalf("link failed: %d %s", ok.Code, ok.Body.String())
	}

	// The Telegram identity now resolves to the existing account.
	again := doJSON(t, r, http.MethodPost, "/api/v1/auth/telegram", "", map[string]string{"init_data": "valid-init-data"})
	var linked sessionPayload
	_ = json.Unmarshal(again.Body.Bytes(), &linked)
	if linked.Data.User.ID != "user-1" {
		t.Fatalf("expected the linked account user-1, got %s", linked.Data.User.ID)
	}
	if len(users.users) != 1 {
		t.Fatalf("linking must not create an extra account, got %d users", len(users.users))
	}
}
