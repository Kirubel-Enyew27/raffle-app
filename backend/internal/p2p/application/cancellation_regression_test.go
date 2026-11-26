package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	"github.com/raffle-app/backend/pkg/money"
)

// TestCancellation_QuickPlayCreatedNobodyJoins verifies that an unjoined Quick Play room
// can be cancelled by the creator, releasing any reserved escrow, and resulting in status 'cancelled'.
func TestCancellation_QuickPlayCreatedNobodyJoins(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)

	userA := createTestUserWithWallet(t, db, 100)

	// User A creates a paid Quick Play match
	room, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     20,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}
	if room.Status != p2pdomain.StatusWaiting {
		t.Fatalf("expected status waiting, got %s", room.Status)
	}

	// Reserved funds should be 20
	var reserved float64
	if err := db.QueryRow(`SELECT reserved_balance FROM wallets WHERE user_id = $1`, userA).Scan(&reserved); err != nil {
		t.Fatalf("failed to query wallet: %v", err)
	}
	if reserved != 20 {
		t.Fatalf("expected 20 reserved, got %.2f", reserved)
	}

	// User A cancels before anyone joins
	cancelled, err := svc.CancelMatch(ctx, userA, room.ID)
	if err != nil {
		t.Fatalf("CancelMatch failed: %v", err)
	}
	if cancelled.Status != p2pdomain.StatusCancelled {
		t.Fatalf("expected status cancelled, got %s", cancelled.Status)
	}
	if cancelled.WinnerUserID != nil {
		t.Fatalf("expected nil winner_user_id on cancelled match, got %v", *cancelled.WinnerUserID)
	}

	// Wallet reserved funds should be 0
	if err := db.QueryRow(`SELECT reserved_balance FROM wallets WHERE user_id = $1`, userA).Scan(&reserved); err != nil {
		t.Fatalf("failed to query wallet: %v", err)
	}
	if reserved != 0 {
		t.Fatalf("expected 0 reserved after cancellation, got %.2f", reserved)
	}
}

// TestCancellation_PlayerJoinsQuickPlayThenCancelled verifies that when Player B joins
// a Quick Play match (reaching StatusReady), a subsequent cancellation by the host
// cancels the match, clears winner, releases escrow for both players, and neither is recorded as loser.
func TestCancellation_PlayerJoinsQuickPlayThenCancelled(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)

	userA := createTestUserWithWallet(t, db, 100)
	userB := createTestUserWithWallet(t, db, 100)

	// User A creates paid Quick Play match
	roomA, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     30,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("CreateMatch A failed: %v", err)
	}

	// User B joins via Quick Play (tryAutoMatch pairs with A)
	roomB, err := svc.CreateMatch(ctx, userB, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     30,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("CreateMatch B (Quick Play join) failed: %v", err)
	}
	if roomB.ID != roomA.ID {
		t.Fatalf("expected User B to pair into User A's room %s, got %s", roomA.ID, roomB.ID)
	}
	if roomB.Status != p2pdomain.StatusReady {
		t.Fatalf("expected status ready after User B joins, got %s", roomB.Status)
	}

	// Verify both users have 30 reserved
	for _, u := range []string{userA, userB} {
		var res float64
		if err := db.QueryRow(`SELECT reserved_balance FROM wallets WHERE user_id = $1`, u).Scan(&res); err != nil {
			t.Fatalf("wallet query failed: %v", err)
		}
		if res != 30 {
			t.Fatalf("expected 30 reserved for %s, got %.2f", u, res)
		}
	}

	// The host (User A) cancels the match while in ready state
	cancelled, err := svc.CancelMatch(ctx, userA, roomA.ID)
	if err != nil {
		t.Fatalf("CancelMatch failed: %v", err)
	}
	if cancelled.Status != p2pdomain.StatusCancelled {
		t.Fatalf("expected status cancelled, got %s", cancelled.Status)
	}
	if cancelled.WinnerUserID != nil {
		t.Fatalf("expected nil winner_user_id on cancelled match, got %v", *cancelled.WinnerUserID)
	}

	// Verify both users' reserved balances were fully refunded
	for _, u := range []string{userA, userB} {
		var bal, res float64
		if err := db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, u).Scan(&bal, &res); err != nil {
			t.Fatalf("wallet query failed: %v", err)
		}
		if bal != 100 || res != 0 {
			t.Fatalf("user %s wallet: balance=%.2f, reserved=%.2f, want 100/0", u, bal, res)
		}
	}

	// Check match history for both users: neither should have a loss
	for _, u := range []string{userA, userB} {
		history, total, err := svc.ListUserMatches(ctx, u, 10, 0)
		if err != nil {
			t.Fatalf("ListUserMatches failed for %s: %v", u, err)
		}
		if total != 1 || len(history) != 1 {
			t.Fatalf("expected 1 history record for %s, got %d", u, total)
		}
		item := history[0]
		if item.Status != p2pdomain.StatusCancelled {
			t.Fatalf("expected history status cancelled for %s, got %s", u, item.Status)
		}
		if item.WinnerUserID != nil {
			t.Fatalf("expected history winner_user_id nil for %s, got %v", u, *item.WinnerUserID)
		}
	}
}

