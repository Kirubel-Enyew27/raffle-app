package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/identity/application"
	"github.com/raffle-app/backend/internal/identity/domain"
	identitygoogle "github.com/raffle-app/backend/internal/identity/infrastructure/google"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/middleware"
)

const (
	testGoogleClientID = "test-client-id"
	testFrontendURL    = "http://localhost:5173"
)

type fakeOAuthIdentityRepo struct {
	users      *fakeUserRepo
	identities map[string]*domain.OAuthIdentity
	findErr    error
	createErr  error
}

func newFakeOAuthIdentityRepo(users *fakeUserRepo) *fakeOAuthIdentityRepo {
	return &fakeOAuthIdentityRepo{users: users, identities: map[string]*domain.OAuthIdentity{}}
}

func identityKey(provider, subject string) string { return provider + ":" + subject }

func (f *fakeOAuthIdentityRepo) FindByProviderUserID(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	identity, ok := f.identities[identityKey(provider, subject)]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return identity, nil
}

func (f *fakeOAuthIdentityRepo) FindByUserID(ctx context.Context, provider, userID string) (*domain.OAuthIdentity, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	for _, identity := range f.identities {
		if identity.Provider == provider && identity.UserID == userID {
			return identity, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (f *fakeOAuthIdentityRepo) Create(ctx context.Context, identity *domain.OAuthIdentity) error {
	if f.createErr != nil {
		return f.createErr
	}
	// Mirror both unique constraints: one identity per account, and one
	// account per provider subject.
	for _, existing := range f.identities {
		if existing.Provider == identity.Provider && existing.UserID == identity.UserID {
			return apperrors.ErrConflict
		}
	}
	if _, ok := f.identities[identityKey(identity.Provider, identity.ProviderUserID)]; ok {
		return apperrors.ErrConflict
	}
	f.identities[identityKey(identity.Provider, identity.ProviderUserID)] = identity
	return nil
}

func (f *fakeOAuthIdentityRepo) CreateUserWithIdentity(ctx context.Context, user *domain.User, identity *domain.OAuthIdentity) error {
	if f.createErr != nil {
		return f.createErr
	}
	// Mirror the database's unique constraints.
	if _, err := f.users.FindByEmail(ctx, user.Email); err == nil {
		return apperrors.ErrConflict
	}
	if _, ok := f.identities[identityKey(identity.Provider, identity.ProviderUserID)]; ok {
		return apperrors.ErrConflict
	}
	f.users.users[user.ID] = user
	f.identities[identityKey(identity.Provider, identity.ProviderUserID)] = identity
	return nil
}

type fakeOAuthStateStore struct {
	attempts map[string]domain.OAuthAttempt
	saveErr  error
	readErr  error
}

func newFakeOAuthStateStore() *fakeOAuthStateStore {
	return &fakeOAuthStateStore{attempts: map[string]domain.OAuthAttempt{}}
}

func (f *fakeOAuthStateStore) Save(ctx context.Context, state string, attempt domain.OAuthAttempt, ttl time.Duration) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.attempts[state] = attempt
	return nil
}

func (f *fakeOAuthStateStore) Consume(ctx context.Context, state string) (*domain.OAuthAttempt, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	attempt, ok := f.attempts[state]
	if !ok {
		return nil, nil
	}
	delete(f.attempts, state)
	return &attempt, nil
}

type fakeHandoffStore struct {
	handoffs map[string]domain.LoginHandoff
	saveErr  error
	readErr  error
}

func newFakeHandoffStore() *fakeHandoffStore {
	return &fakeHandoffStore{handoffs: map[string]domain.LoginHandoff{}}
}

func (f *fakeHandoffStore) Save(ctx context.Context, code string, handoff domain.LoginHandoff, ttl time.Duration) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.handoffs[code] = handoff
	return nil
}

func (f *fakeHandoffStore) Consume(ctx context.Context, code string) (*domain.LoginHandoff, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	handoff, ok := f.handoffs[code]
	if !ok {
		return nil, nil
	}
	delete(f.handoffs, code)
	return &handoff, nil
}

type googleTestEnv struct {
	router     *gin.Engine
	google     *fakeGoogle
	users      *fakeUserRepo
	identities *fakeOAuthIdentityRepo
	states     *fakeOAuthStateStore
	handoffs   *fakeHandoffStore
	service    *application.IdentityService
}

// newGoogleTestEnv wires the real HTTP handlers, the real Google provider
// (pointed at the fake Google) and in-memory repositories.
func newGoogleTestEnv(t *testing.T, configure bool) *googleTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	secret := "google-sign-in-test-secret-123456789"
	viper.Set("JWT_SECRET", secret)

	users := &fakeUserRepo{users: map[string]*domain.User{}}
	identities := newFakeOAuthIdentityRepo(users)
	states := newFakeOAuthStateStore()
	handoffs := newFakeHandoffStore()

	svc := application.NewIdentityService(users, auditapp.NewAuditService(fakeAuditRepo{}), []byte(secret), 15*time.Minute, time.Hour, newFakeTokenStore())

	env := &googleTestEnv{
		google:     newFakeGoogle(t),
		users:      users,
		identities: identities,
		states:     states,
		handoffs:   handoffs,
		service:    svc,
	}

	provider := identitygoogle.NewProvider(identitygoogle.Config{
		ClientID:     testGoogleClientID,
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/api/v1/auth/google/callback",
		Endpoints: identitygoogle.Endpoints{
			AuthURL:  env.google.server.URL + "/auth",
			TokenURL: env.google.server.URL + "/token",
			JWKSURL:  env.google.server.URL + "/certs",
		},
		HTTPClient: env.google.server.Client(),
	})
	if configure {
		svc.SetGoogleOAuth(provider, identities, states, handoffs)
	} else {
		svc.SetGoogleOAuth(nil, identities, states, handoffs)
	}

	r := gin.New()
	api := r.Group("/api/v1")
	RegisterIdentityRoutes(api, NewIdentityHandler(svc))
	RegisterGoogleAuthRoutes(api, NewGoogleAuthHandler(svc, testFrontendURL), nil)
	api.GET("/protected", middleware.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"user_id": c.GetString("user_id")}})
	})
	env.router = r
	return env
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// beginSignIn performs the /auth/google redirect and records the state/nonce
// that Google would have received.
func (e *googleTestEnv) beginSignIn(t *testing.T, query string) (state, nonce string) {
	t.Helper()
	w := get(t, e.router, "/api/v1/auth/google"+query)
	if w.Code != http.StatusFound {
		t.Fatalf("expected a redirect to Google, got %d: %s", w.Code, w.Body.String())
	}
	state, nonce = stateFromLocation(t, w.Header().Get("Location"))
	if state == "" || nonce == "" {
		t.Fatalf("authorization URL is missing state/nonce: %s", w.Header().Get("Location"))
	}
	// Google would echo the nonce back inside the ID token.
	e.google.set(func(g *fakeGoogle) { g.nonce = nonce })
	return state, nonce
}

