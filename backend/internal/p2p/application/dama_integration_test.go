package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
)

func damaBoardJSON(t *testing.T, pieces map[int]int, turn, seq, moveCount int) []byte {
	t.Helper()
	return damaBoardJSONWithClock(t, pieces, turn, seq, moveCount, 0)
}

func damaBoardJSONWithClock(t *testing.T, pieces map[int]int, turn, seq, moveCount, halfmoveClock int) []byte {
	t.Helper()
	cells := make([]int, 64)
	for index, value := range pieces {
		cells[index] = value
	}
	board := map[string]interface{}{
		"variant":            "ethiopian-dama",
		"board_size":         8,
		"cells":              cells,
		"turn":               turn,
		"turn_seq":           seq,
		"move_count":         moveCount,
		"halfmove_clock":     halfmoveClock,
		"turn_had_capture":   false,
		"must_continue_from": nil,
		"last_move":          nil,
		"move_history":       []interface{}{},
		"repetitions":        map[string]int{},
		"result":             nil,
	}
	raw, err := json.Marshal(board)
	if err != nil {
		t.Fatalf("failed to encode test board: %v", err)
	}
	return raw
}

func boardInt(t *testing.T, board map[string]interface{}, key string) int {
	t.Helper()
	value, ok := board[key]
	if !ok {
		t.Fatalf("board key %q is missing", key)
	}
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	default:
		t.Fatalf("board key %q has unexpected type %T", key, value)
		return 0
	}
}

