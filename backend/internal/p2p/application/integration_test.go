package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
	"go.uber.org/zap"
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
		t.Skipf("skipping integration test, postgres not available: %v", err)
	}
	return db
}

func createTestUserWithWallet(t *testing.T, db *sql.DB, balance float64) string {
	t.Helper()
	email := fmt.Sprintf("test-%s@example.com", generateInviteCode())

	var userID string
	err := db.QueryRow(`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'user') RETURNING id`, email).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	_, err = db.Exec(`INSERT INTO wallets (user_id, balance, reserved_balance, reserved_cents, currency)
		VALUES ($1, $2, 0, 0, 'ETB')`, userID, balance)
	if err != nil {
		t.Fatalf("failed to insert test wallet: %v", err)
	}
	return userID
}

func TestP2PService_TicTacToeFullMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Keep the between-round reveal short so the test does not wait seconds.
	previousReveal := roundRevealDelay
	roundRevealDelay = 20 * time.Millisecond
	t.Cleanup(func() { roundRevealDelay = previousReveal })

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 500.0) // 500 ETB
	userB := createTestUserWithWallet(t, db, 500.0) // 500 ETB

	gameRepo := p2prepo.NewGameRepo(db)
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	moveRepo := p2prepo.NewMoveRepo(db)
	settingsRepo := p2prepo.NewSettingsRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := NewP2PService(gameRepo, matchRepo, playerRepo, moveRepo, settingsRepo, escrowRepo, nil, nil)
	svc.SetPaidMatchesEnabled(true)

	// 1. User A creates a match with 50 ETB stake
	match, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 50.0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}
	if match.Status != p2pdomain.StatusWaiting {
		t.Fatalf("expected status waiting, got %s", match.Status)
	}

	// Verify User A wallet has 50 ETB reserved
	var reservedA float64
	_ = db.QueryRow(`SELECT reserved_balance FROM wallets WHERE user_id = $1`, userA).Scan(&reservedA)
	if reservedA != 50.0 {
		t.Fatalf("expected User A reserved balance 50.0, got %v", reservedA)
	}

	// 2. User B joins the match
	joined, err := svc.JoinMatch(ctx, userB, match.ID, "")
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}
	if joined.Status != p2pdomain.StatusReady {
		t.Fatalf("expected status ready, got %s", joined.Status)
	}
	if joined.PrizePool != 100.0 {
		t.Fatalf("expected prize pool 100.0, got %v", joined.PrizePool)
	}

	// 3. Both ready up
	_, err = svc.ReadyUp(ctx, userA, match.ID)
	if err != nil {
		t.Fatalf("User A ReadyUp failed: %v", err)
	}
	started, err := svc.ReadyUp(ctx, userB, match.ID)
	if err != nil {
		t.Fatalf("User B ReadyUp failed: %v", err)
	}
	if started.Status != p2pdomain.StatusInProgress {
		t.Fatalf("expected status in_progress, got %s", started.Status)
	}

	// 4. Play Game (User A wins round 1: 0, 1, 2)
	// Round 1
	// P0: 0, P1: 3
	// P0: 1, P1: 4
	// P0: 2 (wins round 1)
	_, _ = svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"cell": 0})
	_, _ = svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"cell": 3})
	_, _ = svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"cell": 1})
	_, _ = svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"cell": 4})
	r1End, err := svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"cell": 2})
	if err != nil {
		t.Fatalf("round 1 winning move failed: %v", err)
	}
	if r1End.Scores["0"] != 1 {
		t.Fatalf("expected User A score 1, got %v", r1End.Scores["0"])
	}

	// The round-winning board is held for a short reveal: the winning line is
	// still visible, the turn is cleared (nobody loses clock time), and the
	// staged next board never reaches the client.
	if r1End.CurrentTurnUserID != nil {
		t.Fatalf("expected no current turn during the round reveal, got %v", *r1End.CurrentTurnUserID)
	}
	line, ok := r1End.BoardState["winning_line"].([]int)
	if !ok || len(line) != 3 {
		t.Fatalf("expected the winning line during the reveal, got %v", r1End.BoardState["winning_line"])
	}
	if _, leaked := r1End.BoardState[roundTransitionKey]; leaked {
		t.Fatalf("internal round transition leaked into the client view")
	}

	// The background sweeper promotes the staged board when the reveal ends.
	time.Sleep(roundRevealDelay + 30*time.Millisecond)
	sweeper := NewP2PEngine(matchRepo, playerRepo, escrowRepo, nil, zap.NewNop())
	if err := sweeper.handleTurnTimeout(ctx, match.ID); err != nil {
		t.Fatalf("starting round 2 failed: %v", err)
	}
	round2, err := svc.GetMatchState(ctx, userA, match.ID)
	if err != nil {
		t.Fatalf("GetMatchState after the reveal failed: %v", err)
	}
	if round2.CurrentRound != 2 {
		t.Fatalf("expected round 2 after the reveal, got %d", round2.CurrentRound)
	}
	if round2.CurrentTurnUserID == nil || *round2.CurrentTurnUserID != userB {
		t.Fatalf("expected user B to start round 2, got %v", round2.CurrentTurnUserID)
	}
	if round2.BoardState["winning_line"] != nil {
		t.Fatalf("the previous round's winning line must be cleared for the new round: %v", round2.BoardState["winning_line"])
	}
	if round2Cells, _ := round2.BoardState["cells"].([]interface{}); len(round2Cells) == 9 {
		for idx, cell := range round2Cells {
			if cell != float64(0) {
				t.Fatalf("round 2 should start from an empty board, cell %d = %v", idx, cell)
			}
		}
	} else {
		t.Fatalf("round 2 board is not a 9-cell Tic-Tac-Toe board: %v", round2.BoardState["cells"])
	}

	// Round 2 (User A wins round 2: 0, 1, 2 -> wins match 2-0!)
	// Note: in round 2 next turn starter is userB
	turnUser := *round2.CurrentTurnUserID
	otherUser := userA
	if turnUser == userA {
		otherUser = userB
	}

	// Alternate moves to let User A win again
	if turnUser == userB {
		_, _ = svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"cell": 6})
		_, _ = svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"cell": 0})
		_, _ = svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"cell": 7})
		_, _ = svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"cell": 1})
		_, _ = svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"cell": 3})
		matchFinal, err := svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"cell": 2})
		if err != nil {
			t.Fatalf("round 2 move failed: %v", err)
		}
		if matchFinal.Status != p2pdomain.StatusCompleted {
			t.Fatalf("expected match completed, got %s", matchFinal.Status)
		}
		if matchFinal.WinnerUserID == nil || *matchFinal.WinnerUserID != userA {
			t.Fatalf("expected User A to be match winner, got %v", matchFinal.WinnerUserID)
		}

		// The complete winning line must survive the match being completed:
		// a refetch of the terminal state is what both clients render, and it
		// used to come back from before the winning move (no winning_line, no
		// final piece), so the winning row could never be highlighted.
		for _, userID := range []string{userA, userB} {
			terminal, err := svc.GetMatchState(ctx, userID, match.ID)
			if err != nil {
				t.Fatalf("GetMatchState for %s failed: %v", userID, err)
			}
			line, ok := terminal.BoardState["winning_line"].([]interface{})
			assertWinningLine(t, line, ok, []int{0, 1, 2})
			cells, _ := terminal.BoardState["cells"].([]interface{})
			for _, idx := range []int{0, 1, 2} {
				if idx >= len(cells) || cells[idx] == float64(0) {
					t.Fatalf("winning cell %d is empty in the terminal board: %v", idx, cells)
				}
			}
			if terminal.Scores["0"] != 2 || terminal.Scores["1"] != 0 {
				t.Fatalf("expected persisted series score 2-0, got %v", terminal.Scores)
			}
		}
	} else {
		_ = otherUser
	}

	// 5. Financial Invariant Verification:
	// Total Pool: 100 ETB
	// Service Fee (5% = 500 bp): 5 ETB
	// Winner Net Payout: 95 ETB
	// User A initial balance: 500. Staked 50. Won 95. Net balance should be 500 - 50 + 95 = 545.0 ETB!
	// User B initial balance: 500. Staked 50. Lost. Net balance should be 500 - 50 = 450.0 ETB!
	// Both reserved balances should be exactly 0!
	var balanceA, resA, balanceB, resB float64
	_ = db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, userA).Scan(&balanceA, &resA)
	_ = db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, userB).Scan(&balanceB, &resB)

	if balanceA != 545.0 {
		t.Errorf("User A balance = %v, want 545.0", balanceA)
	}
	if resA != 0.0 {
		t.Errorf("User A reserved = %v, want 0.0", resA)
	}
	if balanceB != 450.0 {
		t.Errorf("User B balance = %v, want 450.0", balanceB)
	}
	if resB != 0.0 {
		t.Errorf("User B reserved = %v, want 0.0", resB)
	}

	// Check platform fee transaction row
	var feeCount int
	var feeAmount float64
	_ = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM wallet_transactions
		WHERE type = 'p2p_platform_fee' AND reference = $1`, "p2p_fee:"+match.ID).Scan(&feeCount, &feeAmount)
	if feeCount != 1 || feeAmount != 5.0 {
		t.Errorf("expected 1 fee transaction of 5.0 ETB, got count=%d, amount=%v", feeCount, feeAmount)
	}
}

func TestP2PService_ConnectFourMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 200.0)
	userB := createTestUserWithWallet(t, db, 200.0)

	gameRepo := p2prepo.NewGameRepo(db)
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	moveRepo := p2prepo.NewMoveRepo(db)
	settingsRepo := p2prepo.NewSettingsRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := NewP2PService(gameRepo, matchRepo, playerRepo, moveRepo, settingsRepo, escrowRepo, nil, nil)
	svc.SetPaidMatchesEnabled(true)

	// Create Connect Four match: 20 ETB stake
	match, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "connect-four",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 20.0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, userB, match.ID, "")
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	_, _ = svc.ReadyUp(ctx, userA, match.ID)
	started, err := svc.ReadyUp(ctx, userB, match.ID)
	if err != nil || started.Status != p2pdomain.StatusInProgress {
		t.Fatalf("ReadyUp failed: %v, status: %s", err, started.Status)
	}

	// User A plays column 0 four times
	// User B plays column 1 three times
	// 1: A(0), B(1)
	// 2: A(0), B(1)
	// 3: A(0), B(1)
	// 4: A(0) -> 4 in a column, User A wins!
	for i := 0; i < 3; i++ {
		_, err = svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"column": 0})
		if err != nil {
			t.Fatalf("step %d userA failed: %v", i, err)
		}
		_, err = svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"column": 1})
		if err != nil {
			t.Fatalf("step %d userB failed: %v", i, err)
		}
	}

	finalMatch, err := svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"column": 0})
	if err != nil {
		t.Fatalf("winning move failed: %v", err)
	}
	if finalMatch.Status != p2pdomain.StatusCompleted {
		t.Fatalf("expected completed status, got %s", finalMatch.Status)
	}
	if finalMatch.WinnerUserID == nil || *finalMatch.WinnerUserID != userA {
		t.Fatalf("expected User A to win, got %v", finalMatch.WinnerUserID)
	}

	// Financials:
	// 20 + 20 = 40 ETB pool.
	// 5% fee = 2 ETB.
	// Winner payout = 38 ETB.
	// User A balance: 200 - 20 + 38 = 218 ETB.
	// User B balance: 200 - 20 = 180 ETB.
	var balanceA, balanceB float64
	_ = db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, userA).Scan(&balanceA)
	_ = db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, userB).Scan(&balanceB)

	if balanceA != 218.0 {
		t.Errorf("expected User A balance 218.0, got %v", balanceA)
	}
	if balanceB != 180.0 {
		t.Errorf("expected User B balance 180.0, got %v", balanceB)
	}

	// The vertical winning four must still be readable once the match is over:
	// the terminal board is what both clients render and what they highlight.
	for _, userID := range []string{userA, userB} {
		terminal, err := svc.GetMatchState(ctx, userID, match.ID)
		if err != nil {
			t.Fatalf("GetMatchState for %s failed: %v", userID, err)
		}
		line, ok := terminal.BoardState["winning_line"].([]interface{})
		assertWinningLine(t, line, ok, []int{14, 21, 28, 35})
		cells, _ := terminal.BoardState["cells"].([]interface{})
		for _, idx := range []int{14, 21, 28, 35} {
			if idx >= len(cells) || cells[idx] != float64(1) {
				t.Fatalf("winning token %d is missing from the terminal board: %v", idx, cells)
			}
		}
	}
}

// assertWinningLine compares a JSON-decoded winning line against the expected
// cells, order-independently (the direction of the line is not part of the
// contract, only the set of cells that must be highlighted).
func assertWinningLine(t *testing.T, line []interface{}, ok bool, want []int) {
	t.Helper()
	if !ok || len(line) != len(want) {
		t.Fatalf("expected winning line %v, got %v", want, line)
	}
	seen := make(map[int]bool, len(line))
	for _, v := range line {
		n, isNumber := v.(float64)
		if !isNumber {
			t.Fatalf("winning line contains a non-number: %v", v)
		}
		seen[int(n)] = true
	}
	for _, idx := range want {
		if !seen[idx] {
			t.Fatalf("winning line %v is missing cell %d", line, idx)
		}
	}
}

func TestP2PService_PaidMatchesComplianceGuard(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 100.0)

	gameRepo := p2prepo.NewGameRepo(db)
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	moveRepo := p2prepo.NewMoveRepo(db)
	settingsRepo := p2prepo.NewSettingsRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	// By default, paidMatchesEnabled is FALSE
	svc := NewP2PService(gameRepo, matchRepo, playerRepo, moveRepo, settingsRepo, escrowRepo, nil, nil)

	// Attempting to create match with stake > 0 must fail with ErrPaidMatchesDisabled
	_, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 25.0,
	})
	if err == nil || !errors.Is(err, p2pdomain.ErrPaidMatchesDisabled) {
		t.Fatalf("expected ErrPaidMatchesDisabled, got: %v", err)
	}

	// Free play match (stake = 0) must succeed!
	freeMatch, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 0.0,
	})
	if err != nil {
		t.Fatalf("free match should succeed, got: %v", err)
	}
	if freeMatch.StakeAmount != 0 || freeMatch.PrizePool != 0 {
		t.Fatalf("expected 0 stake and prize pool, got stake=%v pool=%v", freeMatch.StakeAmount, freeMatch.PrizePool)
	}
}

func TestP2PService_QuickPlayPairsWithWaitingOpponent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 500.0)
	userB := createTestUserWithWallet(t, db, 500.0)

	svc := NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		nil, nil,
	)
	svc.SetPaidMatchesEnabled(true)

	// A stake no other test uses, so matchmaking can only find rooms created here.
	const stake = 41.5
	quickPlay := func(userID string) *MatchView {
		t.Helper()
		view, err := svc.CreateMatch(ctx, userID, CreateMatchRequest{
			GameSlug:    "connect-four",
			MatchType:   p2pdomain.TypePublicQueue,
			StakeAmount: stake,
			AutoMatch:   true,
		})
		if err != nil {
			t.Fatalf("Quick Play for %s failed: %v", userID, err)
		}
		return view
	}

	// 1. User A presses Quick Play with nobody waiting: a room opens.
	room := quickPlay(userA)
	if room.Status != p2pdomain.StatusWaiting {
		t.Fatalf("expected waiting room, got %s", room.Status)
	}
	if len(room.Players) != 1 {
		t.Fatalf("expected 1 player in the new room, got %d", len(room.Players))
	}

	// 2. Pressing Quick Play again re-opens the same room instead of queueing twice.
	again := quickPlay(userA)
	if again.ID != room.ID {
		t.Fatalf("expected the same room %s, got %s", room.ID, again.ID)
	}
	if len(again.Players) != 1 {
		t.Fatalf("expected 1 player after re-entering, got %d", len(again.Players))
	}

	// 3. User B presses Quick Play for the same game: they must be paired with A.
	paired := quickPlay(userB)
	if paired.ID != room.ID {
		t.Fatalf("expected User B to join room %s, got a different room %s", room.ID, paired.ID)
	}
	if paired.Status != p2pdomain.StatusReady {
		t.Fatalf("expected paired room to be ready, got %s", paired.Status)
	}
	if len(paired.Players) != 2 {
		t.Fatalf("expected 2 players after pairing, got %d", len(paired.Players))
	}

	var waiting int
	if err := db.QueryRow(`SELECT COUNT(*) FROM p2p_matches WHERE status = 'waiting' AND game_slug = 'connect-four' AND stake_cents = $1`, 4150).Scan(&waiting); err != nil {
		t.Fatalf("failed to count waiting rooms: %v", err)
	}
	if waiting != 0 {
		t.Fatalf("expected no leftover waiting rooms for this stake, found %d", waiting)
	}

	// 4. Both ready up and the duel starts.
	if _, err := svc.ReadyUp(ctx, userA, room.ID); err != nil {
		t.Fatalf("User A ReadyUp failed: %v", err)
	}
	started, err := svc.ReadyUp(ctx, userB, room.ID)
	if err != nil {
		t.Fatalf("User B ReadyUp failed: %v", err)
	}
	if started.Status != p2pdomain.StatusInProgress {
		t.Fatalf("expected match to start, got %s", started.Status)
	}
}

func TestP2PService_RockPaperScissorsMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 100.0)
	userB := createTestUserWithWallet(t, db, 100.0)

	gameRepo := p2prepo.NewGameRepo(db)
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	moveRepo := p2prepo.NewMoveRepo(db)
	settingsRepo := p2prepo.NewSettingsRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := NewP2PService(gameRepo, matchRepo, playerRepo, moveRepo, settingsRepo, escrowRepo, nil, nil)

	// Free play RPS match
	match, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "rock-paper-scissors",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 0.0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// User B joins
	_, err = svc.JoinMatch(ctx, userB, match.ID, "")
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Ready up
	_, _ = svc.ReadyUp(ctx, userA, match.ID)
	started, err := svc.ReadyUp(ctx, userB, match.ID)
	if err != nil {
		t.Fatalf("ReadyUp failed: %v", err)
	}
	if started.Status != p2pdomain.StatusInProgress {
		t.Fatalf("expected in_progress, got %s", started.Status)
	}

	// Round 1:
	// User A picks "rock"
	_, err = svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"choice": "rock"})
	if err != nil {
		t.Fatalf("move 1 failed: %v", err)
	}
	// Verify choice is masked for User B:
	vB, err := svc.GetMatchState(ctx, userB, match.ID)
	if err != nil {
		t.Fatalf("GetMatchState failed: %v", err)
	}
	choicesB, ok := vB.BoardState["choices"].(map[string]interface{})
	if !ok || choicesB["0"] != "hidden" {
		t.Fatalf("expected User A choice to be hidden for User B, got %v", choicesB)
	}

	// User B picks "scissors" -> Rock beats Scissors, User A wins round 1!
	r1End, err := svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"choice": "scissors"})
	if err != nil {
		t.Fatalf("move 2 failed: %v", err)
	}
	if r1End.Scores["0"] != 1 {
		t.Fatalf("expected User A score 1, got %v", r1End.Scores["0"])
	}

	// Round 2:
	// User B picks "paper", User A picks "scissors" -> Scissors beats Paper, User A wins 2-0!
	turnUser := *r1End.CurrentTurnUserID

	if turnUser == userB {
		_, _ = svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"choice": "paper"})
		finalMatch, err := svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"choice": "scissors"})
		if err != nil {
			t.Fatalf("round 2 move 2 failed: %v", err)
		}
		if finalMatch.Status != p2pdomain.StatusCompleted {
			t.Fatalf("expected match completed, got %s", finalMatch.Status)
		}
		if finalMatch.WinnerUserID == nil || *finalMatch.WinnerUserID != userA {
			t.Fatalf("expected User A to win, got %v", finalMatch.WinnerUserID)
		}
	} else {
		_, _ = svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"choice": "scissors"})
		finalMatch, err := svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"choice": "paper"})
		if err != nil {
			t.Fatalf("round 2 move 2 failed: %v", err)
		}
		if finalMatch.Status != p2pdomain.StatusCompleted {
			t.Fatalf("expected match completed, got %s", finalMatch.Status)
		}
		if finalMatch.WinnerUserID == nil || *finalMatch.WinnerUserID != userA {
			t.Fatalf("expected User A to win, got %v", finalMatch.WinnerUserID)
		}
	}
}
func TestP2PService_RematchRequiresBothPlayersAndStartsFreshMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 200.0)
	userB := createTestUserWithWallet(t, db, 200.0)
	outsider := createTestUserWithWallet(t, db, 200.0)

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

	match, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "connect-four",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}
	if _, err := svc.JoinMatch(ctx, userB, match.ID, ""); err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}
	_, _ = svc.ReadyUp(ctx, userA, match.ID)
	if _, err := svc.ReadyUp(ctx, userB, match.ID); err != nil {
		t.Fatalf("ReadyUp failed: %v", err)
	}

	// A rematch of a live match would abandon a game in progress.
	if _, err := svc.RequestRematch(ctx, userA, match.ID); !errors.Is(err, p2pdomain.ErrMatchNotFinished) {
		t.Fatalf("expected ErrMatchNotFinished while in progress, got %v", err)
	}

	// Finish the game: A wins with four chips in column 0.
	for i := 0; i < 3; i++ {
		if _, err := svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"column": 0}); err != nil {
			t.Fatalf("userA move %d failed: %v", i, err)
		}
		if _, err := svc.SubmitMove(ctx, userB, match.ID, map[string]interface{}{"column": 1}); err != nil {
			t.Fatalf("userB move %d failed: %v", i, err)
		}
	}
	finished, err := svc.SubmitMove(ctx, userA, match.ID, map[string]interface{}{"column": 0})
	if err != nil {
		t.Fatalf("winning move failed: %v", err)
	}
	if finished.Status != p2pdomain.StatusCompleted {
		t.Fatalf("expected completed match, got %s", finished.Status)
	}

	if _, err := svc.RequestRematch(ctx, outsider, match.ID); !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant for an outsider, got %v", err)
	}

	// First request only records the intent.
	first, err := svc.RequestRematch(ctx, userA, match.ID)
	if err != nil {
		t.Fatalf("first rematch request failed: %v", err)
	}
	if first.Status != RematchRequested {
		t.Fatalf("expected status %q, got %q", RematchRequested, first.Status)
	}
	if first.Match.RematchRequestedBy == nil || *first.Match.RematchRequestedBy != userA {
		t.Fatalf("expected rematch requested by user A, got %v", first.Match.RematchRequestedBy)
	}
	if first.Match.RematchMatchID != nil {
		t.Fatalf("no rematch match should exist yet, got %s", *first.Match.RematchMatchID)
	}

	// A retried click is idempotent and must not open a match on its own.
	again, err := svc.RequestRematch(ctx, userA, match.ID)
	if err != nil {
		t.Fatalf("repeated rematch request failed: %v", err)
	}
	if again.Status != RematchRequested {
		t.Fatalf("expected still waiting, got %q", again.Status)
	}

	// The opponent accepting starts the return game.
	started, err := svc.RequestRematch(ctx, userB, match.ID)
	if err != nil {
		t.Fatalf("accepting rematch failed: %v", err)
	}
	if started.Status != RematchStarted {
		t.Fatalf("expected status %q, got %q", RematchStarted, started.Status)
	}
	next := started.Match
	if next == nil || next.ID == match.ID {
		t.Fatalf("expected a new match, got %+v", next)
	}
	if next.Status != p2pdomain.StatusInProgress {
		t.Fatalf("rematch should start immediately, got status %s", next.Status)
	}
	if next.StakeAmount != 0 || next.PrizePool != 0 {
		t.Fatalf("expected a free rematch, got stake=%v pool=%v", next.StakeAmount, next.PrizePool)
	}
	if next.Scores["0"] != 0 || next.Scores["1"] != 0 || next.CurrentRound != 1 {
		t.Fatalf("expected a fresh scoreboard, got scores=%v round=%d", next.Scores, next.CurrentRound)
	}
	if len(next.Players) != 2 {
		t.Fatalf("expected both players seated, got %d", len(next.Players))
	}
	// The previous second mover opens the return game.
	if next.Players[0].UserID != userB || next.Players[1].UserID != userA {
		t.Fatalf("expected seats to alternate, got %s then %s", next.Players[0].UserID, next.Players[1].UserID)
	}
	if next.Players[0].Symbol != "RED" || next.Players[1].Symbol != "YELLOW" {
		t.Fatalf("unexpected symbols: %s / %s", next.Players[0].Symbol, next.Players[1].Symbol)
	}
	if next.CurrentTurnUserID == nil || *next.CurrentTurnUserID != userB {
		t.Fatalf("expected user B to move first, got %v", next.CurrentTurnUserID)
	}
	if next.BoardState["move_count"] != 0 && next.BoardState["move_count"] != float64(0) {
		t.Fatalf("expected an empty board, got move_count=%v", next.BoardState["move_count"])
	}

	// The finished match remembers the rematch, and asking again joins that one
	// instead of opening another.
	reloaded, err := svc.GetMatchState(ctx, userA, match.ID)
	if err != nil {
		t.Fatalf("GetMatchState failed: %v", err)
	}
	if reloaded.RematchMatchID == nil || *reloaded.RematchMatchID != next.ID {
		t.Fatalf("expected rematch_match_id %s, got %v", next.ID, reloaded.RematchMatchID)
	}

	repeat, err := svc.RequestRematch(ctx, userA, match.ID)
	if err != nil {
		t.Fatalf("requesting the existing rematch failed: %v", err)
	}
	if repeat.Status != RematchStarted || repeat.Match.ID != next.ID {
		t.Fatalf("expected the existing rematch %s, got %+v", next.ID, repeat)
	}
}
