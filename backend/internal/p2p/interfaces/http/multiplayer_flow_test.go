package http

// End-to-end multiplayer flow test — the regression guard for the reported bug
// ("one player is stuck on Connecting… while the other plays and wins").
//
// Two authenticated players drive a real match through the real HTTP API while
// each holds an independent SSE stream to the realtime endpoint. The test
// asserts they agree on membership, seat, readiness, turn ownership and board
// state at every step, and that duplicate/stale/illegal moves cannot corrupt
// the match.
//
// It skips automatically when PostgreSQL or Redis is unavailable, like the rest
// of the DB-backed suites.

import (
	"bufio"
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
	"github.com/raffle-app/backend/internal/p2p/application"
	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
	rt "github.com/raffle-app/backend/internal/realtime"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
	"go.uber.org/zap"
)

const multiplayerTestSecret = "test-jwt-secret-for-handlers-test-12345"

// ─── Test harness ─────────────────────────────────────────────────────────────

type testPlayer struct {
	name    string
	userID  string
	token   string
	stream  *sseReader
	baseURL string
}

func setupMultiplayerServer(t *testing.T) (*httptest.Server, *sql.DB, string) {
	t.Helper()

	db := setupTestDB(t)

	rdb, err := database.NewRedis(config.RedisConfig{Host: "localhost", Port: "6379"})
	if err != nil {
		db.Close()
		t.Skipf("skipping multiplayer integration test, redis not available: %v", err)
	}

	gin.SetMode(gin.TestMode)

	rtSvc := rt.NewService(rdb, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	go rtSvc.Start(ctx)
	t.Cleanup(cancel)

	svc := application.NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		rtSvc,
		nil,
	)

	router := gin.New()
	api := router.Group("/api/v1")
	RegisterP2PRoutes(api, NewP2PHandler(svc))
	api.GET("/realtime/stream", rt.NewHandler(rtSvc, multiplayerTestSecret, zap.NewNop()).Stream)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return srv, db, multiplayerTestSecret
}

// newTestPlayer registers a user row, mints a JWT for it and opens an SSE
// stream for the given channel.
func newTestPlayer(t *testing.T, db *sql.DB, srv *httptest.Server, name, channel string) *testPlayer {
	t.Helper()

	userID := createTestUser(t, db, name, fmt.Sprintf("%s-%d@example.com", name, time.Now().UnixNano()))
	token := makeTestJWT(t, userID)

	player := &testPlayer{name: name, userID: userID, token: token, baseURL: srv.URL}
	if channel != "" {
		player.stream = openSSE(t, srv.URL, channel, token)
	}
	return player
}

func (p *testPlayer) do(t *testing.T, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()

	var payload *strings.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		payload = strings.NewReader(string(encoded))
	} else {
		payload = strings.NewReader("")
	}

	req, err := http.NewRequest(method, p.baseURL+path, payload)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	decoded := map[string]interface{}{}
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		decoded = map[string]interface{}{}
	}
	return res.StatusCode, decoded
}

// ─── SSE client ───────────────────────────────────────────────────────────────

type sseFrame struct {
	event string
	data  map[string]interface{}
	at    time.Time
}

type sseReader struct {
	frames chan sseFrame
	cancel context.CancelFunc
}

func openSSE(t *testing.T, baseURL, channel, token string) *sseReader {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	url := fmt.Sprintf("%s/api/v1/realtime/stream?channel=%s&token=%s", baseURL, channel, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		t.Fatalf("build SSE request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("open SSE stream: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		cancel()
		t.Fatalf("SSE stream returned %d (a rejected stream surfaces to players as a permanent \"Connecting…\")", res.StatusCode)
	}

	reader := &sseReader{frames: make(chan sseFrame, 128), cancel: cancel}

	go func() {
		defer res.Body.Close()
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var event string
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				event = ""
			case strings.HasPrefix(line, "event:"):
				event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if raw == "" {
					continue
				}
				decoded := map[string]interface{}{}
				if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
					continue
				}
				if event == "connected" || event == "ping" {
					continue
				}
				select {
				case reader.frames <- sseFrame{event: event, data: decoded, at: time.Now()}:
				default:
				}
			}
		}
	}()

	t.Cleanup(reader.cancel)
	return reader
}

// waitFor blocks until a frame of the given realtime type arrives.
func (r *sseReader) waitFor(t *testing.T, eventType string, timeout time.Duration) (sseFrame, bool) {
	t.Helper()
	return r.waitForMatching(t, eventType, nil, timeout)
}

