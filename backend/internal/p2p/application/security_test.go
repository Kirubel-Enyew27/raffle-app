package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
)

func newTestService(t *testing.T, db *sql.DB, paid bool) *P2PService {
	t.Helper()
	svc := NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		nil,
		nil,
	)
	svc.SetPaidMatchesEnabled(paid)
	return svc
}

// setupReadyMatch creates a two-player match and leaves it in `ready` state
// (joined, no one has readied up).
func setupReadyMatch(t *testing.T, db *sql.DB, svc *P2PService, gameSlug string, stake float64) (creator, joiner string, matchID string) {
	t.Helper()
	ctx := context.Background()

	creator = createTestUserWithWallet(t, db, 500)
	joiner = createTestUserWithWallet(t, db, 500)

	match, err := svc.CreateMatch(ctx, creator, CreateMatchRequest{
		GameSlug:    gameSlug,
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: stake,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}
	joined, err := svc.JoinMatch(ctx, joiner, match.ID, "")
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}
	if joined.Status != p2pdomain.StatusReady {
		t.Fatalf("expected ready after join, got %s", joined.Status)
	}
	return creator, joiner, match.ID
}

func TestP2PService_ResignRequiresParticipant(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	_, joiner, matchID := setupReadyMatch(t, db, svc, "tic-tac-toe", 0)
	outsider := createTestUserWithWallet(t, db, 500)

	_, err := svc.ResignMatch(context.Background(), outsider, matchID)
	if !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant, got %v", err)
	}

	// The match must be untouched and still have no winner.
	var status string
	var winner sql.NullString
	if err := db.QueryRow(`SELECT status, winner_user_id FROM p2p_matches WHERE id = $1`, matchID).
		Scan(&status, &winner); err != nil {
		t.Fatalf("failed to reload match: %v", err)
	}
	if status != p2pdomain.StatusReady {
		t.Fatalf("outsider resign must not change status, got %s", status)
	}
	if winner.Valid {
		t.Fatalf("outsider resign must not set a winner, got %s", winner.String)
	}

	// A seated player can still resign and the opponent is awarded the match.
	view, err := svc.ResignMatch(context.Background(), joiner, matchID)
	if err != nil {
		t.Fatalf("participant resign failed: %v", err)
	}
	if view.Status != p2pdomain.StatusForfeited {
		t.Fatalf("expected forfeited, got %s", view.Status)
	}
	if view.WinnerUserID == nil || *view.WinnerUserID == joiner {
		t.Fatalf("expected the opponent to be awarded the match")
	}
}

func TestP2PService_ReadyUpRequiresParticipant(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	_, _, matchID := setupReadyMatch(t, db, svc, "connect-four", 0)
	outsider := createTestUserWithWallet(t, db, 500)

	if _, err := svc.ReadyUp(context.Background(), outsider, matchID); !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant, got %v", err)
	}
}

func TestP2PService_MatchStateRequiresParticipant(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupReadyMatch(t, db, svc, "rock-paper-scissors", 0)
	outsider := createTestUserWithWallet(t, db, 500)

	if _, err := svc.GetMatchState(context.Background(), outsider, matchID); !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant, got %v", err)
	}
	if _, err := svc.GetMatchState(context.Background(), "", matchID); !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant for anonymous caller, got %v", err)
	}
	if _, err := svc.GetMatchState(context.Background(), creator, matchID); err != nil {
		t.Fatalf("participant should be allowed to read state: %v", err)
	}
}

func TestP2PService_UnsupportedGameIsRejectedWithoutPanic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)
	user := createTestUserWithWallet(t, db, 500)

	if _, err := db.Exec(`INSERT INTO p2p_games (slug, title, is_active)
		VALUES ('unsupported-test-game', 'Unsupported', TRUE)
		ON CONFLICT (slug) DO NOTHING`); err != nil {
		t.Fatalf("failed to seed game row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM p2p_games WHERE slug = 'unsupported-test-game'`)
	})

	_, err := svc.CreateMatch(context.Background(), user, CreateMatchRequest{
		GameSlug:  "unsupported-test-game",
		MatchType: p2pdomain.TypePublicQueue,
	})
	if !errors.Is(err, p2pdomain.ErrUnsupportedGame) {
		t.Fatalf("expected ErrUnsupportedGame, got %v", err)
	}
}

