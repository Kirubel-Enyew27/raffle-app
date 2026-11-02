package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
)

// ─── fakes ───

type googleUserRepo struct {
	users map[string]*domain.User
}

func newGoogleUserRepo() *googleUserRepo {
	return &googleUserRepo{users: map[string]*domain.User{}}
}

func (r *googleUserRepo) Create(ctx context.Context, user *domain.User) error {
	r.users[user.ID] = user
	return nil
}

func (r *googleUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if user, ok := r.users[id]; ok {
		return user, nil
	}
	return nil, apperrors.ErrNotFound
}

func (r *googleUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (r *googleUserRepo) FindByName(ctx context.Context, name string) (*domain.User, error) {
	return nil, apperrors.ErrNotFound
}

func (r *googleUserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, apperrors.ErrNotFound
}

func (r *googleUserRepo) Update(ctx context.Context, user *domain.User) error {
	r.users[user.ID] = user
	return nil
}

func (r *googleUserRepo) SoftDelete(ctx context.Context, id string) error {
	delete(r.users, id)
	return nil
}

func (r *googleUserRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	return nil
}

func (r *googleUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, err := r.FindByEmail(ctx, email)
	return err == nil, nil
}

func (r *googleUserRepo) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	return false, nil
}

type googleIdentityRepo struct {
	identities map[string]*domain.OAuthIdentity
	users      *googleUserRepo
}

func newGoogleIdentityRepo(users *googleUserRepo) *googleIdentityRepo {
	return &googleIdentityRepo{identities: map[string]*domain.OAuthIdentity{}, users: users}
}

func (r *googleIdentityRepo) FindByProviderUserID(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error) {
	if identity, ok := r.identities[provider+":"+subject]; ok {
		return identity, nil
	}
	return nil, apperrors.ErrNotFound
}