// waitForMatching blocks until an event of `eventType` also satisfies `pred`.
// Both players receive their own copy of every state event, so a test that
// waits for "the next move" must be able to ignore the player's own move.
func (r *sseReader) waitForMatching(
	t *testing.T,
	eventType string,
	pred func(sseFrame) bool,
	timeout time.Duration,
) (sseFrame, bool) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case frame := <-r.frames:
			kind, _ := frame.data["type"].(string)
			if kind != eventType {
				continue
			}
			if pred == nil || pred(frame) {
				return frame, true
			}
		case <-deadline:
			return sseFrame{}, false
		}
	}
}

// stateOf extracts the match view that state-bearing events carry under
// payload.state (the SSE frame wraps the realtime Event, which wraps the view).
func stateOf(t *testing.T, frame sseFrame) map[string]interface{} {
	t.Helper()
	payload, ok := frame.data["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("event %v carried no payload: %v", frame.data["type"], frame.data)
	}
	state, ok := payload["state"].(map[string]interface{})
	if !ok {
		t.Fatalf("event %v carried no per-player state: %v", frame.data["type"], payload)
	}
	return state
}

func dataOf(t *testing.T, response map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response carried no data: %v", response)
	}
	return data
}

func playerIndex(t *testing.T, state map[string]interface{}, userID string) int {
	t.Helper()
	players, ok := state["players"].([]interface{})
	if !ok {
		t.Fatalf("state carried no players: %v", state)
	}
	for _, raw := range players {
		player, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if player["user_id"] == userID {
			idx, _ := player["player_index"].(float64)
			return int(idx)
		}
	}
	return -1
}

// boardCell returns the occupancy marker of a Tic-Tac-Toe cell:
// 0 = empty, 1 = host (X), 2 = guest (O).
func boardCell(t *testing.T, state map[string]interface{}, index int) int {
	t.Helper()
	board, ok := state["board_state"].(map[string]interface{})
	if !ok {
		return -1
	}
	cells, ok := board["cells"].([]interface{})
	if !ok || index >= len(cells) {
		return -1
	}
	value, _ := cells[index].(float64)
	return int(value)
}

// ─── The regression test ──────────────────────────────────────────────────────