func TestP2PService_DamaMatchLifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, false)

	creator, joiner, matchID := setupReadyMatch(t, db, svc, "ethiopian-dama", 0)
	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ReadyUp failed: %v", err)
	}
	started, err := svc.ReadyUp(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("joiner ReadyUp failed: %v", err)
	}
	if started.Status != p2pdomain.StatusInProgress {
		t.Fatalf("expected in_progress, got %s", started.Status)
	}
	if started.CurrentTurnUserID == nil || *started.CurrentTurnUserID != creator {
		t.Fatalf("creator should open the match")
	}
	if started.BoardState["variant"] != "ethiopian-dama" {
		t.Fatalf("unexpected board variant %v", started.BoardState["variant"])
	}
	if len(started.BoardState["legal_moves"].([]map[string]interface{})) != 7 {
		t.Fatalf("expected 7 opening moves, got %v", started.BoardState["legal_moves"])
	}

	// Drive the match to a capture-chain position directly. This keeps the test
	// deterministic without depending on a long opening sequence.
	if _, err := db.Exec(`UPDATE p2p_matches SET board_state = $1 WHERE id = $2`,
		string(damaBoardJSON(t, map[int]int{35: 1, 26: 3, 10: 3, 44: 3}, 0, 7, 0)), matchID); err != nil {
		t.Fatalf("failed to seed chain position: %v", err)
	}

	// First hop: the same player keeps the turn.
	afterFirst, err := svc.SubmitMove(ctx, creator, matchID, map[string]interface{}{
		"from": 35, "to": 17, "turn_seq": 7,
	})
	if err != nil {
		t.Fatalf("first capture hop rejected: %v", err)
	}
	if afterFirst.CurrentTurnUserID == nil || *afterFirst.CurrentTurnUserID != creator {
		t.Fatalf("turn must stay with the capturing player")
	}
	if boardInt(t, afterFirst.BoardState, "turn_seq") != 8 {
		t.Fatalf("expected turn_seq 8, got %v", afterFirst.BoardState["turn_seq"])
	}
	if boardInt(t, afterFirst.BoardState, "must_continue_from") != 17 {
		t.Fatalf("expected continuation from 17, got %v", afterFirst.BoardState["must_continue_from"])
	}

	// Replaying the previous hop is stale, not a second capture.
	if _, err := svc.SubmitMove(ctx, creator, matchID, map[string]interface{}{
		"from": 17, "to": 3, "turn_seq": 7,
	}); !errors.Is(err, p2pdomain.ErrStaleMove) {
		t.Fatalf("expected ErrStaleMove, got %v", err)
	}

	// The current sequence completes the chain and passes the turn.
	afterSecond, err := svc.SubmitMove(ctx, creator, matchID, map[string]interface{}{
		"from": 17, "to": 3, "turn_seq": 8,
	})
	if err != nil {
		t.Fatalf("second capture hop rejected: %v", err)
	}
	if afterSecond.CurrentTurnUserID == nil || *afterSecond.CurrentTurnUserID != joiner {
		t.Fatalf("turn should pass to the opponent after the chain")
	}
	if afterSecond.BoardState["must_continue_from"] != nil {
		t.Fatalf("continuation marker should be cleared")
	}

	// Both hops were persisted with increasing move numbers (the old int-only
	// extraction collapsed them to move 0 and lost rows to the unique index).
	var recorded int
	if err := db.QueryRow(`SELECT COUNT(*) FROM p2p_match_moves WHERE match_id = $1`, matchID).Scan(&recorded); err != nil {
		t.Fatalf("failed to count moves: %v", err)
	}
	if recorded != 2 {
		t.Fatalf("expected 2 recorded moves, got %d", recorded)
	}

	// Reconnect: the opponent can rebuild the exact authoritative state.
	reconnected, err := svc.GetMatchState(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	if reconnected.MyPlayerIndex == nil || *reconnected.MyPlayerIndex != 1 {
		t.Fatalf("expected joiner index 1, got %v", reconnected.MyPlayerIndex)
	}
	cells, ok := reconnected.BoardState["cells"].([]interface{})
	if !ok {
		t.Fatalf("reconnected cells have unexpected type %T", reconnected.BoardState["cells"])
	}
	if len(cells) != 64 {
		t.Fatalf("expected 64 reconnected cells, got %d", len(cells))
	}

	// An outsider cannot read a private match.
	outsider := createTestUserWithWallet(t, db, 0)
	if _, err := svc.GetMatchState(ctx, outsider, matchID); !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant for outsider, got %v", err)
	}

	// Resignation is server-validated and awards the opponent.
	resigned, err := svc.ResignMatch(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("resign failed: %v", err)
	}
	if resigned.Status != p2pdomain.StatusForfeited {
		t.Fatalf("expected forfeited, got %s", resigned.Status)
	}
	if resigned.WinnerUserID == nil || *resigned.WinnerUserID != creator {
		t.Fatalf("expected creator to win the forfeit")
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM p2p_matches WHERE id = $1`, matchID).Scan(&status); err != nil {
		t.Fatalf("failed to reload match: %v", err)
	}
	if status != p2pdomain.StatusForfeited {
		t.Fatalf("persisted status should be forfeited, got %s", status)
	}
}

func TestP2PService_DamaPaidMatchSettlement(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, true)

	creator, joiner, matchID := setupReadyMatch(t, db, svc, "ethiopian-dama", 50)
	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ReadyUp failed: %v", err)
	}
	if _, err := svc.ReadyUp(ctx, joiner, matchID); err != nil {
		t.Fatalf("joiner ReadyUp failed: %v", err)
	}

	// Both stakes are held in reserve while the match is live.
	var reserved float64
	if err := db.QueryRow(`SELECT reserved_balance FROM wallets WHERE user_id = $1`, creator).Scan(&reserved); err != nil {
		t.Fatalf("failed to read creator reservation: %v", err)
	}
	if reserved != 50 {
		t.Fatalf("expected 50 ETB reserved before settlement, got %v", reserved)
	}

	view, err := svc.ResignMatch(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("resign failed: %v", err)
	}
	if view.Status != p2pdomain.StatusForfeited || view.WinnerUserID == nil || *view.WinnerUserID != creator {
		t.Fatalf("expected creator forfeit win, got status=%s winner=%v", view.Status, view.WinnerUserID)
	}

	// Pool 100, configured fee 5% = 5, net payout 95. Creator: 500 - 50 + 95.
	var creatorBalance, creatorReserved, joinerBalance, joinerReserved float64
	if err := db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, creator).
		Scan(&creatorBalance, &creatorReserved); err != nil {
		t.Fatalf("failed to read creator wallet: %v", err)
	}
	if err := db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, joiner).
		Scan(&joinerBalance, &joinerReserved); err != nil {
		t.Fatalf("failed to read joiner wallet: %v", err)
	}
	if creatorBalance != 545 || creatorReserved != 0 {
		t.Fatalf("creator wallet = balance %v reserved %v, want 545/0", creatorBalance, creatorReserved)
	}
	if joinerBalance != 450 || joinerReserved != 0 {
		t.Fatalf("joiner wallet = balance %v reserved %v, want 450/0", joinerBalance, joinerReserved)
	}

	var feeCount int
	var feeAmount float64
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM wallet_transactions
		WHERE type = 'p2p_platform_fee' AND reference = $1`, "p2p_fee:"+matchID).
		Scan(&feeCount, &feeAmount); err != nil {
		t.Fatalf("failed to read platform fee: %v", err)
	}
	if feeCount != 1 || feeAmount != 5 {
		t.Fatalf("expected one 5 ETB fee row, got count=%d amount=%v", feeCount, feeAmount)
	}

	// A second resignation must not settle the match twice.
	if _, err := svc.ResignMatch(ctx, joiner, matchID); err == nil {
		t.Fatalf("expected a second resignation to be rejected")
	}
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, creator).Scan(&creatorBalance); err != nil {
		t.Fatalf("failed to reread creator wallet: %v", err)
	}
	if creatorBalance != 545 {
		t.Fatalf("duplicate settlement changed the creator balance to %v", creatorBalance)
	}
}

