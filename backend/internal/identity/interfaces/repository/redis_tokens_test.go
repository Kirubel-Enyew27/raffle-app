package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
)

// newTestStore connects to the local Redis and namespaces every key under a
// unique prefix, so the test never touches live session keys.
func newTestStore(t *testing.T) *RefreshTokenStore {
	t.Helper()

	rdb, err := database.NewRedis(config.RedisConfig{Host: "localhost", Port: "6379"})
	if err != nil {
		t.Skipf("skipping integration test, redis not available: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	return &RefreshTokenStore{
		rdb:    rdb,
		prefix: fmt.Sprintf("auth:refresh:test:%d:", time.Now().UnixNano()),
	}
}

func TestRefreshTokenStoreUnknownTokenHasNoRecord(t *testing.T) {
	store := newTestStore(t)

	record, err := store.Get(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record != nil {
		t.Fatalf("expected no record for an unused token, got %+v", record)
	}
}

func TestRefreshTokenStoreRevokeAndCheck(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Revoke(ctx, "token-a", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	record, err := store.Get(ctx, "token-a")
	if err != nil {
		t.Fatalf("failed to read record: %v", err)
	}
	if record == nil || !record.Revoked {
		t.Fatalf("expected the token to be revoked, got %+v", record)
	}
	if session := record.Reusable(time.Now()); session != nil {
		t.Fatal("a revoked token must never yield a session")
	}

	// Revocation is per token.
	other, err := store.Get(ctx, "token-b")
	if err != nil {
		t.Fatalf("failed to read record: %v", err)
	}
	if other != nil {
		t.Fatalf("another token must stay untouched, got %+v", other)
	}
}

func TestRefreshTokenStoreKeysExpireWithTheToken(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Revoke(ctx, "short-lived", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	ttl, err := store.rdb.TTL(ctx, store.key("short-lived")).Result()
	if err != nil {
		t.Fatalf("failed to read ttl: %v", err)
	}
	// Without a TTL the revocation set would grow without bound forever.
	if ttl <= 0 || ttl > 30*time.Minute {
		t.Fatalf("expected a ttl bounded by the token lifetime, got %v", ttl)
	}

	// An already-expired token cannot be exchanged, so nothing is written and no
	// key without an expiry is left behind.
	if err := store.Revoke(ctx, "already-expired", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("revoking an expired token should be a no-op: %v", err)
	}
	if exists, err := store.rdb.Exists(ctx, store.key("already-expired")).Result(); err != nil || exists != 0 {
		t.Fatalf("expected no key for an expired token (err=%v exists=%d)", err, exists)
	}
}

func TestRefreshTokenStoreRecordExchange(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	session := domain.RotatedSession{AccessToken: "access-1", RefreshToken: "refresh-1", ExpiresIn: 900}
	grace := time.Now().Add(time.Minute)
	if err := store.RecordExchange(ctx, "token-a", time.Now().Add(time.Hour), session, grace); err != nil {
		t.Fatalf("record exchange failed: %v", err)
	}

	record, err := store.Get(ctx, "token-a")
	if err != nil {
		t.Fatalf("failed to read record: %v", err)
	}
	if record == nil || record.Revoked {
		t.Fatalf("an exchanged token is spent, not revoked, got %+v", record)
	}
	reused := record.Reusable(time.Now())
	if reused == nil || reused.AccessToken != session.AccessToken || reused.RefreshToken != session.RefreshToken {
		t.Fatalf("expected the stored session to be reusable, got %+v", reused)
	}

	// Past the grace window the same record stops being reusable.
	if later := record.Reusable(grace.Add(time.Second)); later != nil {
		t.Fatal("an exchanged token must not be reusable after the grace window")
	}

	// The record outlives the grace window (it has to keep saying "spent" until
	// the token expires), so its TTL tracks the token, not the window.
	ttl, err := store.rdb.TTL(ctx, store.key("token-a")).Result()
	if err != nil {
		t.Fatalf("failed to read ttl: %v", err)
	}
	if ttl <= time.Minute {
		t.Fatalf("expected the record to live until the token expires, got ttl %v", ttl)
	}
}

// A rotation must never read back as a revocation: that is what logged players
// out when two requests refreshed at the same moment.
func TestRefreshTokenStoreConcurrentExchangeIsNeverRevoked(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			session := domain.RotatedSession{
				AccessToken:  fmt.Sprintf("access-%d", i),
				RefreshToken: fmt.Sprintf("refresh-%d", i),
				ExpiresIn:    900,
			}
			if err := store.RecordExchange(ctx, "token-race", expires, session, time.Now().Add(time.Minute)); err != nil {
				t.Errorf("record exchange failed: %v", err)
			}
			// Every reader in that window must see a complete, non-revoked record.
			record, err := store.Get(ctx, "token-race")
			if err != nil {
				t.Errorf("get failed: %v", err)
				return
			}
			if record != nil && record.Revoked {
				t.Error("a concurrent rotation must never look like a revocation")
			}
		}(i)
	}
	wg.Wait()

	record, err := store.Get(ctx, "token-race")
	if err != nil {
		t.Fatalf("failed to read record: %v", err)
	}
	if record == nil || record.Revoked || record.Reusable(time.Now()) == nil {
		t.Fatalf("expected a usable exchanged record, got %+v", record)
	}
}

func TestRefreshTokenStoreTreatsCorruptRecordAsSpent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.rdb.Set(ctx, store.key("token-a"), "not-json", time.Minute).Err(); err != nil {
		t.Fatalf("failed to seed corrupt record: %v", err)
	}

	record, err := store.Get(ctx, "token-a")
	if err != nil {
		t.Fatalf("a corrupt record must not be an error: %v", err)
	}
	if record == nil || !record.Revoked {
		t.Fatalf("a corrupt record must be treated as spent, got %+v", record)
	}
}

func TestRefreshTokenStoreReportsOutage(t *testing.T) {
	rdb, err := database.NewRedis(config.RedisConfig{Host: "localhost", Port: "6379"})
	if err != nil {
		t.Skipf("skipping integration test, redis not available: %v", err)
	}
	store := NewRefreshTokenStore(rdb)
	_ = rdb.Close()

	// The failure has to surface as an error: callers turn it into a transient
	// failure, never into "this token is fine" or "this session is dead".
	if _, err := store.Get(context.Background(), "token-a"); err == nil {
		t.Fatal("expected an error from an unreachable store")
	}
	if err := store.Revoke(context.Background(), "token-a", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected an error from an unreachable store")
	}
}
