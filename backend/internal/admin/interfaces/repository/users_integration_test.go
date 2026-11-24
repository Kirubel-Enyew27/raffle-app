package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/raffle-app/backend/internal/admin/domain"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
)

func setupAdminTestDB(t *testing.T) *sql.DB {
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
	if err := database.RunMigrations(db, "../../../../migrations"); err != nil {
		_ = db.Close()
		t.Fatalf("failed to apply database migrations for integration test: %v", err)
	}
	// Register the close first so per-test cleanup functions (registered later)
	// still have a live connection when they run.
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type testUser struct {
	id    string
	email string
	phone string
	name  string
}

func insertAdminTestUser(t *testing.T, db *sql.DB, email, phone, name, role string, banned bool) testUser {
	t.Helper()
	user := testUser{email: email, phone: phone, name: name}

	var emailValue, phoneValue any
	if email != "" {
		emailValue = email
	}
	if phone != "" {
		phoneValue = phone
	}
	err := db.QueryRow(`
INSERT INTO users (email, phone, full_name, password_hash, role, is_banned, created_at, updated_at)
VALUES ($1, $2, $3, 'test-hash', $4, $5, NOW(), NOW())
RETURNING id`, emailValue, phoneValue, name, role, banned).Scan(&user.id)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO wallets (user_id, balance, reserved_balance, currency) VALUES ($1, 100, 0, 'ETB')`, user.id); err != nil {
		t.Fatalf("failed to insert test wallet: %v", err)
	}
	return user
}

func cleanupAdminTestUsers(t *testing.T, db *sql.DB, ids ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, id := range ids {
			if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, id); err != nil {
				t.Errorf("failed to clean up test user %s: %v", id, err)
			}
		}
	})
}

// Telegram Mini App accounts have no email (see migration 27). The users list
// must keep working when such an account is present: scanning a NULL email
// into a string used to fail the whole request with "could not load users".
func TestAdminRepoUsers_HandlesAccountWithoutEmail(t *testing.T) {
	db := setupAdminTestDB(t)
	token := fmt.Sprintf("noemail-%d", time.Now().UnixNano())
	user := insertAdminTestUser(t, db, "", "", "Telegram "+token, "user", false)
	cleanupAdminTestUsers(t, db, user.id)

	repo := NewAdminRepo(db)
	page, _, err := repo.Users(context.Background(), domain.UserFilter{Search: token, Limit: 25, Offset: 0})
	if err != nil {
		t.Fatalf("Users returned an error for a NULL email account: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 matching user, got %d", len(page))
	}
	if page[0].ID != user.id {
		t.Fatalf("expected user %s, got %s", user.id, page[0].ID)
	}
	if page[0].Email != "" {
		t.Fatalf("expected an empty email for an account without one, got %q", page[0].Email)
	}
}

func TestAdminRepoUsers_SearchByNameEmailAndPhone(t *testing.T) {
	db := setupAdminTestDB(t)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	name := "Search Name " + token
	email := "search-" + token + "@example.com"
	phone := "+2519" + token[len(token)-8:]
	user := insertAdminTestUser(t, db, email, phone, name, "user", false)
	cleanupAdminTestUsers(t, db, user.id)

	repo := NewAdminRepo(db)
	for _, tc := range []struct {
		name string
		term string
	}{
		{"name", "Search Name " + token},
		{"email", email},
		{"phone", phone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.Users(context.Background(), domain.UserFilter{Search: tc.term, Limit: 25})
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if len(page) != 1 || page[0].ID != user.id {
				t.Fatalf("expected exactly the fixture user, got %d rows", len(page))
			}
		})
	}
}

func TestAdminRepoUsers_StatusFilters(t *testing.T) {
	db := setupAdminTestDB(t)
	token := fmt.Sprintf("status-%d", time.Now().UnixNano())
	active := insertAdminTestUser(t, db, token+"-active@example.com", "", "Active "+token, "user", false)
	banned := insertAdminTestUser(t, db, token+"-banned@example.com", "", "Banned "+token, "user", true)
	cleanupAdminTestUsers(t, db, active.id, banned.id)

	repo := NewAdminRepo(db)

	activePage, _, err := repo.Users(context.Background(), domain.UserFilter{Search: token, Status: "active", Limit: 25})
	if err != nil {
		t.Fatalf("active filter failed: %v", err)
	}
	if len(activePage) != 1 || activePage[0].ID != active.id {
		t.Fatalf("active filter returned the wrong users: %+v", activePage)
	}

	bannedPage, _, err := repo.Users(context.Background(), domain.UserFilter{Search: token, Status: "banned", Limit: 25})
	if err != nil {
		t.Fatalf("banned filter failed: %v", err)
	}
	if len(bannedPage) != 1 || bannedPage[0].ID != banned.id {
		t.Fatalf("banned filter returned the wrong users: %+v", bannedPage)
	}
}

func TestAdminRepoUsers_EmptyResultsAndPagination(t *testing.T) {
	db := setupAdminTestDB(t)
	token := fmt.Sprintf("page-%d", time.Now().UnixNano())
	users := []testUser{
		insertAdminTestUser(t, db, token+"-1@example.com", "", "Page "+token+" 1", "user", false),
		insertAdminTestUser(t, db, token+"-2@example.com", "", "Page "+token+" 2", "user", false),
		insertAdminTestUser(t, db, token+"-3@example.com", "", "Page "+token+" 3", "user", false),
	}
	cleanupAdminTestUsers(t, db, users[0].id, users[1].id, users[2].id)

	repo := NewAdminRepo(db)

	first, total, err := repo.Users(context.Background(), domain.UserFilter{Search: token, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}
	if len(first) != 2 || total != 3 {
		t.Fatalf("expected 2 of 3 rows, got %d rows and total %d", len(first), total)
	}

	second, _, err := repo.Users(context.Background(), domain.UserFilter{Search: token, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected the remaining row on page 2, got %d", len(second))
	}
	if first[0].ID == second[0].ID || first[1].ID == second[0].ID {
		t.Fatal("pagination returned an overlapping row")
	}

	empty, _, err := repo.Users(context.Background(), domain.UserFilter{Search: "no-such-user-" + token, Limit: 25})
	if err != nil {
		t.Fatalf("empty search failed: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no rows, got %d", len(empty))
	}
}

func TestAdminRepoUsers_DoesNotExposeSensitiveFields(t *testing.T) {
	db := setupAdminTestDB(t)
	token := fmt.Sprintf("sensitive-%d", time.Now().UnixNano())
	user := insertAdminTestUser(t, db, token+"@example.com", "", "Sensitive "+token, "user", false)
	cleanupAdminTestUsers(t, db, user.id)

	repo := NewAdminRepo(db)
	page, _, err := repo.Users(context.Background(), domain.UserFilter{Search: token, Limit: 25})
	if err != nil {
		t.Fatalf("Users failed: %v", err)
	}

	payload, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("failed to marshal users page: %v", err)
	}
	body := strings.ToLower(string(payload))
	for _, forbidden := range []string{"password_hash", "\"password\"", "refresh_token", "access_token"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("users response exposes sensitive field %q", forbidden)
		}
	}
}
