package application

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	walletrepo "github.com/raffle-app/backend/internal/wallet/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
)

func setupWalletTestDB(t *testing.T) *sql.DB {
	t.Helper()
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "raffle",
		Password: "raffle_dev",
		DBName:   "raffle_db",
		SSLMode:  "disable",
	}
	db, err := database.NewPostgres(cfg)
	if err != nil {
		t.Skipf("skipping wallet integration test, postgres not available: %v", err)
	}
	if err := database.RunMigrations(db, "../../../migrations"); err != nil {
		_ = db.Close()
		t.Fatalf("failed to apply database migrations: %v", err)
	}
	return db
}

func TestGetWallet_ConcurrentFirstAccessCreatesOneWallet(t *testing.T) {
	db := setupWalletTestDB(t)
	defer db.Close()

	ctx := context.Background()
	email := fmt.Sprintf("wallet-race-%d@example.com", time.Now().UnixNano())
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'user') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	svc := NewWalletService(walletrepo.NewWalletRepo(db), nil)

	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.GetWallet(ctx, userID); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent GetWallet failed: %v", err)
	}

	var wallets int
	if err := db.QueryRow(`SELECT COUNT(*) FROM wallets WHERE user_id = $1`, userID).Scan(&wallets); err != nil {
		t.Fatalf("failed to count wallets: %v", err)
	}
	if wallets != 1 {
		t.Fatalf("expected exactly one wallet per user, got %d", wallets)
	}
}
