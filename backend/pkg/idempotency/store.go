package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewStore(rdb *redis.Client, ttl time.Duration) *Store {
	return &Store{
		rdb: rdb,
		ttl: ttl,
	}
}

func (s *Store) Acquire(ctx context.Context, key string) (bool, error) {
	result, err := s.rdb.SetNX(ctx, key, "1", s.ttl).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (s *Store) Get(ctx context.Context, key string) (string, error) {
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (s *Store) Set(ctx context.Context, key string, value string) error {
	return s.rdb.Set(ctx, key, value, s.ttl).Err()
}