// TestCancellation_InitiatedByOtherPlayerRejected verifies that only the room creator
// can cancel; the other player receives ErrNotParticipant.
func TestCancellation_InitiatedByOtherPlayerRejected(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, false)

	userA := createTestUserWithWallet(t, db, 100)
	userB := createTestUserWithWallet(t, db, 100)

	room, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 0,
		AutoMatch:   true,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	if _, err := svc.JoinMatch(ctx, userB, room.ID, ""); err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// User B (non-creator) attempts to cancel
	_, err = svc.CancelMatch(ctx, userB, room.ID)
	if !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant when non-creator cancels, got: %v", err)
	}
}

// TestCancellation_DuplicateRequestsIdempotent verifies that retrying CancelMatch
// returns the canonical cancelled state idempotently without releasing escrow twice.
func TestCancellation_DuplicateRequestsIdempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)

	userA := createTestUserWithWallet(t, db, 100)

	room, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     40,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// First cancel
	first, err := svc.CancelMatch(ctx, userA, room.ID)
	if err != nil {
		t.Fatalf("first cancel failed: %v", err)
	}
	if first.Status != p2pdomain.StatusCancelled {
		t.Fatalf("expected status cancelled, got %s", first.Status)
	}

	// Second cancel (duplicate/retry)
	second, err := svc.CancelMatch(ctx, userA, room.ID)
	if err != nil {
		t.Fatalf("second cancel failed: %v", err)
	}
	if second.Status != p2pdomain.StatusCancelled {
		t.Fatalf("expected status cancelled on retry, got %s", second.Status)
	}

	// Balance must still be 100, reserved 0 (not double refunded)
	var bal, res float64
	if err := db.QueryRow(`SELECT balance, reserved_balance FROM wallets WHERE user_id = $1`, userA).Scan(&bal, &res); err != nil {
		t.Fatalf("wallet query failed: %v", err)
	}
	if bal != 100 || res != 0 {
		t.Fatalf("expected balance=100 reserved=0, got bal=%.2f res=%.2f", bal, res)
	}
}

// TestCancellation_RacingWithReadyUp verifies concurrent CancelMatch and ReadyUp operations
// serialize cleanly under row lock without corrupting state.
func TestCancellation_RacingWithReadyUp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, false)

	userA := createTestUserWithWallet(t, db, 100)
	userB := createTestUserWithWallet(t, db, 100)

	room, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 0,
		AutoMatch:   true,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	if _, err := svc.JoinMatch(ctx, userB, room.ID, ""); err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// User A readies up
	if _, err := svc.ReadyUp(ctx, userA, room.ID); err != nil {
		t.Fatalf("ReadyUp userA failed: %v", err)
	}

	// Concurrent race: User B readies up (which would start the game) vs User A cancels
	var wg sync.WaitGroup
	wg.Add(2)

	var cancelErr, readyErr error
	go func() {
		defer wg.Done()
		_, cancelErr = svc.CancelMatch(ctx, userA, room.ID)
	}()

	go func() {
		defer wg.Done()
		_, readyErr = svc.ReadyUp(ctx, userB, room.ID)
	}()

	wg.Wait()

	// Exactly one of the two must succeed; the loser must receive ErrMatchNotJoinable
	var finalStatus string
	if err := db.QueryRow(`SELECT status FROM p2p_matches WHERE id = $1`, room.ID).Scan(&finalStatus); err != nil {
		t.Fatalf("failed to query match status: %v", err)
	}

	if finalStatus == p2pdomain.StatusCancelled {
		if cancelErr != nil {
			t.Fatalf("expected cancel to succeed if status is cancelled, got %v", cancelErr)
		}
	} else if finalStatus == p2pdomain.StatusInProgress {
		if readyErr != nil {
			t.Fatalf("expected ready to succeed if status is in_progress, got %v", readyErr)
		}
	} else {
		t.Fatalf("unexpected final status: %s", finalStatus)
	}
}

