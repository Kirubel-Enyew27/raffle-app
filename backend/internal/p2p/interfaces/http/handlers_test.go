package http

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/raffle-app/backend/internal/p2p/application"
	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
)

func setupTestDB(t *testing.T) *sql.DB {
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
		t.Skipf("skipping test, postgres not available: %v", err)
	}
	return db
}

func createTestUser(t *testing.T, db *sql.DB, fullName, email string) string {
	t.Helper()
	var userID string
	err := db.QueryRow(`INSERT INTO users (email, full_name, password_hash, role)
		VALUES ($1, $2, 'hash', 'user') RETURNING id`, email, fullName).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	return userID
}

func insertMatchWithPlayers(t *testing.T, db *sql.DB, gameSlug, user1, user2 string, winner *string, status string, completedAt time.Time) string {
	t.Helper()
	var winnerVal interface{}
	if winner != nil {
		winnerVal = *winner
	}
	var matchID string
	err := db.QueryRow(`INSERT INTO p2p_matches (
		id, game_slug, status, match_type, stake_cents, prize_pool_cents,
		winner_user_id, created_by, completed_at, created_at, updated_at
	) VALUES (uuid_generate_v4(), $1, $2, 'public_queue', 0, 0, $3, $4, $5, $5, $5) RETURNING id`,
		gameSlug, status, winnerVal, user1, completedAt).Scan(&matchID)
	if err != nil {
		t.Fatalf("failed to insert match: %v", err)
	}

	_, _ = db.Exec(`INSERT INTO p2p_match_players (id, match_id, user_id, player_index, status, joined_at)
		VALUES (uuid_generate_v4(), $1, $2, 0, 'active', $3)`, matchID, user1, completedAt)
	_, _ = db.Exec(`INSERT INTO p2p_match_players (id, match_id, user_id, player_index, status, joined_at)
		VALUES (uuid_generate_v4(), $1, $2, 1, 'active', $3)`, matchID, user2, completedAt)

	return matchID
}

func setupTestRouter(svc *application.P2PService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	handler := NewP2PHandler(svc)
	RegisterP2PRoutes(api, handler)
	return r
}

func TestHTTP_Leaderboard_ValidAndInvalidParameters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gameRepo := p2prepo.NewGameRepo(db)
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	moveRepo := p2prepo.NewMoveRepo(db)
	settingsRepo := p2prepo.NewSettingsRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := application.NewP2PService(gameRepo, matchRepo, playerRepo, moveRepo, settingsRepo, escrowRepo, nil, nil)
	router := setupTestRouter(svc)

	// 1. Invalid period
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/p2p/leaderboard?period=hourly", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid period, got %d", w.Code)
	}

	// 2. Invalid sort_by
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/p2p/leaderboard?sort_by=hacks", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid sort_by, got %d", w.Code)
	}

	// 3. Invalid page
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/p2p/leaderboard?page=0", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid page 0, got %d", w.Code)
	}

	// 4. Invalid limit (>100)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/p2p/leaderboard?limit=150", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for limit 150, got %d", w.Code)
	}

	// 5. Valid request
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/p2p/leaderboard?period=weekly&sort_by=wins&page=1&limit=20", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid request, got %d", w.Code)
	}

	var resp struct {
		Code    string                      `json:"code"`
		Success bool                        `json:"success"`
		Data    p2pdomain.LeaderboardResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success || resp.Code != "success" {
		t.Errorf("expected success response envelope, got code=%s, success=%v", resp.Code, resp.Success)
	}
	if resp.Data.Period != "weekly" {
		t.Errorf("expected period weekly, got %s", resp.Data.Period)
	}
	if resp.Data.Pagination == nil {
		t.Fatalf("expected pagination metadata")
	}
	if resp.Data.Pagination.Page != 1 || resp.Data.Pagination.Limit != 20 {
		t.Errorf("expected page 1 limit 20, got page=%d limit=%d", resp.Data.Pagination.Page, resp.Data.Pagination.Limit)
	}
}

func TestHTTP_Leaderboard_PrivacyAndNoLeakage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	secEmail := fmt.Sprintf("private-%d@secretcorp.com", time.Now().UnixNano())
	userID := createTestUser(t, db, "Secret Agent", secEmail)
	otherID := createTestUser(t, db, "", fmt.Sprintf("other-%d@domain.org", time.Now().UnixNano()))

	insertMatchWithPlayers(t, db, "tic-tac-toe", userID, otherID, &userID, p2pdomain.StatusCompleted, time.Now().UTC())

	gameRepo := p2prepo.NewGameRepo(db)
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	moveRepo := p2prepo.NewMoveRepo(db)
	settingsRepo := p2prepo.NewSettingsRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := application.NewP2PService(gameRepo, matchRepo, playerRepo, moveRepo, settingsRepo, escrowRepo, nil, nil)
	router := setupTestRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/p2p/leaderboard?period=all_time&limit=100", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	bodyStr := w.Body.String()
	// Must not contain user's private email
	if containsSubstring(bodyStr, "@secretcorp.com") {
		t.Errorf("CRITICAL PRIVACY VIOLATION: email domain found in leaderboard response: %s", bodyStr)
	}
	if containsSubstring(bodyStr, "@domain.org") {
		t.Errorf("CRITICAL PRIVACY VIOLATION: second user email found in leaderboard response: %s", bodyStr)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