func (e *googleTestEnv) callback(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	return get(t, e.router, "/api/v1/auth/google/callback"+query)
}

func (e *googleTestEnv) signIn(t *testing.T) (handoff, redirect string) {
	t.Helper()
	state, _ := e.beginSignIn(t, "")
	w := e.callback(t, "?state="+state+"&code=test-authorization-code")
	if w.Code != http.StatusFound {
		t.Fatalf("expected callback redirect, got %d", w.Code)
	}
	handoff, redirect = handoffFromLocation(t, w.Header().Get("Location"))
	if handoff == "" {
		t.Fatalf("callback did not issue a handoff code: %s", w.Header().Get("Location"))
	}
	return handoff, redirect
}

func (e *googleTestEnv) exchange(t *testing.T, code string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, e.router, http.MethodPost, "/api/v1/auth/google/exchange", "", map[string]string{"code": code})
}

type googleSessionPayload struct {
	Code string `json:"code"`
	Data struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		Redirect     string `json:"redirect"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"user"`
	} `json:"data"`
}

func decodeSession(t *testing.T, w *httptest.ResponseRecorder) googleSessionPayload {
	t.Helper()
	var payload googleSessionPayload
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode session: %v (%s)", err, w.Body.String())
	}
	return payload
}

func redirectedError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	location := w.Header().Get("Location")
	if !strings.HasPrefix(location, testFrontendURL+"/login?error=") {
		t.Fatalf("expected a redirect to the login page with an error, got %q", location)
	}
	return strings.TrimPrefix(location, testFrontendURL+"/login?error=")
}

