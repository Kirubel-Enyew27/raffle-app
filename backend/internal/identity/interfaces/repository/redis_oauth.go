package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/raffle-app/backend/internal/identity/domain"
)

// Both stores use GETDEL so consuming a value is a single atomic operation:
// two concurrent callbacks (or two browser tabs) can never both succeed, across
// however many backend instances are running.
const (
	oauthStateKeyPrefix   = "auth:oauth:state:"
	loginHandoffKeyPrefix = "auth:oauth:handoff:"
)

// OAuthStateStore keeps pending authorization attempts in Redis.
type OAuthStateStore struct {
	rdb *redis.Client
}

func NewOAuthStateStore(rdb *redis.Client) *OAuthStateStore {
	return &OAuthStateStore{rdb: rdb}
}

func (s *OAuthStateStore) Save(ctx context.Context, state string, attempt domain.OAuthAttempt, ttl time.Duration) error {
	if state == "" {
		return errors.New("oauth state must not be empty")
	}
	payload, err := json.Marshal(attempt)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, oauthStateKeyPrefix+state, payload, ttl).Err()
}

func (s *OAuthStateStore) Consume(ctx context.Context, state string) (*domain.OAuthAttempt, error) {
	if state == "" {
		return nil, nil
	}
	raw, err := s.rdb.GetDel(ctx, oauthStateKeyPrefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var attempt domain.OAuthAttempt
	if err := json.Unmarshal(raw, &attempt); err != nil {
		// A corrupt record cannot be trusted; treat it as already consumed.
		return nil, nil
	}
	return &attempt, nil
}

// LoginHandoffStore holds a minted session for a few seconds so the SPA can
// collect it without the access token ever appearing in a URL.
type LoginHandoffStore struct {
	rdb *redis.Client
}

func NewLoginHandoffStore(rdb *redis.Client) *LoginHandoffStore {
	return &LoginHandoffStore{rdb: rdb}
}

func (s *LoginHandoffStore) Save(ctx context.Context, code string, handoff domain.LoginHandoff, ttl time.Duration) error {
	if code == "" {
		return errors.New("handoff code must not be empty")
	}
	payload, err := json.Marshal(handoff)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, loginHandoffKeyPrefix+code, payload, ttl).Err()
}

func (s *LoginHandoffStore) Consume(ctx context.Context, code string) (*domain.LoginHandoff, error) {
	if code == "" {
		return nil, nil
	}
	raw, err := s.rdb.GetDel(ctx, loginHandoffKeyPrefix+code).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var handoff domain.LoginHandoff
	if err := json.Unmarshal(raw, &handoff); err != nil {
		return nil, nil
	}
	return &handoff, nil
}
