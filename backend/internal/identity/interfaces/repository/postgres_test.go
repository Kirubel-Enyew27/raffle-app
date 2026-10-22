package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/raffle-app/backend/internal/identity/domain"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
)

func TestUserRepo_CreateAllowsMultipleUsersWithoutPhone(t *testing.T) {
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
		t.Skipf("skipping integration test, postgres not available: %v", err)
	}
	defer db.Close()

	repo := NewUserRepo(db)
	ctx := context.Background()
	stamp := time.Now().UnixNano()

	// users.phone is UNIQUE. Historically the repository wrote '' for missing
	// phone numbers, so the second phone-less registration failed.
	first := &domain.User{
		ID:           fmt.Sprintf("11111111-1111-4111-8111-%012d", stamp%1e12),
		Email:        fmt.Sprintf("nophone-a-%d@example.com", stamp),
		PasswordHash: "hash",
		Role:         "user",
	}
	second := &domain.User{
		ID:           fmt.Sprintf("22222222-2222-4222-8222-%012d", stamp%1e12),
		Email:        fmt.Sprintf("nophone-b-%d@example.com", stamp),
		PasswordHash: "hash",
		Role:         "user",
	}

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first phone-less user creation failed: %v", err)
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("second phone-less user creation failed: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, first.ID, second.ID)
	})

	loaded, err := repo.FindByEmail(ctx, second.Email)
	if err != nil {
		t.Fatalf("failed to reload second user: %v", err)
	}
	if loaded.Phone != "" {
		t.Fatalf("expected empty phone on reload, got %q", loaded.Phone)
	}
}