func TestGoogleSignUpCreatesUserAndUsableSession(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	handoff, _ := env.signIn(t)

	w := env.exchange(t, handoff)
	if w.Code != http.StatusOK {
		t.Fatalf("exchange failed: %d %s", w.Code, w.Body.String())
	}
	session := decodeSession(t, w)
	if session.Data.Token == "" || session.Data.RefreshToken == "" {
		t.Fatalf("expected a full token pair, got %+v", session.Data)
	}
	if session.Data.User.Role != "user" {
		t.Errorf("google registrations must receive the default role, got %q", session.Data.User.Role)
	}
	if session.Data.User.Email != "player@example.com" {
		t.Errorf("unexpected email %q", session.Data.User.Email)
	}

	// The issued access token must work on protected routes.
	if w := doJSON(t, env.router, http.MethodGet, "/api/v1/protected", session.Data.Token, nil); w.Code != http.StatusOK {
		t.Fatalf("access token rejected on a protected route: %d %s", w.Code, w.Body.String())
	}

	// The refresh token must work too: this is the same session mechanism as
	// password login, not a parallel one.
	w = doJSON(t, env.router, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": session.Data.RefreshToken})
	if w.Code != http.StatusOK {
		t.Fatalf("refresh failed for a google session: %d %s", w.Code, w.Body.String())
	}

	// Exactly one account and one provider link were created.
	if len(env.users.users) != 1 {
		t.Fatalf("expected one user, got %d", len(env.users.users))
	}
	if len(env.identities.identities) != 1 {
		t.Fatalf("expected one linked identity, got %d", len(env.identities.identities))
	}
	for _, user := range env.users.users {
		if user.PasswordHash != "" {
			t.Error("a google-only account must not be given a local password")
		}
	}
}

func TestGoogleSignInReusesLinkedAccount(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	first, _ := env.signIn(t)
	if w := env.exchange(t, first); w.Code != http.StatusOK {
		t.Fatalf("first sign-in failed: %d %s", w.Code, w.Body.String())
	}
	second, _ := env.signIn(t)
	if w := env.exchange(t, second); w.Code != http.StatusOK {
		t.Fatalf("second sign-in failed: %d %s", w.Code, w.Body.String())
	}

	if len(env.users.users) != 1 {
		t.Fatalf("repeat sign-in must not create a second account, got %d", len(env.users.users))
	}
}

func TestGoogleSignInRefusesToLinkExistingPasswordAccount(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	// An email/password account already owns this address.
	env.users.users["existing"] = &domain.User{
		ID:           "existing",
		Email:        "player@example.com",
		Role:         "user",
		PasswordHash: "$2a$10$notarealhash",
	}

	state, _ := env.beginSignIn(t, "")
	w := env.callback(t, "?state="+state+"&code=test-authorization-code")
	if reason := redirectedError(t, w); reason != "account_exists" {
		t.Fatalf("expected account_exists, got %q", reason)
	}

	if len(env.users.users) != 1 {
		t.Fatalf("no account must be created on an email conflict, got %d users", len(env.users.users))
	}
	if len(env.identities.identities) != 0 {
		t.Fatal("a matching email must never be auto-linked to a google identity")
	}
}

func TestGoogleCallbackRejectsUnknownAndReplayedState(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	if reason := redirectedError(t, env.callback(t, "?state=forged&code=x")); reason != "invalid_state" {
		t.Fatalf("expected invalid_state for a forged state, got %q", reason)
	}

	state, _ := env.beginSignIn(t, "")
	if w := env.callback(t, "?state="+state+"&code=first"); w.Code != http.StatusFound {
		t.Fatalf("first callback should succeed, got %d", w.Code)
	}
	// Replaying the same callback must not mint a second session.
	if reason := redirectedError(t, env.callback(t, "?state="+state+"&code=first")); reason != "invalid_state" {
		t.Fatalf("expected invalid_state on replay, got %q", reason)
	}
}

func TestGoogleCallbackRejectsInvalidIDTokens(t *testing.T) {
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	cases := []struct {
		name   string
		setup  func(g *fakeGoogle)
		reason string
	}{
		{
			name:   "forged signature",
			setup:  func(g *fakeGoogle) { g.signing = otherKey },
			reason: "google_failed",
		},
		{
			name:   "wrong audience",
			setup:  func(g *fakeGoogle) { g.audience = "someone-elses-client" },
			reason: "google_failed",
		},
		{
			name:   "wrong issuer",
			setup:  func(g *fakeGoogle) { g.issuer = "https://evil.example.com" },
			reason: "google_failed",
		},
		{
			name:   "expired token",
			setup:  func(g *fakeGoogle) { g.expiry = -time.Hour },
			reason: "google_failed",
		},
		{
			name:   "unverified email",
			setup:  func(g *fakeGoogle) { g.emailVerified = false },
			reason: "email_unverified",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newGoogleTestEnv(t, true)
			env.google.set(tc.setup)

			state, _ := env.beginSignIn(t, "")
			w := env.callback(t, "?state="+state+"&code=test-authorization-code")
			if reason := redirectedError(t, w); reason != tc.reason {
				t.Fatalf("expected %q, got %q", tc.reason, reason)
			}
		})
	}
}

