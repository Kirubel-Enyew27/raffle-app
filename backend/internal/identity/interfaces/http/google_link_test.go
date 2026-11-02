package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"

	"github.com/raffle-app/backend/internal/identity/domain"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
)

var errSentinel = errors.New("redis unavailable")

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

func getWithBearer(t *testing.T, r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const linkTestPassword = "password123"

// addPasswordUser seeds an email/password account and returns a valid access
// token for it, exactly like a session created by /auth/login.
func (e *googleTestEnv) addPasswordUser(t *testing.T, id, email string) (*domain.User, string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(linkTestPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := &domain.User{
		ID:           id,
		Email:        email,
		FullName:     "Linked Player",
		PasswordHash: string(hash),
		Role:         "user",
	}
	e.users.users[id] = user

	token, err := appjwt.GenerateTokenHMAC(user.ID, user.Email, user.Role, []byte(viper.GetString("JWT_SECRET")), 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to mint access token: %v", err)
	}
	return user, token
}

func (e *googleTestEnv) startLink(t *testing.T, token, password string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, e.router, http.MethodPost, "/api/v1/auth/google/link", token, map[string]string{"password": password})
}

// startLinkURL performs a successful link start and returns the authorization
// URL plus the state it stored.
func (e *googleTestEnv) startLinkURL(t *testing.T, token string) (string, string) {
	t.Helper()
	w := e.startLink(t, token, linkTestPassword)
	if w.Code != http.StatusOK {
		t.Fatalf("link start failed: %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode link start response: %v", err)
	}
	if payload.Data.URL == "" {
		t.Fatal("expected an authorization URL")
	}
	state, nonce := stateFromLocation(t, payload.Data.URL)
	if state == "" {
		t.Fatal("expected the authorization URL to carry a state")
	}
	return payload.Data.URL, nonce
}

func TestGoogleLinkRequiresAuthentication(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")

	if w := env.startLink(t, "", linkTestPassword); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a session, got %d", w.Code)
	}
	if w := get(t, env.router, "/api/v1/auth/google/link"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when reading the link status without a session, got %d", w.Code)
	}
	if len(env.states.attempts) != 0 {
		t.Fatal("an unauthenticated request must not create an authorization attempt")
	}

	// The status endpoint reports "not connected" for a real session.
	w := getWithBearer(t, env.router, "/api/v1/auth/google/link", token)
	if w.Code != http.StatusOK || !contains(w.Body.String(), `"linked":false`) {
		t.Fatalf("unexpected link status: %d %s", w.Code, w.Body.String())
	}
}

func TestGoogleLinkRequiresTheAccountPassword(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")

	w := env.startLink(t, token, "wrong-password")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a wrong password, got %d %s", w.Code, w.Body.String())
	}
	if !contains(w.Body.String(), "INVALID_PASSWORD") {
		t.Fatalf("expected INVALID_PASSWORD, got %s", w.Body.String())
	}
	if len(env.states.attempts) != 0 {
		t.Fatal("a failed password check must not start an authorization attempt")
	}
}

