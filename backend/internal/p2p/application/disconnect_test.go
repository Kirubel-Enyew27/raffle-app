package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
	"go.uber.org/zap"
)

// setupInProgressMatch creates two users, creates and joins a match, both
// ready up, and returns the IDs once the match is in_progress.
func setupInProgressMatch(t *testing.T, db *sql.DB, svc *P2PService, gameSlug string) (creator, joiner, matchID string) {
	t.Helper()
	ctx := context.Background()
	creator = createTestUserWithWallet(t, db, 500)
	joiner = createTestUserWithWallet(t, db, 500)

	m, err := svc.CreateMatch(ctx, creator, CreateMatchRequest{
		GameSlug:  gameSlug,
		MatchType: p2pdomain.TypePublicQueue,
	})
	if err != nil {
		t.Fatalf("CreateMatch: %v", err)
	}
	if _, err := svc.JoinMatch(ctx, joiner, m.ID, ""); err != nil {
		t.Fatalf("JoinMatch: %v", err)
	}
	if _, err := svc.ReadyUp(ctx, creator, m.ID); err != nil {
		t.Fatalf("creator ReadyUp: %v", err)
	}
	started, err := svc.ReadyUp(ctx, joiner, m.ID)
	if err != nil {
		t.Fatalf("joiner ReadyUp: %v", err)
	}
	if started.Status != p2pdomain.StatusInProgress {
		t.Fatalf("expected in_progress, got %s", started.Status)
	}
	return creator, joiner, m.ID
}

// ─── Disconnect / Reconnect ───────────────────────────────────────────────────

func TestHandlePlayerDisconnect_SetsDeadline(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	view, err := svc.HandlePlayerDisconnect(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("HandlePlayerDisconnect: %v", err)
	}
	if view.DisconnectUserID == nil || *view.DisconnectUserID != creator {
		t.Fatalf("expected disconnect_user_id=%s, got %v", creator, view.DisconnectUserID)
	}
	if view.DisconnectSecondsLeft <= 0 {
		t.Fatalf("expected disconnect_seconds_left > 0, got %d", view.DisconnectSecondsLeft)
	}

	// Verify the deadline is persisted in the DB.
	var ddl sql.NullTime
	if err := db.QueryRow(`SELECT disconnect_deadline FROM p2p_matches WHERE id = $1`, matchID).Scan(&ddl); err != nil {
		t.Fatalf("failed to query deadline: %v", err)
	}
	if !ddl.Valid {
		t.Fatal("disconnect_deadline must be set in DB")
	}
	expectedDeadline := time.Now().Add(time.Duration(DisconnectGraceSeconds) * time.Second)
	diff := ddl.Time.Sub(expectedDeadline).Abs()
	if diff > 2*time.Second {
		t.Fatalf("deadline is off by %v", diff)
	}
}

func TestHandlePlayerDisconnect_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	first, err := svc.HandlePlayerDisconnect(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("first disconnect: %v", err)
	}
	// A second disconnect call while the deadline is still active must not
	// reset the deadline (that would extend the grace period unfairly).
	second, err := svc.HandlePlayerDisconnect(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("second disconnect: %v", err)
	}
	// The reported seconds-left may differ by at most 1 second due to timing.
	diff := first.DisconnectSecondsLeft - second.DisconnectSecondsLeft
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		t.Fatalf("idempotent disconnect must not extend deadline (first=%d, second=%d)",
			first.DisconnectSecondsLeft, second.DisconnectSecondsLeft)
	}
}

func TestHandlePlayerDisconnect_RequiresParticipant(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	_, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	outsider := createTestUserWithWallet(t, db, 100)

	_, err := svc.HandlePlayerDisconnect(context.Background(), outsider, matchID)
	if !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant, got %v", err)
	}
}

func TestHandlePlayerDisconnect_RejectsFinishedMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	// Resign to finish the match.
	if _, err := svc.ResignMatch(ctx, creator, matchID); err != nil {
		t.Fatalf("resign: %v", err)
	}

	_, err := svc.HandlePlayerDisconnect(ctx, creator, matchID)
	if !errors.Is(err, p2pdomain.ErrMatchAlreadyEnded) {
		t.Fatalf("expected ErrMatchAlreadyEnded, got %v", err)
	}
}

