package infrastructure

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter is a small Redis fixed-window limiter used to stop a single
// account hammering the betting and cash-out endpoints.
//
// It deliberately fails open: if Redis is unavailable, the request is allowed
// and the caller logs the error. Financial correctness never depends on the
// limiter — duplicates are prevented by database constraints and the atomic
// bet-status transitions — so degrading to "no rate limit" is strictly safer
// than refusing legitimate cash-outs.
type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 30
	}
	if window <= 0 {
		window = time.Second
	}
	return &RateLimiter{rdb: rdb, limit: limit, window: window}
}

// Allow reports whether the caller is within the window budget. The second
// return value is a Redis/transport error; callers should allow the action and
// log when it is non-nil.
func (l *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if l == nil || l.rdb == nil {
		return true, nil
	}

	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, l.window)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, err
	}
	return incr.Val() <= int64(l.limit), nil
}