func TestGoogleCallbackRejectsNonceMismatch(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	state, _ := env.beginSignIn(t, "")
	// Google echoing a different nonce means the ID token belongs to another
	// authorization attempt.
	env.google.set(func(g *fakeGoogle) { g.nonce = "a-different-nonce" })

	if reason := redirectedError(t, env.callback(t, "?state="+state+"&code=code")); reason != "google_failed" {
		t.Fatalf("expected google_failed for a nonce mismatch, got %q", reason)
	}
}

func TestGoogleCallbackHandlesCancelledConsent(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	state, _ := env.beginSignIn(t, "")
	w := env.callback(t, "?state="+state+"&error=access_denied")
	if reason := redirectedError(t, w); reason != "access_denied" {
		t.Fatalf("expected access_denied, got %q", reason)
	}
	// The attempt is burnt even when the user backs out.
	if _, ok := env.states.attempts[state]; ok {
		t.Fatal("a cancelled attempt must not stay reusable")
	}
}

func TestGoogleCallbackHandlesProviderOutage(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	env.google.set(func(g *fakeGoogle) { g.failToken = true })

	state, _ := env.beginSignIn(t, "")
	if reason := redirectedError(t, env.callback(t, "?state="+state+"&code=code")); reason != "google_failed" {
		t.Fatalf("expected google_failed, got %q", reason)
	}
}

func TestGoogleHandoffCodeIsSingleUse(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	handoff, _ := env.signIn(t)

	if w := env.exchange(t, handoff); w.Code != http.StatusOK {
		t.Fatalf("first exchange failed: %d %s", w.Code, w.Body.String())
	}
	w := env.exchange(t, handoff)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a consumed handoff code must be rejected, got %d %s", w.Code, w.Body.String())
	}

	if w := env.exchange(t, "never-issued"); w.Code != http.StatusBadRequest {
		t.Fatalf("an unknown handoff code must be rejected, got %d", w.Code)
	}
}

func TestGoogleSignInForBannedAccountIsRejected(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	env.signIn(t)

	for _, user := range env.users.users {
		user.IsBanned = true
	}

	state, _ := env.beginSignIn(t, "")
	if reason := redirectedError(t, env.callback(t, "?state="+state+"&code=code")); reason != "account_disabled" {
		t.Fatalf("expected account_disabled, got %q", reason)
	}
}

func TestGoogleSignInCarriesOnlySafeRedirects(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	state, _ := env.beginSignIn(t, "?redirect=%2Fgames%2Fmatch%2F123")
	w := env.callback(t, "?state="+state+"&code=code")
	handoff, redirect := handoffFromLocation(t, w.Header().Get("Location"))
	if redirect != "/games/match/123" {
		t.Fatalf("expected the internal redirect to survive, got %q", redirect)
	}
	session := decodeSession(t, env.exchange(t, handoff))
	if session.Data.Redirect != "/games/match/123" {
		t.Errorf("exchange should return the redirect, got %q", session.Data.Redirect)
	}

	// Protocol-relative and absolute targets are dropped instead of followed.
	for _, evil := range []string{"%2F%2Fevil.example.com", "https%3A%2F%2Fevil.example.com"} {
		state, _ := env.beginSignIn(t, "?redirect="+evil)
		w := env.callback(t, "?state="+state+"&code=code")
		_, redirect := handoffFromLocation(t, w.Header().Get("Location"))
		if redirect != "" {
			t.Errorf("unsafe redirect %q must be dropped, got %q", evil, redirect)
		}
	}
}

func TestGoogleEndpointsWhenNotConfigured(t *testing.T) {
	env := newGoogleTestEnv(t, false)

	w := get(t, env.router, "/api/v1/auth/providers")
	if w.Code != http.StatusOK {
		t.Fatalf("providers endpoint failed: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"google":false`) {
		t.Fatalf("expected google to be reported as unavailable, got %s", w.Body.String())
	}

	if w := get(t, env.router, "/api/v1/auth/google"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when google is not configured, got %d", w.Code)
	}
}

func TestGoogleEndpointsWhenConfigured(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	w := get(t, env.router, "/api/v1/auth/providers")
	if !strings.Contains(w.Body.String(), `"google":true`) {
		t.Fatalf("expected google to be reported as available, got %s", w.Body.String())
	}
}

func TestGoogleFlowFailsClosedWhenRedisIsUnavailable(t *testing.T) {
	env := newGoogleTestEnv(t, true)

	env.states.saveErr = context.DeadlineExceeded
	if w := get(t, env.router, "/api/v1/auth/google"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("state store outage must refuse to start a login, got %d", w.Code)
	}

	env.states.saveErr = nil
	handoff, _ := env.signIn(t)
	env.handoffs.readErr = context.DeadlineExceeded
	if w := env.exchange(t, handoff); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("handoff store outage must not authenticate anyone, got %d %s", w.Code, w.Body.String())
	}
}