func TestHandlePlayerReconnect_ClearsDeadline(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	if _, err := svc.HandlePlayerDisconnect(ctx, creator, matchID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	view, err := svc.HandlePlayerReconnect(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if view.DisconnectUserID != nil {
		t.Fatalf("expected disconnect_user_id to be cleared, got %v", view.DisconnectUserID)
	}
	if view.DisconnectSecondsLeft != 0 {
		t.Fatalf("expected disconnect_seconds_left=0, got %d", view.DisconnectSecondsLeft)
	}

	// Verify DB is cleared.
	var uid sql.NullString
	var ddl sql.NullTime
	if err := db.QueryRow(`SELECT disconnect_user_id, disconnect_deadline FROM p2p_matches WHERE id = $1`, matchID).Scan(&uid, &ddl); err != nil {
		t.Fatalf("query: %v", err)
	}
	if uid.Valid || ddl.Valid {
		t.Fatalf("expected both disconnect columns to be NULL in DB, got uid=%v ddl=%v", uid, ddl)
	}
}

func TestHandlePlayerReconnect_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	// Reconnect without a preceding disconnect must succeed silently (idempotent).
	view, err := svc.HandlePlayerReconnect(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("reconnect without prior disconnect: %v", err)
	}
	if view.Status != p2pdomain.StatusInProgress {
		t.Fatalf("expected in_progress, got %s", view.Status)
	}
}

func TestHandlePlayerReconnect_WrongPlayerIsNoop(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, joiner, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	// Mark creator as disconnected.
	if _, err := svc.HandlePlayerDisconnect(ctx, creator, matchID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	// The joiner "reconnects" (but they were never disconnected): this must not
	// clear the creator's deadline.
	view, err := svc.HandlePlayerReconnect(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("reconnect by wrong player: %v", err)
	}
	if view.DisconnectUserID == nil || *view.DisconnectUserID != creator {
		t.Fatalf("wrong-player reconnect must not clear the creator's disconnect, got %v", view.DisconnectUserID)
	}
}

func TestHandlePlayerReconnect_RejectsFinishedMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	if _, err := svc.ResignMatch(ctx, creator, matchID); err != nil {
		t.Fatalf("resign: %v", err)
	}

	_, err := svc.HandlePlayerReconnect(ctx, creator, matchID)
	if !errors.Is(err, p2pdomain.ErrMatchAlreadyEnded) {
		t.Fatalf("expected ErrMatchAlreadyEnded, got %v", err)
	}
}

// ─── Engine: disconnect timeout ───────────────────────────────────────────────

func TestEngine_DisconnectTimeout_ForfeitsMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := newTestService(t, db, false)
	creator, joiner, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")

	// Force the deadline into the past.
	past := time.Now().Add(-1 * time.Second)
	_, err := db.Exec(`UPDATE p2p_matches SET disconnect_user_id = $1, disconnect_deadline = $2 WHERE id = $3`,
		creator, past, matchID)
	if err != nil {
		t.Fatalf("failed to force disconnect deadline: %v", err)
	}

	engine := newNopEngine(matchRepo, playerRepo, escrowRepo)

	engine.sweepDisconnectTimeouts(ctx)

	var status string
	var winner sql.NullString
	if err := db.QueryRow(`SELECT status, winner_user_id FROM p2p_matches WHERE id = $1`, matchID).
		Scan(&status, &winner); err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != p2pdomain.StatusForfeited {
		t.Fatalf("expected forfeited, got %s", status)
	}
	if !winner.Valid || winner.String != joiner {
		t.Fatalf("expected joiner to win, got %v", winner)
	}
}