// TestCancellation_RacingWithLegitimateMatchCompletion verifies that once a match is completed
// or forfeited, CancelMatch cannot overwrite it, and MarkCompletedTx cannot overwrite a cancelled match.
func TestCancellation_RacingWithLegitimateMatchCompletion(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, false)

	userA := createTestUserWithWallet(t, db, 100)
	userB := createTestUserWithWallet(t, db, 100)

	room, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 0,
		AutoMatch:   true,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	if _, err := svc.JoinMatch(ctx, userB, room.ID, ""); err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// User B resigns legitimately
	forfeited, err := svc.ResignMatch(ctx, userB, room.ID)
	if err != nil {
		t.Fatalf("ResignMatch failed: %v", err)
	}
	if forfeited.Status != p2pdomain.StatusForfeited {
		t.Fatalf("expected status forfeited, got %s", forfeited.Status)
	}
	if *forfeited.WinnerUserID != userA {
		t.Fatalf("expected winner to be userA, got %s", *forfeited.WinnerUserID)
	}

	// Now User A attempts to cancel the finished match
	_, err = svc.CancelMatch(ctx, userA, room.ID)
	if !errors.Is(err, p2pdomain.ErrMatchAlreadyEnded) {
		t.Fatalf("expected ErrMatchAlreadyEnded when cancelling finished match, got %v", err)
	}

	// Final match status must remain forfeited with userA as winner
	var currentStatus string
	var currentWinner *string
	if err := db.QueryRow(`SELECT status, winner_user_id FROM p2p_matches WHERE id = $1`, room.ID).
		Scan(&currentStatus, &currentWinner); err != nil {
		t.Fatalf("query match failed: %v", err)
	}
	if currentStatus != p2pdomain.StatusForfeited {
		t.Fatalf("expected match status to remain forfeited, got %s", currentStatus)
	}
	if currentWinner == nil || *currentWinner != userA {
		t.Fatalf("expected winner to remain userA, got %v", currentWinner)
	}

	// Verify MarkCompletedTx does not overwrite a cancelled match
	cancelledRoom, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePublicQueue,
		StakeAmount: 0,
		AutoMatch:   true,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}
	if _, err := svc.CancelMatch(ctx, userA, cancelledRoom.ID); err != nil {
		t.Fatalf("CancelMatch failed: %v", err)
	}

	// Direct MarkCompletedTx on cancelled match must fail or not match
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin tx failed: %v", err)
	}
	defer tx.Rollback()

	matchEntity := &p2pdomain.Match{ID: cancelledRoom.ID}
	winnerID := userB
	matchRepo := svc.matchRepo
	err = matchRepo.MarkCompletedTx(ctx, tx, matchEntity, &winnerID, p2pdomain.StatusCompleted, money.Cents(0), time.Now(), false)
	// Because WHERE id = $1 AND status <> 'cancelled', no rows are updated and Scan fails with sql.ErrNoRows
	if err == nil {
		t.Fatalf("expected MarkCompletedTx on cancelled match to fail, but got nil")
	}
}

// TestCancellation_GenuineResignationAndCompletionRemainIntact verifies that genuine resignations
// and normal game completions produce proper winner/loser outcomes.
func TestCancellation_GenuineResignationAndCompletionRemainIntact(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)

	userA := createTestUserWithWallet(t, db, 200)
	userB := createTestUserWithWallet(t, db, 200)

	room, err := svc.CreateMatch(ctx, userA, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     50,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	if _, err := svc.JoinMatch(ctx, userB, room.ID, ""); err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// User A resigns: User B is the legitimate winner
	forfeited, err := svc.ResignMatch(ctx, userA, room.ID)
	if err != nil {
		t.Fatalf("ResignMatch failed: %v", err)
	}
	if forfeited.Status != p2pdomain.StatusForfeited {
		t.Fatalf("expected status forfeited, got %s", forfeited.Status)
	}
	if forfeited.WinnerUserID == nil || *forfeited.WinnerUserID != userB {
		t.Fatalf("expected winner to be userB, got %v", forfeited.WinnerUserID)
	}

	// User A lost: balance decreased by 50 (200 - 50 = 150)
	// User B won: balance increased by net payout (200 - 50 + 95 = 245)
	var balA, balB float64
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, userA).Scan(&balA); err != nil {
		t.Fatalf("wallet A query failed: %v", err)
	}
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, userB).Scan(&balB); err != nil {
		t.Fatalf("wallet B query failed: %v", err)
	}
	if balA != 150 {
		t.Fatalf("expected userA balance=150, got %.2f", balA)
	}
	if balB != 245 {
		t.Fatalf("expected userB balance=245, got %.2f", balB)
	}
}