func TestMultiplayer_TwoClientsStayInSync(t *testing.T) {
	srv, db, _ := setupMultiplayerServer(t)
	defer db.Close()

	host := newTestPlayer(t, db, srv, "host", "")
	guest := newTestPlayer(t, db, srv, "guest", "")

	// 1. The host opens a private room.
	status, created := host.do(t, http.MethodPost, "/api/v1/p2p/matches", map[string]interface{}{
		"game_slug":    "tic-tac-toe",
		"match_type":   "private_room",
		"stake_amount": 0,
	})
	if status != http.StatusCreated {
		t.Fatalf("create match: expected 201, got %d (%v)", status, created)
	}
	match := dataOf(t, created)
	matchID, _ := match["id"].(string)
	inviteCode, _ := match["invite_code"].(string)
	if matchID == "" || inviteCode == "" {
		t.Fatalf("create match returned no id/code: %v", match)
	}

	// Both players subscribe to the match channel, exactly like the browser does
	// once it is on the match page.
	host.stream = openSSE(t, srv.URL, "p2p_match:"+matchID, host.token)
	guest.stream = openSSE(t, srv.URL, "p2p_match:"+matchID, guest.token)

	// 2. The guest joins by invite code and receives an explicit acknowledgement
	//    naming the same match.
	status, joined := guest.do(t, http.MethodPost, "/api/v1/p2p/matches/join-code", map[string]interface{}{
		"invite_code": inviteCode,
	})
	if status != http.StatusOK {
		t.Fatalf("join by code: expected 200, got %d (%v)", status, joined)
	}
	guestView := dataOf(t, joined)
	if guestView["id"] != matchID {
		t.Fatalf("join ack named match %v, expected %s", guestView["id"], matchID)
	}
	if got := playerIndex(t, guestView, guest.userID); got != 1 {
		t.Fatalf("guest seat: expected player_index 1, got %d", got)
	}

	// 3. Repeated joins are idempotent: no duplicate membership, same seat.
	status, rejoined := guest.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/join", nil)
	if status != http.StatusOK {
		t.Fatalf("idempotent re-join: expected 200, got %d (%v)", status, rejoined)
	}
	rejoinView := dataOf(t, rejoined)
	if players, _ := rejoinView["players"].([]interface{}); len(players) != 2 {
		t.Fatalf("re-join created a duplicate membership: %d players", len(players))
	}

	// 4. The host is told about the join over its own stream, with its own seat.
	joinFrame, ok := host.stream.waitFor(t, "p2p.player_joined", 3*time.Second)
	if !ok {
		t.Fatal("host never received p2p.player_joined — this is the client-visible half of the stuck-Connecting bug")
	}
	hostState := stateOf(t, joinFrame)
	if got := playerIndex(t, hostState, host.userID); got != 0 {
		t.Fatalf("host seat: expected player_index 0, got %d", got)
	}
	if players, _ := hostState["players"].([]interface{}); len(players) != 2 {
		t.Fatalf("host view reports %d players after the guest joined", len(players))
	}

	// 5. Both players ready up. The match must not start until both have.
	status, _ = host.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/ready", nil)
	if status != http.StatusOK {
		t.Fatalf("host ready: expected 200, got %d", status)
	}
	if frame, ok := host.stream.waitFor(t, "p2p.match_started", 500*time.Millisecond); ok {
		t.Fatalf("match started after only one player readied up: %v", frame.data)
	}

	status, guestReady := guest.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/ready", nil)
	if status != http.StatusOK {
		t.Fatalf("guest ready: expected 200, got %d", status)
	}
	if got, _ := dataOf(t, guestReady)["status"].(string); got != p2pdomain.StatusInProgress {
		t.Fatalf("guest ready response status: expected in_progress, got %q", got)
	}

	// 6. Both clients receive exactly one authoritative start event naming the
	//    same match and the same opening turn, each with their own seat.
	hostStart, okHost := host.stream.waitFor(t, "p2p.match_started", 3*time.Second)
	guestStart, okGuest := guest.stream.waitFor(t, "p2p.match_started", 3*time.Second)
	if !okHost || !okGuest {
		t.Fatalf("match_started delivery: host=%v guest=%v", okHost, okGuest)
	}
	hostStartState := stateOf(t, hostStart)
	guestStartState := stateOf(t, guestStart)

	if hostStartState["id"] != matchID || guestStartState["id"] != matchID {
		t.Fatalf("clients disagree on the match id: %v vs %v", hostStartState["id"], guestStartState["id"])
	}
	if hostStartState["current_turn_user_id"] != guestStartState["current_turn_user_id"] {
		t.Fatalf("clients disagree on the opening turn: %v vs %v",
			hostStartState["current_turn_user_id"], guestStartState["current_turn_user_id"])
	}
	if hostStartState["current_turn_user_id"] != host.userID {
		t.Fatalf("opening turn should belong to the host (player 0), got %v", hostStartState["current_turn_user_id"])
	}
	if got := playerIndex(t, hostStartState, host.userID); got != 0 {
		t.Fatalf("host my_player_index in the start event: expected 0, got %d", got)
	}
	if got := playerIndex(t, guestStartState, guest.userID); got != 1 {
		t.Fatalf("guest my_player_index in the start event: expected 1, got %d", got)
	}
	if hostStartState["turn_deadline_at"] == nil {
		t.Fatal("start event carried no absolute turn deadline for the client clock")
	}

	// 7. A move appears on the opponent's stream promptly.
	moveStart := time.Now()
	status, moved := host.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/move", map[string]interface{}{
		"cell": 0,
	})
	if status != http.StatusOK {
		t.Fatalf("host move: expected 200, got %d (%v)", status, moved)
	}
	moveFrame, ok := guest.stream.waitFor(t, "p2p.move_made", 3*time.Second)
	if !ok {
		t.Fatal("the opponent never received the move — the board would silently diverge")
	}
	delivery := time.Since(moveStart)
	t.Logf("move delivery (host submit -> guest SSE frame): %v", delivery)
	if delivery > 3*time.Second {
		t.Fatalf("move delivery too slow: %v", delivery)
	}

	guestBoard := stateOf(t, moveFrame)
	if cell := boardCell(t, guestBoard, 0); cell != 1 {
		t.Fatalf("guest board did not show the host's move: cell 0 = %d", cell)
	}
	if guestBoard["current_turn_user_id"] != guest.userID {
		t.Fatalf("turn did not pass to the guest: %v", guestBoard["current_turn_user_id"])
	}

	// 8. Out-of-turn and duplicate moves are rejected without corrupting state.
	status, rejected := host.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/move", map[string]interface{}{
		"cell": 1,
	})
	if status != http.StatusConflict {
		t.Fatalf("out-of-turn move: expected 409, got %d (%v)", status, rejected)
	}

	status, _ = guest.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/move", map[string]interface{}{
		"cell": 0,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("move onto an occupied cell: expected 400, got %d", status)
	}

	// The rejected moves must not have changed the board for either player.
	status, stateResp := host.do(t, http.MethodGet, "/api/v1/p2p/matches/"+matchID+"/state", nil)
	if status != http.StatusOK {
		t.Fatalf("fetch state: expected 200, got %d", status)
	}
	authoritative := dataOf(t, stateResp)
	if cell := boardCell(t, authoritative, 0); cell != 1 {
		t.Fatalf("rejected moves corrupted the board: cell 0 = %d", cell)
	}
	if cell := boardCell(t, authoritative, 1); cell != 0 {
		t.Fatalf("rejected moves corrupted the board: cell 1 = %d", cell)
	}
	if authoritative["current_turn_user_id"] != guest.userID {
		t.Fatalf("a rejected move advanced the turn to %v", authoritative["current_turn_user_id"])
	}

	// 9. The guest answers; the host sees it. Both boards now agree.
	status, _ = guest.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/move", map[string]interface{}{
		"cell": 4,
	})
	if status != http.StatusOK {
		t.Fatalf("guest move: expected 200, got %d", status)
	}
	hostMoveFrame, ok := host.stream.waitForMatching(t, "p2p.move_made", func(frame sseFrame) bool {
		payload, _ := frame.data["payload"].(map[string]interface{})
		state, _ := payload["state"].(map[string]interface{})
		return state != nil && boardCell(t, state, 4) == 2
	}, 3*time.Second)
	if !ok {
		t.Fatal("the host never saw the guest's reply")
	}
	if cell := boardCell(t, stateOf(t, hostMoveFrame), 4); cell != 2 {
		t.Fatalf("host board did not show the guest's reply: cell 4 = %d", cell)
	}

	// 10. Explicit resignation is a distinct path from a timeout, and the match
	//     cannot accept further moves once it is finished.
	status, _ = host.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/resign", nil)
	if status != http.StatusOK {
		t.Fatalf("resign: expected 200, got %d", status)
	}
	status, finalState := guest.do(t, http.MethodGet, "/api/v1/p2p/matches/"+matchID+"/state", nil)
	if status != http.StatusOK {
		t.Fatalf("final state: expected 200, got %d", status)
	}
	final := dataOf(t, finalState)
	if got, _ := final["status"].(string); got != p2pdomain.StatusForfeited {
		t.Fatalf("expected forfeited after resignation, got %q", got)
	}
	if final["winner_user_id"] != guest.userID {
		t.Fatalf("resignation did not award the win to the opponent: %v", final["winner_user_id"])
	}

	status, afterEnd := guest.do(t, http.MethodPost, "/api/v1/p2p/matches/"+matchID+"/move", map[string]interface{}{
		"cell": 8,
	})
	if status != http.StatusConflict {
		t.Fatalf("move after match end: expected 409, got %d (%v)", status, afterEnd)
	}

	// 11. Both clients converge on the same terminal result.
	hostEnd, okHostEnd := host.stream.waitFor(t, "p2p.match_completed", 3*time.Second)
	guestEnd, okGuestEnd := guest.stream.waitFor(t, "p2p.match_completed", 3*time.Second)
	if !okHostEnd || !okGuestEnd {
		t.Fatalf("terminal event delivery: host=%v guest=%v", okHostEnd, okGuestEnd)
	}
	if stateOf(t, hostEnd)["status"] != stateOf(t, guestEnd)["status"] {
		t.Fatalf("clients disagree on the final status: %v vs %v",
			stateOf(t, hostEnd)["status"], stateOf(t, guestEnd)["status"])
	}
}