func TestEngine_DisconnectTimeout_IdempotentOnAlreadyForfeited(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := newTestService(t, db, false)
	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")

	// Resign first to finish the match normally.
	if _, err := svc.ResignMatch(ctx, creator, matchID); err != nil {
		t.Fatalf("resign: %v", err)
	}

	// Force a disconnect deadline in the past to simulate a stale sweep entry.
	past := time.Now().Add(-1 * time.Second)
	_, _ = db.Exec(`UPDATE p2p_matches SET disconnect_user_id = $1, disconnect_deadline = $2 WHERE id = $3`,
		creator, past, matchID)

	engine := newNopEngine(matchRepo, playerRepo, escrowRepo)

	// Must not panic or double-settle.
	engine.sweepDisconnectTimeouts(ctx)
}

func TestEngine_DisconnectTimeout_ReconnectBeforeDeadlinePreventsForfeiture(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := newTestService(t, db, false)
	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")

	// Mark disconnected with a deadline in the past...
	past := time.Now().Add(-1 * time.Second)
	_, _ = db.Exec(`UPDATE p2p_matches SET disconnect_user_id = $1, disconnect_deadline = $2 WHERE id = $3`,
		creator, past, matchID)

	// ...but the player reconnects right before the sweep runs (clears columns).
	if _, err := svc.HandlePlayerReconnect(ctx, creator, matchID); err != nil {
		t.Fatalf("reconnect: %v", err)
	}

	engine := newNopEngine(matchRepo, playerRepo, escrowRepo)
	engine.sweepDisconnectTimeouts(ctx)

	var status string
	if err := db.QueryRow(`SELECT status FROM p2p_matches WHERE id = $1`, matchID).Scan(&status); err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != p2pdomain.StatusInProgress {
		t.Fatalf("expected match still in_progress after reconnect, got %s", status)
	}
}

// ─── Resign idempotency / duplicate ──────────────────────────────────────────

func TestResignMatch_DuplicateRequestAfterCompletion(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	if _, err := svc.ResignMatch(ctx, creator, matchID); err != nil {
		t.Fatalf("first resign: %v", err)
	}

	// A duplicate resign attempt must return a conflict, not a 500 or panic.
	_, err := svc.ResignMatch(ctx, creator, matchID)
	if err == nil {
		t.Fatal("expected error on duplicate resign, got nil")
	}
	if !errors.Is(err, p2pdomain.ErrMatchNotInProgress) {
		t.Fatalf("expected ErrMatchNotInProgress, got %v", err)
	}
}

// ─── Moves after match completion ─────────────────────────────────────────────

func TestSubmitMove_RejectedAfterCompletion(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, joiner, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	// Finish the match via resignation.
	if _, err := svc.ResignMatch(ctx, creator, matchID); err != nil {
		t.Fatalf("resign: %v", err)
	}

	// Both players attempt to submit a move post-completion.
	for _, userID := range []string{creator, joiner} {
		_, err := svc.SubmitMove(ctx, userID, matchID, map[string]interface{}{"cell": 0})
		if !errors.Is(err, p2pdomain.ErrMatchNotInProgress) {
			t.Fatalf("expected ErrMatchNotInProgress for post-completion move by %s, got %v", userID, err)
		}
	}
}

// ─── Unauthorized actions ─────────────────────────────────────────────────────

func TestHandlePlayerDisconnect_UnauthorizedUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	_, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	outsider := createTestUserWithWallet(t, db, 100)

	_, err := svc.HandlePlayerDisconnect(context.Background(), outsider, matchID)
	if !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant, got %v", err)
	}
}

func TestHandlePlayerReconnect_UnauthorizedUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	outsider := createTestUserWithWallet(t, db, 100)
	ctx := context.Background()

	// Disconnect creator first.
	if _, err := svc.HandlePlayerDisconnect(ctx, creator, matchID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	_, err := svc.HandlePlayerReconnect(ctx, outsider, matchID)
	if !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant, got %v", err)
	}
}

// ─── Simultaneous disconnects ─────────────────────────────────────────────────

