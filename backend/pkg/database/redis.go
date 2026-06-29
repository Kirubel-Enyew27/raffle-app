package database

import (
	"context"
	"crypto/tls"

	"github.com/redis/go-redis/v9"
	"github.com/raffle-app/backend/pkg/config"
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
	rdb := redis.NewClient(opts)
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return rdb, nil
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
