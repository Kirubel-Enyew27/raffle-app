package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/raffle-app/backend/internal/p2p/application"
	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"github.com/spf13/viper"
)

func makeTestJWT(t *testing.T, userID string) string {
	t.Helper()
	secret := "test-jwt-secret-for-handlers-test-12345"
	viper.Set("JWT_SECRET", secret)
	token, err := appjwt.GenerateTokenHMAC(userID, "user@example.com", "user", []byte(secret), time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test JWT: %v", err)
	}
	return token
}

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

func TestHTTP_GetInvite_PublicAndNotFound(t *testing.T) {
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

	// 1. Not found invite
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/p2p/invites/INVALID", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid invite, got %d", w.Code)
	}

	// 2. Create private match and preview via GET /invites/:code
	user := createTestUser(t, db, "Host Player", fmt.Sprintf("host-%d@example.com", time.Now().UnixNano()))
	match, err := svc.CreateMatch(context.Background(), user, application.CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/p2p/invites/"+match.InviteCode, nil)
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid invite, got %d", w2.Code)
	}

	var resp struct {
		Data application.InviteSummary `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.InviteCode != match.InviteCode {
		t.Errorf("expected invite code %s, got %s", match.InviteCode, resp.Data.InviteCode)
	}
	if !resp.Data.IsJoinable {
		t.Errorf("expected is_joinable true")
	}
}

func TestHTTP_JoinMatchByCode_EndToEnd(t *testing.T) {
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

	hostID := createTestUser(t, db, "Host Player", fmt.Sprintf("host-%d@example.com", time.Now().UnixNano()))
	joinerID := createTestUser(t, db, "Joiner Player", fmt.Sprintf("joiner-%d@example.com", time.Now().UnixNano()))
	thirdID := createTestUser(t, db, "Third Player", fmt.Sprintf("third-%d@example.com", time.Now().UnixNano()))

	// Host creates match
	match, err := svc.CreateMatch(context.Background(), hostID, application.CreateMatchRequest{
		GameSlug:    "connect-four",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// 1. Preview invite by code (public)
	wPreview := httptest.NewRecorder()
	reqPreview, _ := http.NewRequest(http.MethodGet, "/api/v1/p2p/invites/"+match.InviteCode, nil)
	router.ServeHTTP(wPreview, reqPreview)
	if wPreview.Code != http.StatusOK {
		t.Fatalf("expected 200 for invite preview, got %d", wPreview.Code)
	}

	// 2. Host tries to join own match by code -> 409 CANNOT_JOIN_OWN_MATCH
	hostToken := makeTestJWT(t, hostID)
	wHostJoin := httptest.NewRecorder()
	bodyHost, _ := json.Marshal(map[string]string{"invite_code": match.InviteCode})
	reqHostJoin, _ := http.NewRequest(http.MethodPost, "/api/v1/p2p/matches/join-code", bytes.NewReader(bodyHost))
	reqHostJoin.Header.Set("Authorization", "Bearer "+hostToken)
	reqHostJoin.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wHostJoin, reqHostJoin)
	if wHostJoin.Code != http.StatusConflict {
		t.Fatalf("expected 409 for host joining own match, got %d (%s)", wHostJoin.Code, wHostJoin.Body.String())
	}

	// 3. Non-existent code -> 404 NOT_FOUND
	joinerToken := makeTestJWT(t, joinerID)
	wNotFound := httptest.NewRecorder()
	bodyNotFound, _ := json.Marshal(map[string]string{"invite_code": "NONEX1"})
	reqNotFound, _ := http.NewRequest(http.MethodPost, "/api/v1/p2p/matches/join-code", bytes.NewReader(bodyNotFound))
	reqNotFound.Header.Set("Authorization", "Bearer "+joinerToken)
	reqNotFound.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wNotFound, reqNotFound)
	if wNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid code join, got %d (%s)", wNotFound.Code, wNotFound.Body.String())
	}

	// 4. Joiner joins successfully using lowercase/whitespace code -> 200 OK
	wJoin := httptest.NewRecorder()
	bodyJoin, _ := json.Marshal(map[string]string{"invite_code": "  " + strings.ToLower(match.InviteCode) + "  "})
	reqJoin, _ := http.NewRequest(http.MethodPost, "/api/v1/p2p/matches/join-code", bytes.NewReader(bodyJoin))
	reqJoin.Header.Set("Authorization", "Bearer "+joinerToken)
	reqJoin.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wJoin, reqJoin)
	if wJoin.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid join, got %d (%s)", wJoin.Code, wJoin.Body.String())
	}

	var joinResp struct {
		Data application.MatchView `json:"data"`
	}
	if err := json.Unmarshal(wJoin.Body.Bytes(), &joinResp); err != nil {
		t.Fatalf("failed to decode join response: %v", err)
	}
	if joinResp.Data.ID != match.ID {
		t.Errorf("expected match ID %s, got %s", match.ID, joinResp.Data.ID)
	}
	if joinResp.Data.Status != p2pdomain.StatusReady {
		t.Errorf("expected status ready, got %s", joinResp.Data.Status)
	}
	if len(joinResp.Data.Players) != 2 {
		t.Errorf("expected 2 players, got %d", len(joinResp.Data.Players))
	}

	// 5. Third player tries to join -> 409 conflict
	thirdToken := makeTestJWT(t, thirdID)
	wThird := httptest.NewRecorder()
	bodyThird, _ := json.Marshal(map[string]string{"invite_code": match.InviteCode})
	reqThird, _ := http.NewRequest(http.MethodPost, "/api/v1/p2p/matches/join-code", bytes.NewReader(bodyThird))
	reqThird.Header.Set("Authorization", "Bearer "+thirdToken)
	reqThird.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wThird, reqThird)
	if wThird.Code != http.StatusConflict {
		t.Fatalf("expected 409 for third player join, got %d (%s)", wThird.Code, wThird.Body.String())
	}
}