func TestHandlePlayerDisconnect_BothPlayersDisconnect(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, joiner, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")
	ctx := context.Background()

	// First disconnect sets creator.
	if _, err := svc.HandlePlayerDisconnect(ctx, creator, matchID); err != nil {
		t.Fatalf("creator disconnect: %v", err)
	}

	// Second disconnect by joiner while creator's deadline is still active.
	// The "first deadline wins" rule preserves the existing record — the joiner's
	// call is a no-op and the creator's deadline is not overwritten.
	view, err := svc.HandlePlayerDisconnect(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("joiner disconnect: %v", err)
	}
	// disconnect_user_id must still point at creator (first disconnect wins).
	if view.DisconnectUserID == nil || *view.DisconnectUserID != creator {
		t.Fatalf("expected disconnect_user_id to remain %s (first disconnect wins), got %v", creator, view.DisconnectUserID)
	}
}

// ─── Late reconnect after match completed ─────────────────────────────────────

func TestHandlePlayerReconnect_LateReturnsAlreadyEnded(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := newTestService(t, db, false)
	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")

	// Force expired deadline to simulate the sweep window.
	past := time.Now().Add(-1 * time.Second)
	_, _ = db.Exec(`UPDATE p2p_matches SET disconnect_user_id = $1, disconnect_deadline = $2 WHERE id = $3`,
		creator, past, matchID)

	// Sweep forfeits the match.
	engine := newNopEngine(matchRepo, playerRepo, escrowRepo)
	engine.sweepDisconnectTimeouts(ctx)

	// Now the player tries to reconnect — must get ErrMatchAlreadyEnded.
	_, err := svc.HandlePlayerReconnect(ctx, creator, matchID)
	if !errors.Is(err, p2pdomain.ErrMatchAlreadyEnded) {
		t.Fatalf("expected ErrMatchAlreadyEnded for late reconnect, got %v", err)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// newNopEngine returns a P2PEngine with a no-op logger for use in tests.
func newNopEngine(matchRepo p2pdomain.MatchRepository, playerRepo p2pdomain.PlayerRepository, escrowRepo p2pdomain.EscrowRepository) *P2PEngine {
	e := NewP2PEngine(matchRepo, playerRepo, escrowRepo, nil, zap.NewNop())
	return e
}

// ─── Turn clock vs. reconnection grace period ────────────────────────────────

// A player inside the reconnection grace period must be judged by that deadline,
// not by the turn clock. Otherwise the 60 s grace period is meaningless: the
// 15 s turn timeout forfeits them first, which is exactly the unfair loss the
// reported bug produced.
func TestTurnTimeout_DoesNotForfeitDuringDisconnectGrace(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, false)
	creator, _, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")

	// The creator (player 0, the opening mover) loses their transport.
	if _, err := svc.HandlePlayerDisconnect(ctx, creator, matchID); err != nil {
		t.Fatalf("HandlePlayerDisconnect: %v", err)
	}

	// The turn deadline has already passed, but the grace period is still open.
	past := time.Now().Add(-5 * time.Second)
	if _, err := db.Exec(`UPDATE p2p_matches SET turn_deadline = $1 WHERE id = $2`, past, matchID); err != nil {
		t.Fatalf("force expired turn deadline: %v", err)
	}

	engine := newNopEngine(
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
	)
	engine.sweepTurnTimeouts(ctx)

	var status string
	if err := db.QueryRow(`SELECT status FROM p2p_matches WHERE id = $1`, matchID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != p2pdomain.StatusInProgress {
		t.Fatalf("turn timeout forfeited a player inside the grace period: status=%s", status)
	}

	// Reconnecting restores the state and refreshes the (expired) turn clock.
	view, err := svc.HandlePlayerReconnect(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("HandlePlayerReconnect: %v", err)
	}
	if view.DisconnectUserID != nil {
		t.Fatalf("reconnect did not clear the disconnect record: %v", view.DisconnectUserID)
	}
	if view.TurnSecondsLeft <= 0 {
		t.Fatalf("reconnect did not restore a full turn clock: %ds", view.TurnSecondsLeft)
	}
}

// Once the grace period lapses the disconnect sweep finishes the match, and only
// then. Having both timeouts fire would produce two conflicting results.
func TestTurnTimeout_ResumesAfterGraceExpires(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, false)
	creator, joiner, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")

	if _, err := svc.HandlePlayerDisconnect(ctx, creator, matchID); err != nil {
		t.Fatalf("HandlePlayerDisconnect: %v", err)
	}

	// Grace period over.
	past := time.Now().Add(-time.Second)
	if _, err := db.Exec(`UPDATE p2p_matches SET disconnect_deadline = $1 WHERE id = $2`, past, matchID); err != nil {
		t.Fatalf("expire grace deadline: %v", err)
	}

	engine := newNopEngine(
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
	)
	engine.sweepTurnTimeouts(ctx)
	engine.sweepDisconnectTimeouts(ctx)

	var status string
	var winner sql.NullString
	if err := db.QueryRow(`SELECT status, winner_user_id FROM p2p_matches WHERE id = $1`, matchID).
		Scan(&status, &winner); err != nil {
		t.Fatalf("read result: %v", err)
	}
	if status != p2pdomain.StatusForfeited {
		t.Fatalf("expected forfeited after the grace period lapsed, got %s", status)
	}
	if !winner.Valid || winner.String != joiner {
		t.Fatalf("expected the connected player to win, got %v", winner)
	}
}

// A match whose stake reservations are missing can never be settled: the
// escrow layer refuses to move money without a matching ledger row, so the
// sweep used to fail and retry every second for ever while both players sat on
// a match that never finished. The terminal transition must happen anyway,
// without touching a wallet.
func TestTurnTimeout_IncompleteEscrowStillEndsMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, false)
	_, joiner, matchID := setupInProgressMatch(t, db, svc, "tic-tac-toe")

	// A funded match whose reservations then went missing (partial join,
	// manual data repair, …): settlement would fail for ever.
	if _, err := db.Exec(
		`UPDATE p2p_matches SET stake_cents = 5000, prize_pool_cents = 10000 WHERE id = $1`, matchID,
	); err != nil {
		t.Fatalf("fund the match: %v", err)
	}
	if _, err := db.Exec(
		`DELETE FROM wallet_transactions WHERE reference = $1 AND type = 'p2p_match_stake'`,
		"p2p_stake:"+matchID,
	); err != nil {
		t.Fatalf("drop reservations: %v", err)
	}

	// The turn clock expired and no reconnection grace period is open.
	past := time.Now().Add(-time.Second)
	if _, err := db.Exec(
		`UPDATE p2p_matches SET turn_deadline = $1, disconnect_deadline = NULL WHERE id = $2`,
		past, matchID,
	); err != nil {
		t.Fatalf("force expired turn deadline: %v", err)
	}

	var txBefore int
	if err := db.QueryRow(
		`SELECT count(*) FROM wallet_transactions WHERE reference = $1`, "p2p_stake:"+matchID,
	).Scan(&txBefore); err != nil {
		t.Fatalf("count transactions: %v", err)
	}

	engine := newNopEngine(
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
	)
	engine.sweepTurnTimeouts(ctx)
	// A second sweep must find nothing left to do — this is the loop that used
	// to run once per second, for ever.
	engine.sweepTurnTimeouts(ctx)

	var status string
	var winner sql.NullString
	if err := db.QueryRow(`SELECT status, winner_user_id FROM p2p_matches WHERE id = $1`, matchID).
		Scan(&status, &winner); err != nil {
		t.Fatalf("read result: %v", err)
	}
	if status != p2pdomain.StatusForfeited {
		t.Fatalf("expected the match to reach a terminal state anyway, got %s", status)
	}
	if !winner.Valid || winner.String != joiner {
		t.Fatalf("expected the connected player to win, got %v", winner)
	}

	var txAfter int
	if err := db.QueryRow(
		`SELECT count(*) FROM wallet_transactions WHERE reference = $1`, "p2p_stake:"+matchID,
	).Scan(&txAfter); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if txAfter != txBefore {
		t.Fatalf("settlement must not move money when reservations are missing: %d -> %d", txBefore, txAfter)
	}
}
