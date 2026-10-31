package database

import (
	"testing"

	"github.com/raffle-app/backend/pkg/config"
)

func TestNewRedisReturnsClientWithoutRequiringAvailability(t *testing.T) {
	client, err := NewRedis(config.RedisConfig{Host: "127.0.0.1", Port: "1"})
	if err != nil {
		t.Fatalf("NewRedis should construct a lazy client during an outage: %v", err)
	}
	if client == nil {
		t.Fatal("NewRedis returned a nil client")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close Redis client: %v", err)
	}
}