func TestGoogleLinkConnectsTheAccount(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")

	_, nonce := env.startLinkURL(t, token)
	env.google.set(func(g *fakeGoogle) { g.nonce = nonce })

	// The link attempt is bound to the user that started it.
	var state string
	for candidate := range env.states.attempts {
		state = candidate
	}
	if attempt := env.states.attempts[state]; attempt.LinkUserID != "user-1" {
		t.Fatalf("expected the attempt to be bound to the user, got %q", attempt.LinkUserID)
	}

	w := env.callback(t, "?state="+state+"&code=test-authorization-code")
	if w.Code != http.StatusFound {
		t.Fatalf("expected a redirect back to the app, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != testFrontendURL+"/profile?google=linked" {
		t.Fatalf("expected the profile page with a success marker, got %q", location)
	}

	// No session is minted and no session handoff is stored.
	if len(env.handoffs.handoffs) != 0 {
		t.Fatal("linking must not create a login session")
	}
	if len(env.users.users) != 1 {
		t.Fatalf("linking must not create an account, got %d", len(env.users.users))
	}
	linked := env.identities.identities[identityKey(domain.ProviderGoogle, "google-subject-1")]
	if linked == nil || linked.UserID != "user-1" {
		t.Fatalf("expected the google identity to be linked to user-1, got %+v", linked)
	}

	// From now on, "Sign in with Google" enters that same account.
	handoff, _ := env.signIn(t)
	session := decodeSession(t, env.exchange(t, handoff))
	if session.Data.User.ID != "user-1" {
		t.Fatalf("google sign-in should reach the linked account, got %q", session.Data.User.ID)
	}

	// And the status endpoint reflects the connection.
	if w := getWithBearer(t, env.router, "/api/v1/auth/google/link", token); !contains(w.Body.String(), `"linked":true`) {
		t.Fatalf("expected the account to report as linked, got %s", w.Body.String())
	}
}

func TestGoogleLinkRejectsADifferentGoogleEmail(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")

	_, nonce := env.startLinkURL(t, token)
	env.google.set(func(g *fakeGoogle) {
		g.nonce = nonce
		g.email = "attacker@example.com"
	})

	var state string
	for candidate := range env.states.attempts {
		state = candidate
	}
	w := env.callback(t, "?state="+state+"&code=code")
	if location := w.Header().Get("Location"); location != testFrontendURL+"/profile?google=link_email_mismatch" {
		t.Fatalf("expected an email mismatch to be reported on the profile, got %q", location)
	}
	if len(env.identities.identities) != 0 {
		t.Fatal("a mismatched google account must not be linked")
	}
}

func TestGoogleLinkRejectsAGoogleAccountOwnedBySomeoneElse(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")
	env.users.users["user-2"] = &domain.User{ID: "user-2", Email: "other@example.com", Role: "user"}
	env.identities.identities[identityKey(domain.ProviderGoogle, "google-subject-1")] = &domain.OAuthIdentity{
		ID:             "identity-2",
		UserID:         "user-2",
		Provider:       domain.ProviderGoogle,
		ProviderUserID: "google-subject-1",
		ProviderEmail:  "other@example.com",
	}

	_, nonce := env.startLinkURL(t, token)
	env.google.set(func(g *fakeGoogle) { g.nonce = nonce })

	var state string
	for candidate := range env.states.attempts {
		state = candidate
	}
	w := env.callback(t, "?state="+state+"&code=code")
	if location := w.Header().Get("Location"); location != testFrontendURL+"/profile?google=link_conflict" {
		t.Fatalf("expected a link conflict, got %q", location)
	}
	if identity := env.identities.identities[identityKey(domain.ProviderGoogle, "google-subject-1")]; identity.UserID != "user-2" {
		t.Fatal("an existing link must not be moved to another account")
	}
}

func TestGoogleLinkRefusesOnceConnected(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")

	_, nonce := env.startLinkURL(t, token)
	env.google.set(func(g *fakeGoogle) { g.nonce = nonce })
	var state string
	for candidate := range env.states.attempts {
		state = candidate
	}
	if w := env.callback(t, "?state="+state+"&code=code"); w.Code != http.StatusFound {
		t.Fatalf("expected the first link to succeed, got %d", w.Code)
	}

	if w := env.startLink(t, token, linkTestPassword); w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when google is already connected, got %d %s", w.Code, w.Body.String())
	}
}

func TestGoogleLinkRequiresAnEmailOnTheAccount(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "")

	w := env.startLink(t, token, linkTestPassword)
	if w.Code != http.StatusBadRequest || !contains(w.Body.String(), "EMAIL_REQUIRED") {
		t.Fatalf("expected EMAIL_REQUIRED, got %d %s", w.Code, w.Body.String())
	}
}

func TestGoogleLinkRejectsBannedAccount(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	user, token := env.addPasswordUser(t, "user-1", "player@example.com")
	user.IsBanned = true

	if w := env.startLink(t, token, linkTestPassword); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a banned account, got %d", w.Code)
	}
}

func TestGoogleLinkStateIsSingleUse(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")

	_, nonce := env.startLinkURL(t, token)
	env.google.set(func(g *fakeGoogle) { g.nonce = nonce })

	var state string
	for candidate := range env.states.attempts {
		state = candidate
	}
	if w := env.callback(t, "?state="+state+"&code=code"); w.Code != http.StatusFound {
		t.Fatalf("expected the first callback to succeed, got %d", w.Code)
	}
	// A replayed link callback must not silently re-link or mint anything.
	// The consumed state carries no record any more, so the attempt can no
	// longer be attributed to the link flow: it is reported as a failed
	// sign-in, which is the safe default.
	w := env.callback(t, "?state="+state+"&code=code")
	if location := w.Header().Get("Location"); location != testFrontendURL+"/login?error=invalid_state" {
		t.Fatalf("expected a replayed state to be rejected, got %q", location)
	}
	if identity := env.identities.identities[identityKey(domain.ProviderGoogle, "google-subject-1")]; identity == nil {
		t.Fatal("the original link must be untouched by the replay")
	}
}

func TestGoogleLinkFailsClosedWithoutRedis(t *testing.T) {
	env := newGoogleTestEnv(t, true)
	_, token := env.addPasswordUser(t, "user-1", "player@example.com")
	env.states.saveErr = errSentinel

	if w := env.startLink(t, token, linkTestPassword); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when the state cannot be stored, got %d", w.Code)
	}
	if len(env.identities.identities) != 0 {
		t.Fatal("nothing may be linked when the attempt could not be recorded")
	}
}
