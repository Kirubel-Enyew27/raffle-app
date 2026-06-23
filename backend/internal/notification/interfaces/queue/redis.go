package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/raffle-app/backend/internal/notification/domain"
)

const (
	queueKey      = "notifications:queue"
	dequeueTimeout = 5 * time.Second
)

// RedisQueue implements domain.Queue using a Redis list (LPUSH / BRPOP).
type RedisQueue struct{ rdb *redis.Client }

func NewRedisQueue(rdb *redis.Client) *RedisQueue { return &RedisQueue{rdb: rdb} }

func (q *RedisQueue) Enqueue(ctx context.Context, n *domain.Notification) error {
	b, err := json.Marshal(n)
	if err != nil {
		return err
	}
	return q.rdb.LPush(ctx, queueKey, b).Err()
}

// Dequeue blocks for up to dequeueTimeout seconds, then returns.
// Returns ctx.Err() when the context is cancelled.
func (q *RedisQueue) Dequeue(ctx context.Context) (*domain.Notification, error) {
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		res, err := q.rdb.BRPop(ctx, dequeueTimeout, queueKey).Result()
		if err == redis.Nil {
			continue // timeout, no message — loop
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		var n domain.Notification
		if err := json.Unmarshal([]byte(res[1]), &n); err != nil {
			return nil, err
		}
		return &n, nil
	}
}
