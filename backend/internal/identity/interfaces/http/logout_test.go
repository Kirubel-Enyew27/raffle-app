package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
)

// fakeTokenStore mirrors the behaviour of the Redis store closely enough for
// HTTP-level tests: one record per token, deciding both revocation and reuse.
type fakeTokenStore struct {
	records   map[string]domain.RefreshTokenRecord
	revokeErr error
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{records: map[string]domain.RefreshTokenRecord{}}
}

func (f *fakeTokenStore) Get(ctx context.Context, tokenID string) (*domain.RefreshTokenRecord, error) {
	record, ok := f.records[tokenID]
	if !ok {
		return nil, nil
	}
	return &record, nil
}

func (f *fakeTokenStore) Revoke(ctx context.Context, tokenID string, expiresAt time.Time) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	f.records[tokenID] = domain.RefreshTokenRecord{Revoked: true}
	return nil
}

func (f *fakeTokenStore) RecordExchange(ctx context.Context, tokenID string, expiresAt time.Time, session domain.RotatedSession, graceUntil time.Time) error {
	f.records[tokenID] = domain.RefreshTokenRecord{Session: &session, GraceUntil: graceUntil}
	return nil
}

func TestLogoutRevokesRefreshTokenOverHTTP(t *testing.T) {
	r, _, _ := newAuthRouter(t, 15*time.Minute, time.Hour)
	session := login(t, r)

	w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/logout", "", map[string]string{
		"refresh_token": session.Data.RefreshToken,
	})
	if w.Code != nethttp.StatusOK {
		t.Fatalf("expected 200 from logout, got %d %s", w.Code, w.Body.String())
	}

	w = doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": session.Data.RefreshToken,
	})
	if w.Code != nethttp.StatusUnauthorized {
		t.Fatalf("a signed-out refresh token must be rejected, got %d %s", w.Code, w.Body.String())
	}
	var errPayload errorPayload
	_ = json.Unmarshal(w.Body.Bytes(), &errPayload)
	if errPayload.Code != "SESSION_EXPIRED" {
		t.Errorf("expected SESSION_EXPIRED, got %q", errPayload.Code)
	}

	// Signing in again still works, so logout revokes a token rather than the user.
	fresh := login(t, r)
	w = doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": fresh.Data.RefreshToken,
	})
	if w.Code != nethttp.StatusOK {
		t.Fatalf("a new login should refresh normally, got %d %s", w.Code, w.Body.String())
	}
}

func TestLogoutIsIdempotentOverHTTP(t *testing.T) {
	r, _, _ := newAuthRouter(t, 15*time.Minute, time.Hour)
	session := login(t, r)

	// Twice with a real token, plus unknown and missing tokens: all fine.
	for _, body := range []any{
		map[string]string{"refresh_token": session.Data.RefreshToken},
		map[string]string{"refresh_token": session.Data.RefreshToken},
		map[string]string{"refresh_token": "not-a-jwt"},
		map[string]string{},
		nil,
	} {
		w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/logout", "", body)
		if w.Code != nethttp.StatusOK {
			t.Fatalf("logout should be idempotent, got %d %s", w.Code, w.Body.String())
		}
	}
}

func TestLogoutReportsRevocationStoreFailure(t *testing.T) {
	r, _, _, store := newAuthRouterWithStore(t, 15*time.Minute, time.Hour)
	session := login(t, r)

	store.revokeErr = errors.New("redis unavailable")
	w := doJSON(t, r, nethttp.MethodPost, "/api/v1/auth/logout", "", map[string]string{
		"refresh_token": session.Data.RefreshToken,
	})
	if w.Code != nethttp.StatusInternalServerError {
		t.Fatalf("expected 500 when revocation cannot be recorded, got %d %s", w.Code, w.Body.String())
	}
}
