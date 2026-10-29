package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// P2PRateLimiter is a Redis-backed sliding-window rate limiter for P2P actions.
// It uses the INCR+EXPIRE pattern: the counter key encodes the action, user ID,
// and a time bucket (Unix seconds / window size). This means each window starts
// at a fixed boundary (not a rolling window), which is simple, cheap, and fair.
//
// Key format: p2p_rl:{action}:{user_id}:{bucket}
type P2PRateLimiter struct {
	rdb *redis.Client
}

// NewP2PRateLimiter creates a rate limiter backed by the given Redis client.
func NewP2PRateLimiter(rdb *redis.Client) *P2PRateLimiter {
	return &P2PRateLimiter{rdb: rdb}
}

// rateLimitConfig defines the per-action limits.
type rateLimitConfig struct {
	action     string
	max        int64
	windowSecs int64
}

var (
	rlMove   = rateLimitConfig{action: "move", max: 30, windowSecs: 60}
	rlCreate = rateLimitConfig{action: "create", max: 10, windowSecs: 60}
	rlJoin   = rateLimitConfig{action: "join", max: 20, windowSecs: 60}
)

// allow returns true when the request is within the configured limit.
// On Redis failure it fails open (allows) to avoid a Redis outage blocking
// all gameplay.
func (rl *P2PRateLimiter) allow(ctx context.Context, userID string, cfg rateLimitConfig) bool {
	if rl == nil || rl.rdb == nil {
		return true
	}
	bucket := time.Now().Unix() / cfg.windowSecs
	key := fmt.Sprintf("p2p_rl:%s:%s:%d", cfg.action, userID, bucket)

	pipe := rl.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(cfg.windowSecs*2)*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		// Fail open: don't block the user on Redis errors.
		return true
	}

	return incrCmd.Val() <= cfg.max
}

// MoveRateLimit is gin middleware that enforces the move submission rate limit.
func (rl *P2PRateLimiter) MoveRateLimit() gin.HandlerFunc {
	return rl.rateLimitMiddleware(rlMove)
}

// CreateRateLimit is gin middleware that enforces the match creation rate limit.
func (rl *P2PRateLimiter) CreateRateLimit() gin.HandlerFunc {
	return rl.rateLimitMiddleware(rlCreate)
}

// JoinRateLimit is gin middleware that enforces the join/ready rate limit.
func (rl *P2PRateLimiter) JoinRateLimit() gin.HandlerFunc {
	return rl.rateLimitMiddleware(rlJoin)
}

func (rl *P2PRateLimiter) rateLimitMiddleware(cfg rateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			// Auth middleware hasn't run yet or was skipped; let the handler
			// deal with the missing auth rather than rate-limiting the wrong key.
			c.Next()
			return
		}
		if !rl.allow(c.Request.Context(), userID, cfg) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    "RATE_LIMITED",
				"message": "too many requests, please slow down",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
