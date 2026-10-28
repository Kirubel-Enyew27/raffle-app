package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	"github.com/redis/go-redis/v9"
)

// Refresh tokens are stateless JWTs, so a token stays valid until it expires
// unless the server remembers that it was used or signed out. This store keeps
// that memory in Redis, one key per token, expiring with the token itself — the
// set stays bounded without any cleanup job.
const refreshTokenKeyPrefix = "auth:refresh:"

// RefreshTokenStore implements domain.RefreshTokenStore on Redis.
type RefreshTokenStore struct {
	rdb    *redis.Client
	prefix string
}

func NewRefreshTokenStore(rdb *redis.Client) *RefreshTokenStore {
	return &RefreshTokenStore{rdb: rdb, prefix: refreshTokenKeyPrefix}
}

func (s *RefreshTokenStore) key(tokenID string) string {
	return s.prefix + tokenID
}

func (s *RefreshTokenStore) Get(ctx context.Context, tokenID string) (*domain.RefreshTokenRecord, error) {
	raw, err := s.rdb.Get(ctx, s.key(tokenID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var record domain.RefreshTokenRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		// A malformed record must not lock anyone out of a live session, but it
		// must not silently look like "never used" either. Treat it as spent.
		_ = s.rdb.Del(ctx, s.key(tokenID)).Err()
		return &domain.RefreshTokenRecord{Revoked: true}, nil
	}
	return &record, nil
}

func (s *RefreshTokenStore) Revoke(ctx context.Context, tokenID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		// Already expired: the token cannot be exchanged anyway, and writing a
		// key without a TTL would leak memory forever.
		return nil
	}
	payload, err := json.Marshal(domain.RefreshTokenRecord{Revoked: true})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.key(tokenID), payload, ttl).Err()
}

func (s *RefreshTokenStore) RecordExchange(ctx context.Context, tokenID string, expiresAt time.Time, session domain.RotatedSession, graceUntil time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}

	payload, err := json.Marshal(domain.RefreshTokenRecord{
		Session:    &session,
		GraceUntil: graceUntil,
	})
	if err != nil {
		return err
	}

	// One key, one write: a refresh either sees the whole exchange or none of
	// it, so a concurrent request can never mistake a rotation for a revocation.
	return s.rdb.Set(ctx, s.key(tokenID), payload, ttl).Err()
}
