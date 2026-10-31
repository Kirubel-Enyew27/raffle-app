package database

import (
	"context"
	"crypto/tls"

	"github.com/raffle-app/backend/pkg/config"
	"github.com/redis/go-redis/v9"
)

func NewRedis(cfg config.RedisConfig) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.TLSEnabled {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	// go-redis connects lazily and retries Pub/Sub subscriptions after socket
	// failures. Do not make Redis availability a process-start requirement:
	// gameplay writes remain authoritative in PostgreSQL, and the realtime
	// subscriber plus distributed rate limiter can recover when Redis returns.
	return redis.NewClient(opts), nil
}

func CloseRedis(rdb *redis.Client) error {
	if rdb == nil {
		return nil
	}
	return rdb.Close()
}

func HealthCheckRedis(rdb *redis.Client) error {
	ctx := context.Background()
	return rdb.Ping(ctx).Err()
}
