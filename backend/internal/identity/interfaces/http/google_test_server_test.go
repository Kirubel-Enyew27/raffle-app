package http

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeGoogle is a local stand-in for Google's token and certificate endpoints.
// Tests control exactly what the ID token says, so signature, issuer,
// audience, expiry and nonce validation are all exercised for real.
type fakeGoogle struct {
	server *httptest.Server
	// published signs the tokens returned by /token.
	published *rsa.PrivateKey
	// publishedKeyID is advertised in the JWKS.
	publishedKeyID string
	// signing, when set, signs with a key the JWKS does not publish.
	signing *rsa.PrivateKey
	// failToken makes the token endpoint return 500.
	failToken bool

	mu            sync.Mutex
	issuer        string
	audience      string
	nonce         string
	subject       string
	email         string
	emailVerified bool
	expiry        time.Duration
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	g := &fakeGoogle{
		published:      key,
		publishedKeyID: "test-key-1",
		issuer:         "https://accounts.google.com",
		audience:       "test-client-id",
		subject:        "google-subject-1",
		email:          "player@example.com",
		emailVerified:  true,
		expiry:         time.Hour,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if g.failToken {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal_failure"}`))
			return
		}
		if r.Form.Get("code") == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		signed, err := g.sign()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed})
	})
	mux.HandleFunc("/certs", func(w http.ResponseWriter, r *http.Request) {
		pub := g.published.PublicKey
		payload := map[string]interface{}{
			"keys": []map[string]string{{
				"kty": "RSA",
				"kid": g.publishedKeyID,
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(payload)
	})

	g.server = httptest.NewServer(mux)
	t.Cleanup(g.server.Close)
	return g
}

func (g *fakeGoogle) set(update func(*fakeGoogle)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	update(g)
}

func (g *fakeGoogle) currentNonce() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.nonce
}

func (g *fakeGoogle) sign() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	claims := jwt.MapClaims{
		"iss":            g.issuer,
		"aud":            g.audience,
		"sub":            g.subject,
		"email":          g.email,
		"email_verified": g.emailVerified,
		"name":           "Google Player",
		"picture":        "https://example.com/avatar.png",
		"nonce":          g.nonce,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(g.expiry).Unix(),
	}
	key := g.published
	if g.signing != nil {
		key = g.signing
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = g.publishedKeyID
	return token.SignedString(key)
}

// googleErrorPayload is the shape the login page receives for failures.
type googleErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// handoffFromLocation pulls the one-time code out of the callback redirect.
func handoffFromLocation(t *testing.T, location string) (string, string) {
	t.Helper()
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect location %q: %v", location, err)
	}
	fragment := parsed.Fragment
	values, err := url.ParseQuery(fragment)
	if err != nil {
		t.Fatalf("failed to parse fragment %q: %v", fragment, err)
	}
	return values.Get("code"), values.Get("redirect")
}

// stateFromLocation extracts the state and nonce Google would have received.
func stateFromLocation(t *testing.T, location string) (state, nonce string) {
	t.Helper()
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse authorization url %q: %v", location, err)
	}
	values := parsed.Query()
	return values.Get("state"), values.Get("nonce")
}