func TestP2PService_PaidSettlementFinalizesBothStakeRows(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, true)

	creator, joiner, matchID := setupReadyMatch(t, db, svc, "tic-tac-toe", 50)

	ctx := context.Background()
	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ready failed: %v", err)
	}
	started, err := svc.ReadyUp(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("joiner ready failed: %v", err)
	}
	if started.Status != p2pdomain.StatusInProgress {
		t.Fatalf("expected in_progress after both ready, got %s", started.Status)
	}

	// A forfeit settles the prize pool: pool 100.00, fee 5%, net 95.00.
	view, err := svc.ResignMatch(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("resign/settlement failed: %v", err)
	}
	if view.WinnerUserID == nil || *view.WinnerUserID != joiner {
		t.Fatalf("expected joiner to win the forfeit")
	}

	// Both reservation rows must be closed and referenced by the real match id.
	var total, completed, reserved int
	if err := db.QueryRow(`SELECT COUNT(*),
			COUNT(*) FILTER (WHERE status = 'completed'),
			COUNT(*) FILTER (WHERE status = 'reserved')
		FROM wallet_transactions
		WHERE type = 'p2p_match_stake' AND reference = 'p2p_stake:' || $1`, matchID).
		Scan(&total, &completed, &reserved); err != nil {
		t.Fatalf("failed to inspect ledger: %v", err)
	}
	if total != 2 || completed != 2 || reserved != 0 {
		t.Fatalf("expected 2 completed stake rows and none reserved, got total=%d completed=%d reserved=%d", total, completed, reserved)
	}

	// No funds may remain locked and the total money must be conserved.
	var reservedBalance float64
	if err := db.QueryRow(`SELECT COALESCE(SUM(reserved_balance), 0) FROM wallets
		WHERE user_id IN ($1, $2)`, creator, joiner).Scan(&reservedBalance); err != nil {
		t.Fatalf("failed to sum reserved: %v", err)
	}
	if reservedBalance != 0 {
		t.Fatalf("expected no locked stake after settlement, got %.2f", reservedBalance)
	}

	var winnerBalance, loserBalance float64
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, joiner).Scan(&winnerBalance); err != nil {
		t.Fatalf("failed to read winner wallet: %v", err)
	}
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, creator).Scan(&loserBalance); err != nil {
		t.Fatalf("failed to read loser wallet: %v", err)
	}
	if winnerBalance != 545.00 {
		t.Fatalf("expected winner balance 545.00 (500 - 50 + 95), got %.2f", winnerBalance)
	}
	if loserBalance != 450.00 {
		t.Fatalf("expected loser balance 450.00, got %.2f", loserBalance)
	}
}

func TestP2PService_ListUserMatchesReturnsHistory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, _, matchID := setupReadyMatch(t, db, svc, "tic-tac-toe", 0)

	matches, total, err := svc.ListUserMatches(context.Background(), creator, 20, 0)
	if err != nil {
		t.Fatalf("ListUserMatches failed: %v", err)
	}
	if total < 1 || len(matches) == 0 {
		t.Fatalf("expected match history, got total=%d rows=%d", total, len(matches))
	}
	found := false
	for _, m := range matches {
		if m.ID == matchID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created match %s missing from history", matchID)
	}
}

func TestP2PService_RockPaperScissorsRevealsRoundBeforeAdvancing(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator, joiner, matchID := setupReadyMatch(t, db, svc, "rock-paper-scissors", 0)
	ctx := context.Background()
	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ready failed: %v", err)
	}
	if _, err := svc.ReadyUp(ctx, joiner, matchID); err != nil {
		t.Fatalf("joiner ready failed: %v", err)
	}

	if _, err := svc.SubmitMove(ctx, creator, matchID, map[string]interface{}{"choice": "rock"}); err != nil {
		t.Fatalf("creator move failed: %v", err)
	}

	// While only one player has committed, the opponent must not see the pick.
	masked, err := svc.GetMatchState(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("state read failed: %v", err)
	}
	choices, _ := masked.BoardState["choices"].(map[string]interface{})
	if choices["0"] == "rock" {
		t.Fatalf("opponent must not see the committed choice, got %v", choices)
	}

	view, err := svc.SubmitMove(ctx, joiner, matchID, map[string]interface{}{"choice": "scissors"})
	if err != nil {
		t.Fatalf("joiner move failed: %v", err)
	}
	if view.CurrentRound != 2 {
		t.Fatalf("expected round 2, got %d", view.CurrentRound)
	}
	result, ok := view.BoardState["last_round_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected the finished round result to survive the board reset, got %v", view.BoardState)
	}
	if result["p0_choice"] != "rock" || result["p1_choice"] != "scissors" {
		t.Fatalf("unexpected reveal: %v", result)
	}
	if view.Scores["0"] != 1 {
		t.Fatalf("expected player 0 to win the round, scores=%v", view.Scores)
	}
}

func TestP2PService_ResignInWaitingStateIsConflictNotServerError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)

	creator := createTestUserWithWallet(t, db, 500)
	match, err := svc.CreateMatch(context.Background(), creator, CreateMatchRequest{
		GameSlug:  "tic-tac-toe",
		MatchType: p2pdomain.TypePublicQueue,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.ResignMatch(context.Background(), creator, match.ID)
	if !errors.Is(err, p2pdomain.ErrMatchNotInProgress) {
		t.Fatalf("expected ErrMatchNotInProgress for a waiting match, got %v", err)
	}
}

func TestP2PService_UnknownMatchIDIsNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)
	user := createTestUserWithWallet(t, db, 500)

	_, err := svc.GetMatchState(context.Background(), user, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, p2pdomain.ErrMatchNotFound) {
		t.Fatalf("expected ErrMatchNotFound, got %v", err)
	}
}
