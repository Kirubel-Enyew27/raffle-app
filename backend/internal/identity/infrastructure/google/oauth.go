package google

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/raffle-app/backend/internal/identity/domain"
)

// Endpoints is overridable so the flow can be exercised end to end against a
// local fake Google in tests. Production always uses DefaultEndpoints.
type Endpoints struct {
	AuthURL  string
	TokenURL string
	JWKSURL  string
}

var DefaultEndpoints = Endpoints{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
	JWKSURL:  "https://www.googleapis.com/oauth2/v3/certs",
}

// googleIssuers are the two spellings Google uses for the iss claim.
var googleIssuers = map[string]bool{
	"accounts.google.com":         true,
	"https://accounts.google.com": true,
}

const defaultKeyTTL = time.Hour

var errInvalidIDToken = errors.New("google: invalid id token")

type Config struct {
	ClientID     string
	ClientSecret string
	// RedirectURL must match the authorized redirect URI in the Google Cloud
	// console exactly, including scheme and path.
	RedirectURL string
	Endpoints   Endpoints
	// HTTPClient is optional; it lets tests (and proxies) supply their own.
	HTTPClient *http.Client
}

// Provider implements domain.GoogleProvider using the Authorization Code flow
// with PKCE and server-side ID token verification against Google's JWKS.
type Provider struct {
	cfg  Config
	http *http.Client
	keys *keySet
}

func NewProvider(cfg Config) *Provider {
	if cfg.Endpoints.AuthURL == "" {
		cfg.Endpoints = DefaultEndpoints
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Provider{
		cfg:  cfg,
		http: client,
		keys: &keySet{url: cfg.Endpoints.JWKSURL, http: client, keys: map[string]*rsa.PublicKey{}},
	}
}

// AuthorizationURL builds the consent-screen URL. PKCE (S256) binds the code to
// this attempt even though the client is confidential, and the nonce binds the
// returned ID token to it as well.
func (p *Provider) AuthorizationURL(state, nonce, codeChallenge string) string {
	q := url.Values{}
	q.Set("client_id", p.cfg.ClientID)
	q.Set("redirect_uri", p.cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	// Let the user pick which Google account to use instead of silently
	// reusing the one already signed in to the browser.
	q.Set("prompt", "select_account")
	return p.cfg.Endpoints.AuthURL + "?" + q.Encode()
}

type tokenResponse struct {
	IDToken          string `json:"id_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Exchange swaps an authorization code for an ID token and returns the verified
// profile. Google's access/refresh tokens are discarded: this application uses
// Google only to establish identity.
func (p *Provider) Exchange(ctx context.Context, code, codeVerifier, nonce string) (*domain.GoogleProfile, error) {
	if code == "" {
		return nil, errors.New("google: missing authorization code")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("redirect_uri", p.cfg.RedirectURL)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Endpoints.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("google: token response unreadable: %w", err)
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("google: token response malformed: %w", err)
	}
	if resp.StatusCode != http.StatusOK || parsed.Error != "" {
		return nil, fmt.Errorf("google: token exchange rejected (%d %s)", resp.StatusCode, parsed.Error)
	}
	if parsed.IDToken == "" {
		return nil, errors.New("google: token response had no id_token")
	}

	return p.verifyIDToken(ctx, parsed.IDToken, nonce)
}

type idTokenClaims struct {
	Email           string `json:"email"`
	EmailVerified   bool   `json:"email_verified"`
	Name            string `json:"name"`
	Picture         string `json:"picture"`
	Nonce           string `json:"nonce"`
	AuthorizedParty string `json:"azp"`
	jwt.RegisteredClaims
}

// verifyIDToken validates signature, algorithm, expiry, issuer, audience and
// nonce before any claim is trusted.
func (p *Provider) verifyIDToken(ctx context.Context, raw, nonce string) (*domain.GoogleProfile, error) {
	claims := &idTokenClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("google: unexpected signing algorithm")
		}
		kid, _ := t.Header["kid"].(string)
		return p.keys.key(ctx, kid)
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}), jwt.WithLeeway(30*time.Second))
	if err != nil || token == nil || !token.Valid {
		return nil, errInvalidIDToken
	}

	if !googleIssuers[claims.Issuer] {
		return nil, fmt.Errorf("%w: unexpected issuer", errInvalidIDToken)
	}
	if !audienceContains(claims.Audience, p.cfg.ClientID) {
		return nil, fmt.Errorf("%w: unexpected audience", errInvalidIDToken)
	}
	if len(claims.Audience) > 1 && claims.AuthorizedParty != p.cfg.ClientID {
		return nil, fmt.Errorf("%w: unexpected authorized party", errInvalidIDToken)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject", errInvalidIDToken)
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce mismatch", errInvalidIDToken)
	}

	return &domain.GoogleProfile{
		Subject:       claims.Subject,
		Email:         strings.ToLower(strings.TrimSpace(claims.Email)),
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		Picture:       claims.Picture,
	}, nil
}

func audienceContains(audience jwt.ClaimStrings, clientID string) bool {
	if clientID == "" {
		return false
	}
	for _, aud := range audience {
		if aud == clientID {
			return true
		}
	}
	return false
}

// ─── JWKS ───

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (j jwk) publicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, err
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, errors.New("google: malformed jwk")
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	if e == 0 {
		return nil, errors.New("google: malformed jwk exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

// keySet caches Google's signing keys. Keys rotate, so an unknown kid triggers a
// refresh; a refresh failure falls back to the cached key rather than locking
// every user out of sign-in.
type keySet struct {
	url  string
	http *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

func (k *keySet) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if time.Now().Before(k.expires) {
		if pub, ok := k.keys[kid]; ok {
			return pub, nil
		}
	}

	if err := k.refresh(ctx); err != nil {
		if pub, ok := k.keys[kid]; ok {
			return pub, nil
		}
		return nil, err
	}
	if pub, ok := k.keys[kid]; ok {
		return pub, nil
	}
	return nil, fmt.Errorf("google: unknown signing key %q", kid)
}

func (k *keySet) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return err
	}
	resp, err := k.http.Do(req)
	if err != nil {
		return fmt.Errorf("google: key fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google: key fetch returned %d", resp.StatusCode)
	}

	var payload struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return fmt.Errorf("google: key set malformed: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, candidate := range payload.Keys {
		if !strings.EqualFold(candidate.Kty, "RSA") {
			continue
		}
		pub, err := candidate.publicKey()
		if err != nil {
			continue
		}
		keys[candidate.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("google: key set was empty")
	}

	k.keys = keys
	k.expires = time.Now().Add(cacheTTL(resp.Header.Get("Cache-Control")))
	return nil
}

// cacheTTL honours the response's max-age, clamped so a stale key can never be
// cached indefinitely and a tiny value cannot trigger a fetch per request.
func cacheTTL(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "max-age=") {
			continue
		}
		seconds, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
		if err != nil {
			break
		}
		ttl := time.Duration(seconds) * time.Second
		if ttl < 5*time.Minute {
			return 5 * time.Minute
		}
		if ttl > 24*time.Hour {
			return 24 * time.Hour
		}
		return ttl
	}
	return defaultKeyTTL
}

// PKCEChallenge derives the S256 code challenge for a verifier.
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