// TestMultiplayer_IndependentStreamsPerPlayer proves a player can reconnect
// without stealing the opponent's stream, and that a second connection for the
// same user+channel replaces the first instead of being rejected. A rejected
// stream used to leave a reconnecting player stuck on "Connecting…" forever.
func TestMultiplayer_ReconnectDoesNotBreakOpponent(t *testing.T) {
	srv, db, _ := setupMultiplayerServer(t)
	defer db.Close()

	host := newTestPlayer(t, db, srv, "host", "")
	guest := newTestPlayer(t, db, srv, "guest", "")

	status, created := host.do(t, http.MethodPost, "/api/v1/p2p/matches", map[string]interface{}{
		"game_slug":    "tic-tac-toe",
		"match_type":   "private_room",
		"stake_amount": 0,
	})
	if status != http.StatusCreated {
		t.Fatalf("create match: expected 201, got %d", status)
	}
	match := dataOf(t, created)
	matchID, _ := match["id"].(string)

	host.stream = openSSE(t, srv.URL, "p2p_match:"+matchID, host.token)
	guest.stream = openSSE(t, srv.URL, "p2p_match:"+matchID, guest.token)

	// The guest's tab reloads: a second stream for the same user+channel opens
	// while the first is still registered on the server. The server must accept
	// it (replacing the old connection) instead of answering 429 — openSSE
	// fails the test on any non-200 response.
	guest.stream = openSSE(t, srv.URL, "p2p_match:"+matchID, guest.token)

	// The host's stream must be unaffected and still deliver events.
	status, joined := guest.do(t, http.MethodPost, "/api/v1/p2p/matches/join-code", map[string]interface{}{
		"invite_code": match["invite_code"],
	})
	if status != http.StatusOK {
		t.Fatalf("join: expected 200, got %d (%v)", status, joined)
	}
	if _, ok := host.stream.waitFor(t, "p2p.player_joined", 3*time.Second); !ok {
		t.Fatal("host lost its stream while the guest reconnected")
	}
}
