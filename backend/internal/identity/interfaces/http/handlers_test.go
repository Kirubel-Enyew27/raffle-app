package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	auditdomain "github.com/raffle-app/backend/internal/audit/domain"
	"github.com/raffle-app/backend/internal/identity/application"
	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"github.com/raffle-app/backend/pkg/middleware"
)

type fakeUserRepo struct {
	users   map[string]*domain.User
	findErr error
}

func (f *fakeUserRepo) Create(ctx context.Context, user *domain.User) error {
	f.users[user.ID] = user
	return nil
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	user, ok := f.users[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return user, nil
}

func (f *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (f *fakeUserRepo) FindByName(ctx context.Context, name string) (*domain.User, error) {
	return nil, apperrors.ErrNotFound
}

func (f *fakeUserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	for _, u := range f.users {
		if u.Phone == phone {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (f *fakeUserRepo) Update(ctx context.Context, user *domain.User) error {
	f.users[user.ID] = user
	return nil
}

func (f *fakeUserRepo) SoftDelete(ctx context.Context, id string) error {
	delete(f.users, id)
	return nil
}

func (f *fakeUserRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if u, ok := f.users[userID]; ok {
		u.PasswordHash = passwordHash
		return nil
	}
	return apperrors.ErrNotFound
}

func (f *fakeUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, err := f.FindByEmail(ctx, email)
	return err == nil, nil
}

func (f *fakeUserRepo) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	_, err := f.FindByPhone(ctx, phone)
	return err == nil, nil
}

type fakeAuditRepo struct{}

func (fakeAuditRepo) Create(ctx context.Context, log *auditdomain.AuditLog) error { return nil }
func (fakeAuditRepo) FindByID(ctx context.Context, id string) (*auditdomain.AuditLog, error) {
	return nil, nil
}
func (fakeAuditRepo) FindByFilter(ctx context.Context, filter auditdomain.AuditLogFilter) ([]auditdomain.AuditLog, int, error) {
	return nil, 0, nil
}
func (fakeAuditRepo) Count(ctx context.Context, filter auditdomain.AuditLogFilter) (int, error) {
	return 0, nil
}
func (fakeAuditRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

type sessionPayload struct {
	Code string `json:"code"`
	Data struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"data"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newAuthRouter(t *testing.T, accessExpiry, refreshExpiry time.Duration) (*gin.Engine, string, *fakeUserRepo) {
	t.Helper()
	r, secret, repo, _ := newAuthRouterWithStore(t, accessExpiry, refreshExpiry)
	return r, secret, repo
}

// newAuthRouterWithStore also returns the token store so tests can simulate a
// revocation-store outage.
func newAuthRouterWithStore(t *testing.T, accessExpiry, refreshExpiry time.Duration) (*gin.Engine, string, *fakeUserRepo, *fakeTokenStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	secret := "identity-handlers-test-secret-1234567890"
	viper.Set("JWT_SECRET", secret)

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	repo := &fakeUserRepo{users: map[string]*domain.User{
		"user-1": {
			ID:           "user-1",
			Email:        "player@example.com",
			PasswordHash: string(hash),
			Role:         "user",
		},
	}}

	store := newFakeTokenStore()
	svc := application.NewIdentityService(repo, auditapp.NewAuditService(fakeAuditRepo{}), []byte(secret), accessExpiry, refreshExpiry, store)

	r := gin.New()
	api := r.Group("/api/v1")
	RegisterIdentityRoutes(api, NewIdentityHandler(svc))
	api.GET("/protected", middleware.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(nethttp.StatusOK, gin.H{"code": "success", "data": gin.H{"user_id": c.GetString("user_id")}})
	})
	return r, secret, repo, store
}

func doJSON(t *testing.T, r *gin.Engine, method, path, bearer string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func login(t *testing.T, r *gin.Engine) sessionPayload {
	t.Helper()
	w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"identifier": "player@example.com",
		"password":   "password123",
	})
	if w.Code != nethttp.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var payload sessionPayload
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	return payload
}

func TestLoginReturnsRefreshTokenAndProtectsRefreshTokens(t *testing.T) {
	r, _, _ := newAuthRouter(t, 15*time.Minute, time.Hour)

	session := login(t, r)
	if session.Data.Token == "" {
		t.Fatal("expected an access token")
	}
	if session.Data.RefreshToken == "" {
		t.Fatal("expected a refresh token")
	}
	if session.Data.ExpiresIn != 900 {
		t.Errorf("expected expires_in 900, got %d", session.Data.ExpiresIn)
	}

	// The access token authorises API calls.
	if w := doJSON(t, r, nethttp.MethodGet, "/api/v1/protected", session.Data.Token, nil); w.Code != nethttp.StatusOK {
		t.Fatalf("access token should be accepted: %d %s", w.Code, w.Body.String())
	}

	// The refresh token must not.
	if w := doJSON(t, r, nethttp.MethodGet, "/api/v1/protected", session.Data.RefreshToken, nil); w.Code != nethttp.StatusUnauthorized {
		t.Fatalf("refresh token must not authorise API calls, got %d", w.Code)
	}

	// An access token must not be exchanged for a new session.
	w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": session.Data.Token})
	if w.Code != nethttp.StatusUnauthorized {
		t.Fatalf("expected 401 when refreshing with an access token, got %d %s", w.Code, w.Body.String())
	}
	var errPayload errorPayload
	_ = json.Unmarshal(w.Body.Bytes(), &errPayload)
	if errPayload.Code != "SESSION_EXPIRED" {
		t.Errorf("expected SESSION_EXPIRED code, got %q", errPayload.Code)
	}
}

func TestRefreshRenewsExpiredAccessTokenWithoutNewLogin(t *testing.T) {
	// A one-second access token that has just lapsed mirrors the 15-minute mark
	// during a live match, where the player used to be logged out.
	r, _, _ := newAuthRouter(t, time.Second, time.Hour)

	session := login(t, r)
	time.Sleep(1100 * time.Millisecond)
	if w := doJSON(t, r, nethttp.MethodGet, "/api/v1/protected", session.Data.Token, nil); w.Code != nethttp.StatusUnauthorized {
		t.Fatalf("expected the expired access token to be rejected, got %d", w.Code)
	}

	w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": session.Data.RefreshToken})
	if w.Code != nethttp.StatusOK {
		t.Fatalf("refresh should succeed: %d %s", w.Code, w.Body.String())
	}
	var renewed sessionPayload
	if err := json.Unmarshal(w.Body.Bytes(), &renewed); err != nil {
		t.Fatalf("failed to decode refresh response: %v", err)
	}
	if renewed.Data.Token == "" || renewed.Data.User.ID != "user-1" {
		t.Fatalf("unexpected refresh payload: %+v", renewed.Data)
	}

	if w := doJSON(t, r, nethttp.MethodGet, "/api/v1/protected", renewed.Data.Token, nil); w.Code != nethttp.StatusOK {
		t.Fatalf("renewed access token should be accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestRefreshRejectsExpiredOrRevokedSessions(t *testing.T) {
	t.Run("expired refresh token", func(t *testing.T) {
		r, secret, _ := newAuthRouter(t, 15*time.Minute, time.Hour)
		expired, err := appjwt.GenerateRefreshTokenHMAC("user-1", "player@example.com", "user", []byte(secret), -time.Minute)
		if err != nil {
			t.Fatalf("failed to build expired refresh token: %v", err)
		}

		w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": expired})
		if w.Code != nethttp.StatusUnauthorized {
			t.Fatalf("expected 401, got %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("banned user", func(t *testing.T) {
		r, _, repo := newAuthRouter(t, 15*time.Minute, time.Hour)
		session := login(t, r)
		repo.users["user-1"].IsBanned = true

		w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": session.Data.RefreshToken})
		if w.Code != nethttp.StatusUnauthorized {
			t.Fatalf("expected 401 for a banned user, got %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("deleted user", func(t *testing.T) {
		r, _, repo := newAuthRouter(t, 15*time.Minute, time.Hour)
		session := login(t, r)
		delete(repo.users, "user-1")

		w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": session.Data.RefreshToken})
		if w.Code != nethttp.StatusUnauthorized {
			t.Fatalf("expected 401 for a deleted user, got %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		r, _, _ := newAuthRouter(t, 15*time.Minute, time.Hour)
		w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": "not-a-jwt"})
		if w.Code != nethttp.StatusUnauthorized {
			t.Fatalf("expected 401, got %d %s", w.Code, w.Body.String())
		}
	})
}

func TestRefreshValidatesPayload(t *testing.T) {
	r, _, _ := newAuthRouter(t, 15*time.Minute, time.Hour)

	if w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{}); w.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected 400 for a missing refresh token, got %d %s", w.Code, w.Body.String())
	}

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/refresh", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

func TestRefreshKeepsSessionWhenDatabaseFails(t *testing.T) {
	r, _, repo := newAuthRouter(t, 15*time.Minute, time.Hour)
	session := login(t, r)

	repo.findErr = errors.New("connection refused")
	w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": session.Data.RefreshToken})
	if w.Code != nethttp.StatusInternalServerError {
		t.Fatalf("a database outage must be a 500, not a session expiry: %d %s", w.Code, w.Body.String())
	}

	// The same session still works once the database is reachable again.
	repo.findErr = nil
	w = doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": session.Data.RefreshToken})
	if w.Code != nethttp.StatusOK {
		t.Fatalf("session should survive the outage, got %d %s", w.Code, w.Body.String())
	}
}

func TestLoginStillRejectsBadPassword(t *testing.T) {
	r, _, _ := newAuthRouter(t, 15*time.Minute, time.Hour)

	w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"identifier": "player@example.com",
		"password":   "wrong-password",
	})
	if w.Code != nethttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("refresh_token")) {
		t.Fatal("failed login must not leak a refresh token")
	}
}
