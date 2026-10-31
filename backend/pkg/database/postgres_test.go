package database

import (
	"database/sql"
	"testing"

	"github.com/raffle-app/backend/pkg/config"
)

func TestApplyPostgresPoolSettings(t *testing.T) {
	db, err := sql.Open("postgres", "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	applyPostgresPoolSettings(db, config.DatabaseConfig{
		MaxOpenConns: 12,
		MaxIdleConns: 20,
	})
	if got := db.Stats().MaxOpenConnections; got != 12 {
		t.Errorf("max open connections = %d, want 12", got)
	}

	db2, err := sql.Open("postgres", "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	applyPostgresPoolSettings(db2, config.DatabaseConfig{})
	if got := db2.Stats().MaxOpenConnections; got != 25 {
		t.Errorf("default max open connections = %d, want 25", got)
	}
}
