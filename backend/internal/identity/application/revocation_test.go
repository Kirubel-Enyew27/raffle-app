package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
)

// fakeTokenStore is an in-memory domain.RefreshTokenStore with a settable
// clock, so single-use rotation and its grace window can be tested without
// sleeping for a minute.
type fakeTokenStore struct {
	now       time.Time
	records   map[string]domain.RefreshTokenRecord
	revokeErr error
	recordErr error
	readErr   error
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{
		now:     time.Now(),
		records: map[string]domain.RefreshTokenRecord{},
	}
}

func (f *fakeTokenStore) advance(d time.Duration) { f.now = f.now.Add(d) }

func (f *fakeTokenStore) Get(ctx context.Context, tokenID string) (*domain.RefreshTokenRecord, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
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
	if !expiresAt.After(f.now) {
		return nil
	}
	f.records[tokenID] = domain.RefreshTokenRecord{Revoked: true}
	return nil
}

func (f *fakeTokenStore) RecordExchange(ctx context.Context, tokenID string, expiresAt time.Time, session domain.RotatedSession, graceUntil time.Time) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	if !expiresAt.After(f.now) {
		return nil
	}
	f.records[tokenID] = domain.RefreshTokenRecord{Session: &session, GraceUntil: graceUntil}
	return nil
}

func mustLogin(t *testing.T, svc *IdentityService) *Session {
	t.Helper()
	session, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	return session
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	svc, _, _ := newAuthTestService(t, 15*time.Minute, time.Hour)
	session := mustLogin(t, svc)

	if err := svc.Logout(context.Background(), session.RefreshToken); err != nil {
		t.Fatalf("unexpected logout error: %v", err)
	}

	if _, err := svc.RefreshSession(context.Background(), session.RefreshToken); !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatalf("a revoked refresh token must not be exchangeable, got %v", err)
	}

	// Signing in again yields a working session, so revocation is per token and
	// not a lockout of the account.
	fresh := mustLogin(t, svc)
	if _, err := svc.RefreshSession(context.Background(), fresh.RefreshToken); err != nil {
		t.Fatalf("a new login should be able to refresh: %v", err)
	}
}

func TestLogoutIsIdempotentAndIgnoresUnknownTokens(t *testing.T) {
	svc, _, _ := newAuthTestService(t, 15*time.Minute, time.Hour)
	session := mustLogin(t, svc)

	for name, token := range map[string]string{
		"empty":        "",
		"garbage":      "not-a-jwt",
		"access token": session.AccessToken,
	} {
		if err := svc.Logout(context.Background(), token); err != nil {
			t.Errorf("%s: logout should be a no-op, got %v", name, err)
		}
	}

	// Twice is fine, and another live session keeps working.
	other := mustLogin(t, svc)
	if err := svc.Logout(context.Background(), session.RefreshToken); err != nil {
		t.Fatalf("first logout failed: %v", err)
	}
	if err := svc.Logout(context.Background(), session.RefreshToken); err != nil {
		t.Fatalf("repeated logout should succeed, got %v", err)
	}
	if _, err := svc.RefreshSession(context.Background(), other.RefreshToken); err != nil {
		t.Fatalf("logging out another session must not affect this one: %v", err)
	}
}

func TestLogoutReportsStorageFailure(t *testing.T) {
	svc, _, _, store := newAuthTestServiceWithStore(t, 15*time.Minute, time.Hour)
	session := mustLogin(t, svc)

	store.revokeErr = errors.New("redis unavailable")
	err := svc.Logout(context.Background(), session.RefreshToken)
	if !errors.Is(err, apperrors.ErrInternal) {
		t.Fatalf("expected a transient internal error, got %v", err)
	}
}

func TestDuplicateRefreshWithinGraceReturnsSameSession(t *testing.T) {
	svc, _, _, _ := newAuthTestServiceWithStore(t, 15*time.Minute, time.Hour)
	session := mustLogin(t, svc)

	// Two tabs (or a retried request) presenting the same token back to back.
	first, err := svc.RefreshSession(context.Background(), session.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	second, err := svc.RefreshSession(context.Background(), session.RefreshToken)
	if err != nil {
		t.Fatalf("a duplicate refresh must not end the session: %v", err)
	}

	if second.AccessToken != first.AccessToken {
		t.Error("expected the duplicate refresh to return the session that was already issued")
	}
	if second.RefreshToken != first.RefreshToken {
		t.Error("expected the duplicate refresh to return the same rotated refresh token")
	}
	if second.User == nil || second.User.ID != "user-123" {
		t.Fatalf("unexpected user on duplicate refresh: %+v", second.User)
	}

	// The rotated token keeps the session alive for further play.
	if _, err := svc.RefreshSession(context.Background(), first.RefreshToken); err != nil {
		t.Fatalf("rotated refresh token should be usable: %v", err)
	}
}

func TestRefreshRejectsTokenReplayedAfterGraceWindow(t *testing.T) {
	svc, _, _, store := newAuthTestServiceWithStore(t, 15*time.Minute, time.Hour)
	session := mustLogin(t, svc)

	renewed, err := svc.RefreshSession(context.Background(), session.RefreshToken)
	if err != nil {
		t.Fatalf("unexpected refresh error: %v", err)
	}

	store.advance(rotationGrace + time.Second)

	if _, err := svc.RefreshSession(context.Background(), session.RefreshToken); !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatalf("an exchanged token must be dead after the grace window, got %v", err)
	}

	// The token issued by that refresh is unaffected.
	if _, err := svc.RefreshSession(context.Background(), renewed.RefreshToken); err != nil {
		t.Fatalf("the rotated token should still work: %v", err)
	}
}

func TestTokenStoreOutageNeverExpiresSession(t *testing.T) {
	svc, _, _, store := newAuthTestServiceWithStore(t, 15*time.Minute, time.Hour)
	session := mustLogin(t, svc)

	store.readErr = errors.New("redis unavailable")
	_, err := svc.RefreshSession(context.Background(), session.RefreshToken)
	if !errors.Is(err, apperrors.ErrInternal) {
		t.Fatalf("expected a transient internal error, got %v", err)
	}
	if errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatal("a revocation-store outage must never be reported as an expired session")
	}

	// Once the store is back, the untouched token is still usable.
	store.readErr = nil
	if _, err := svc.RefreshSession(context.Background(), session.RefreshToken); err != nil {
		t.Fatalf("session should survive the outage: %v", err)
	}
}

func TestExchangeFailureDiscardsNewSession(t *testing.T) {
	svc, _, _, store := newAuthTestServiceWithStore(t, 15*time.Minute, time.Hour)
	session := mustLogin(t, svc)

	store.recordErr = errors.New("redis unavailable")
	rotated, err := svc.RefreshSession(context.Background(), session.RefreshToken)
	if !errors.Is(err, apperrors.ErrInternal) {
		t.Fatalf("expected a transient internal error, got %v", err)
	}
	if rotated != nil {
		t.Fatal("a session whose exchange could not be recorded must not be handed out")
	}
}