func (r *googleIdentityRepo) FindByUserID(ctx context.Context, provider, userID string) (*domain.OAuthIdentity, error) {
	for _, identity := range r.identities {
		if identity.Provider == provider && identity.UserID == userID {
			return identity, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (r *googleIdentityRepo) Create(ctx context.Context, identity *domain.OAuthIdentity) error {
	for _, existing := range r.identities {
		if existing.Provider == identity.Provider && existing.UserID == identity.UserID {
			return apperrors.ErrConflict
		}
	}
	r.identities[identity.Provider+":"+identity.ProviderUserID] = identity
	return nil
}

func (r *googleIdentityRepo) CreateUserWithIdentity(ctx context.Context, user *domain.User, identity *domain.OAuthIdentity) error {
	r.users.users[user.ID] = user
	r.identities[identity.Provider+":"+identity.ProviderUserID] = identity
	return nil
}

type googleStateStore struct {
	attempts map[string]domain.OAuthAttempt
	saveErr  error
}

func newGoogleStateStore() *googleStateStore {
	return &googleStateStore{attempts: map[string]domain.OAuthAttempt{}}
}

func (s *googleStateStore) Save(ctx context.Context, state string, attempt domain.OAuthAttempt, ttl time.Duration) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.attempts[state] = attempt
	return nil
}

func (s *googleStateStore) Consume(ctx context.Context, state string) (*domain.OAuthAttempt, error) {
	attempt, ok := s.attempts[state]
	if !ok {
		return nil, nil
	}
	delete(s.attempts, state)
	return &attempt, nil
}

type googleHandoffStore struct {
	handoffs map[string]domain.LoginHandoff
}

func newGoogleHandoffStore() *googleHandoffStore {
	return &googleHandoffStore{handoffs: map[string]domain.LoginHandoff{}}
}

func (s *googleHandoffStore) Save(ctx context.Context, code string, handoff domain.LoginHandoff, ttl time.Duration) error {
	s.handoffs[code] = handoff
	return nil
}

func (s *googleHandoffStore) Consume(ctx context.Context, code string) (*domain.LoginHandoff, error) {
	handoff, ok := s.handoffs[code]
	if !ok {
		return nil, nil
	}
	delete(s.handoffs, code)
	return &handoff, nil
}

type googleProviderFake struct {
	challenge  string
	state      string
	nonce      string
	profile    *domain.GoogleProfile
	exchangeOK bool
}

func (p *googleProviderFake) AuthorizationURL(state, nonce, codeChallenge string) string {
	p.state, p.nonce, p.challenge = state, nonce, codeChallenge
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state
}

func (p *googleProviderFake) Exchange(ctx context.Context, code, codeVerifier, nonce string) (*domain.GoogleProfile, error) {
	if !p.exchangeOK || code == "" {
		return nil, errors.New("exchange failed")
	}
	return p.profile, nil
}

type googleFixture struct {
	svc        *IdentityService
	provider   *googleProviderFake
	states     *googleStateStore
	handoffs   *googleHandoffStore
	identities *googleIdentityRepo
	users      *googleUserRepo
}

func newGoogleFixture(t *testing.T) *googleFixture {
	t.Helper()
	users := newGoogleUserRepo()
	identities := newGoogleIdentityRepo(users)
	states := newGoogleStateStore()
	handoffs := newGoogleHandoffStore()
	provider := &googleProviderFake{
		exchangeOK: true,
		profile: &domain.GoogleProfile{
			Subject:       "google-subject-1",
			Email:         "new-player@example.com",
			EmailVerified: true,
			Name:          "New Player",
		},
	}

	svc := NewIdentityService(users, auditapp.NewAuditService(&mockAuditRepo{}), generateTestSecret(), 15*time.Minute, time.Hour, newFakeTokenStore())
	svc.SetGoogleOAuth(provider, identities, states, handoffs)

	return &googleFixture{
		svc:        svc,
		provider:   provider,
		states:     states,
		handoffs:   handoffs,
		identities: identities,
		users:      users,
	}
}

func TestStartGoogleLoginStoresAttemptWithPKCEChallenge(t *testing.T) {
	f := newGoogleFixture(t)

	if _, err := f.svc.StartGoogleLogin(context.Background(), "/games"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attempt, ok := f.states.attempts[f.provider.state]
	if !ok {
		t.Fatal("expected the authorization attempt to be stored server-side")
	}
	if attempt.RedirectTo != "/games" {
		t.Errorf("expected the redirect to be kept, got %q", attempt.RedirectTo)
	}
	if attempt.Nonce != f.provider.nonce {
		t.Errorf("the stored nonce must be the one sent to Google")
	}
	if len(attempt.CodeVerifier) < 43 {
		t.Errorf("PKCE verifier is too short: %d characters", len(attempt.CodeVerifier))
	}

	sum := sha256.Sum256([]byte(attempt.CodeVerifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	if f.provider.challenge != expected {
		t.Errorf("code challenge mismatch: got %q, expected %q", f.provider.challenge, expected)
	}
}

func TestStartGoogleLoginDropsUnsafeRedirects(t *testing.T) {
	f := newGoogleFixture(t)

	for _, unsafe := range []string{"//evil.example.com", "https://evil.example.com", "javascript:alert(1)", ""} {
		if _, err := f.svc.StartGoogleLogin(context.Background(), unsafe); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.states.attempts[f.provider.state].RedirectTo; got != "" {
			t.Errorf("unsafe redirect %q must be dropped, got %q", unsafe, got)
		}
	}
}

func TestCompleteGoogleLoginConsumesStateExactlyOnce(t *testing.T) {
	f := newGoogleFixture(t)

	if _, err := f.svc.StartGoogleLogin(context.Background(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := f.provider.state

	if _, err := f.svc.CompleteGoogleCallback(context.Background(), state, "code", ""); err != nil {
		t.Fatalf("first callback failed: %v", err)
	}
	if _, err := f.svc.CompleteGoogleCallback(context.Background(), state, "code", ""); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("expected a replayed state to be rejected, got %v", err)
	}
}

func TestCompleteGoogleLoginFailsClosedWithoutStateStore(t *testing.T) {
	f := newGoogleFixture(t)
	f.states.saveErr = errors.New("redis unavailable")

	if _, err := f.svc.StartGoogleLogin(context.Background(), ""); !errors.Is(err, apperrors.ErrInternal) {
		t.Fatalf("expected a transient failure when the state cannot be stored, got %v", err)
	}
}

func TestCompleteGoogleLoginRefusesExistingEmailAccount(t *testing.T) {
	f := newGoogleFixture(t)
	f.users.users["existing"] = &domain.User{ID: "existing", Email: "new-player@example.com", Role: "user"}

	if _, err := f.svc.StartGoogleLogin(context.Background(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := f.svc.CompleteGoogleCallback(context.Background(), f.provider.state, "code", "")
	if !errors.Is(err, ErrGoogleAccountExists) {
		t.Fatalf("expected the email conflict to be refused, got %v", err)
	}
	if len(f.identities.identities) != 0 {
		t.Fatal("no identity may be linked on an email conflict")
	}
}

func TestGoogleHandoffIsSingleUse(t *testing.T) {
	f := newGoogleFixture(t)

	if _, err := f.svc.StartGoogleLogin(context.Background(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, err := f.svc.CompleteGoogleCallback(context.Background(), f.provider.state, "code", "")
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}
	code := result.HandoffCode

	session, _, err := f.svc.ExchangeGoogleHandoff(context.Background(), code)
	if err != nil {
		t.Fatalf("first exchange failed: %v", err)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatal("expected the stored session to be returned")
	}
	if _, _, err := f.svc.ExchangeGoogleHandoff(context.Background(), code); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("a handoff code must only work once, got %v", err)
	}
}

func TestGoogleHandoffDoesNotLeakTokensIntoARedirect(t *testing.T) {
	f := newGoogleFixture(t)

	if _, err := f.svc.StartGoogleLogin(context.Background(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, err := f.svc.CompleteGoogleCallback(context.Background(), f.provider.state, "code", "")
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}
	code := result.HandoffCode
	stored, ok := f.handoffs.handoffs[code]
	if !ok {
		t.Fatal("expected the session to be parked behind the handoff code")
	}
	if stored.AccessToken == "" {
		t.Fatal("expected the handoff to hold the minted session")
	}
	// The code itself must be a random credential, not a JWT.
	if stored.AccessToken == code {
		t.Fatal("the handoff code must never be the access token")
	}
}

func TestGoogleSignInCreatesOnlyTheDefaultRole(t *testing.T) {
	f := newGoogleFixture(t)

	if _, err := f.svc.StartGoogleLogin(context.Background(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := f.svc.CompleteGoogleCallback(context.Background(), f.provider.state, "code", ""); err != nil {
		t.Fatalf("callback failed: %v", err)
	}

	for _, user := range f.users.users {
		if user.Role != "user" {
			t.Fatalf("google sign-up must receive the default role, got %q", user.Role)
		}
	}
}

func TestSafeRedirectPath(t *testing.T) {
	cases := map[string]string{
		"/dashboard":               "/dashboard",
		"  /wallet  ":              "/wallet",
		"//evil.example.com":       "",
		`/\evil.example.com`:       "",
		"https://evil.example.com": "",
		"javascript:alert(1)":      "",
		"":                         "",
	}
	for input, expected := range cases {
		if got := SafeRedirectPath(input); got != expected {
			t.Errorf("SafeRedirectPath(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestGoogleSignInUnavailableWithoutCredentials(t *testing.T) {
	users := newGoogleUserRepo()
	svc := NewIdentityService(users, auditapp.NewAuditService(&mockAuditRepo{}), generateTestSecret(), time.Minute, time.Hour, newFakeTokenStore())

	if svc.GoogleEnabled() {
		t.Fatal("google sign-in must be off until it is configured")
	}
	if _, err := svc.StartGoogleLogin(context.Background(), ""); !errors.Is(err, ErrGoogleUnavailable) {
		t.Fatalf("expected ErrGoogleUnavailable, got %v", err)
	}
	if _, _, err := svc.ExchangeGoogleHandoff(context.Background(), "code"); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("expected the exchange to be refused, got %v", err)
	}
}