func TestP2PService_DamaPaidDrawRefunds(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, true)

	creator, joiner, matchID := setupReadyMatch(t, db, svc, "ethiopian-dama", 50)
	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ReadyUp failed: %v", err)
	}
	if _, err := svc.ReadyUp(ctx, joiner, matchID); err != nil {
		t.Fatalf("joiner ReadyUp failed: %v", err)
	}

	// Two kings and one legal shuttle away from the 50-move draw. The next
	// non-capturing turn completes the clock and must refund both stakes.
	if _, err := db.Exec(`UPDATE p2p_matches SET board_state = $1 WHERE id = $2`,
		string(damaBoardJSONWithClock(t, map[int]int{1: 2, 62: 4}, 0, 0, 0, 99)), matchID); err != nil {
		t.Fatalf("failed to seed draw position: %v", err)
	}

	view, err := svc.SubmitMove(ctx, creator, matchID, map[string]interface{}{
		"from": 1, "to": 10, "turn_seq": 0,
	})
	if err != nil {
		t.Fatalf("drawing move rejected: %v", err)
	}
	if view.Status != p2pdomain.StatusTied {
		t.Fatalf("expected tied, got %s", view.Status)
	}
	if view.WinnerUserID != nil {
		t.Fatalf("a draw must not have a winner, got %v", view.WinnerUserID)
	}

	var creatorBalance, creatorReserved, joinerBalance, joinerReserved float64
	if err := db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, creator).
		Scan(&creatorBalance, &creatorReserved); err != nil {
		t.Fatalf("failed to read creator wallet: %v", err)
	}
	if err := db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, joiner).
		Scan(&joinerBalance, &joinerReserved); err != nil {
		t.Fatalf("failed to read joiner wallet: %v", err)
	}
	if creatorBalance != 500 || creatorReserved != 0 {
		t.Fatalf("creator wallet after draw = balance %v reserved %v, want 500/0", creatorBalance, creatorReserved)
	}
	if joinerBalance != 500 || joinerReserved != 0 {
		t.Fatalf("joiner wallet after draw = balance %v reserved %v, want 500/0", joinerBalance, joinerReserved)
	}

	var feeCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM wallet_transactions
		WHERE type = 'p2p_platform_fee' AND reference = $1`, "p2p_fee:"+matchID).Scan(&feeCount); err != nil {
		t.Fatalf("failed to count platform fees: %v", err)
	}
	if feeCount != 0 {
		t.Fatalf("a drawn match must not charge a platform fee, got %d rows", feeCount)
	}
}

func TestP2PService_DamaConcurrentMoves(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, false)

	creator, joiner, matchID := setupReadyMatch(t, db, svc, "ethiopian-dama", 0)
	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ReadyUp failed: %v", err)
	}
	if _, err := svc.ReadyUp(ctx, joiner, matchID); err != nil {
		t.Fatalf("joiner ReadyUp failed: %v", err)
	}

	// Two simultaneous submissions of the same opening move. The row lock must
	// let exactly one through; the other sees a changed turn.
	action := map[string]interface{}{"from": 40, "to": 33, "turn_seq": 0}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.SubmitMove(context.Background(), creator, matchID, action)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes, rejections := 0, 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, p2pdomain.ErrNotYourTurn) || errors.Is(err, p2pdomain.ErrStaleMove) {
			rejections++
			continue
		}
		t.Fatalf("unexpected concurrent move error: %v", err)
	}
	if successes != 1 || rejections != 1 {
		t.Fatalf("expected one accepted and one rejected move, got successes=%d rejections=%d", successes, rejections)
	}

	var recorded int
	if err := db.QueryRow(`SELECT COUNT(*) FROM p2p_match_moves WHERE match_id = $1`, matchID).Scan(&recorded); err != nil {
		t.Fatalf("failed to count moves: %v", err)
	}
	if recorded != 1 {
		t.Fatalf("expected exactly one recorded move, got %d", recorded)
	}

	state, err := svc.GetMatchState(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if state.BoardState["turn"] == nil {
		t.Fatalf("board turn missing after concurrent move")
	}
}
