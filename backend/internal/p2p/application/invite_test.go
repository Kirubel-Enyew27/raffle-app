package application

import (
	"context"
	"errors"
	"testing"
	"time"

	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
)

func TestP2PService_GetInvite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)
	ctx := context.Background()

	host := createTestUserWithWallet(t, db, 100)

	// Create a private room
	match, err := svc.CreateMatch(ctx, host, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	if match.InviteCode == "" {
		t.Fatalf("expected invite code for private room, got empty")
	}
	if match.ExpiresAt == nil {
		t.Errorf("expected ExpiresAt to be set on waiting MatchView")
	}

	// 1. Fetch valid invite
	summary, err := svc.GetInvite(ctx, match.InviteCode)
	if err != nil {
		t.Fatalf("GetInvite failed: %v", err)
	}
	if summary.MatchID != match.ID {
		t.Errorf("expected match ID %s, got %s", match.ID, summary.MatchID)
	}
	if summary.GameSlug != "tic-tac-toe" {
		t.Errorf("expected game_slug tic-tac-toe, got %s", summary.GameSlug)
	}
	if !summary.IsJoinable {
		t.Errorf("expected is_joinable to be true")
	}
	if summary.IsExpired {
		t.Errorf("expected is_expired to be false")
	}
	if summary.SecondsLeft <= 0 || summary.SecondsLeft > 300 {
		t.Errorf("expected seconds_left between 1 and 300, got %d", summary.SecondsLeft)
	}

	// 2. Case insensitive invite code
	summaryLower, err := svc.GetInvite(ctx, "  "+match.InviteCode+"  ")
	if err != nil {
		t.Fatalf("GetInvite with whitespace failed: %v", err)
	}
	if summaryLower.MatchID != match.ID {
		t.Errorf("expected match ID %s, got %s", match.ID, summaryLower.MatchID)
	}

	// 3. Non-existent invite code
	_, err = svc.GetInvite(ctx, "NONEXS")
	if !errors.Is(err, p2pdomain.ErrMatchNotFound) {
		t.Errorf("expected ErrMatchNotFound, got %v", err)
	}
}

func TestP2PService_CancelMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)
	ctx := context.Background()

	host := createTestUserWithWallet(t, db, 100)
	stranger := createTestUserWithWallet(t, db, 100)

	match, err := svc.CreateMatch(ctx, host, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// 1. Stranger cannot cancel
	_, err = svc.CancelMatch(ctx, stranger, match.ID)
	if !errors.Is(err, p2pdomain.ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant for stranger, got %v", err)
	}

	// 2. Creator can cancel
	cancelled, err := svc.CancelMatch(ctx, host, match.ID)
	if err != nil {
		t.Fatalf("CancelMatch failed: %v", err)
	}
	if cancelled.Status != p2pdomain.StatusCancelled {
		t.Errorf("expected status cancelled, got %s", cancelled.Status)
	}

	// 3. Joining cancelled match fails
	joiner := createTestUserWithWallet(t, db, 100)
	_, err = svc.JoinMatch(ctx, joiner, match.ID, "")
	if !errors.Is(err, p2pdomain.ErrMatchNotJoinable) {
		t.Errorf("expected ErrMatchNotJoinable for cancelled match, got %v", err)
	}

	// 4. Invite preview reports cancelled
	summary, err := svc.GetInvite(ctx, match.InviteCode)
	if err != nil {
		t.Fatalf("GetInvite on cancelled room failed: %v", err)
	}
	if summary.IsJoinable {
		t.Errorf("expected is_joinable false on cancelled room")
	}
	if summary.Status != p2pdomain.StatusCancelled {
		t.Errorf("expected status cancelled, got %s", summary.Status)
	}
}

func TestP2PService_JoinMatch_IdempotencyAndExpiry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)
	ctx := context.Background()

	host := createTestUserWithWallet(t, db, 100)
	joiner := createTestUserWithWallet(t, db, 100)

	match, err := svc.CreateMatch(ctx, host, CreateMatchRequest{
		GameSlug:    "connect-four",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// 1. Host cannot join their own match
	_, err = svc.JoinMatch(ctx, host, match.ID, "")
	if !errors.Is(err, p2pdomain.ErrCannotJoinOwnMatch) {
		t.Errorf("expected ErrCannotJoinOwnMatch, got %v", err)
	}

	// 2. Joiner joins successfully
	view1, err := svc.JoinMatch(ctx, joiner, "", match.InviteCode)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}
	if view1.Status != p2pdomain.StatusReady {
		t.Errorf("expected ready status, got %s", view1.Status)
	}

	// 3. Duplicate / repeated join by joiner returns existing match view idempotently
	view2, err := svc.JoinMatch(ctx, joiner, match.ID, "")
	if err != nil {
		t.Fatalf("Repeated JoinMatch should succeed idempotently, got: %v", err)
	}
	if view2.ID != match.ID {
		t.Errorf("expected match ID %s, got %s", match.ID, view2.ID)
	}

	// 4. Third player cannot join (room full)
	third := createTestUserWithWallet(t, db, 100)
	_, err = svc.JoinMatch(ctx, third, match.ID, "")
	if !errors.Is(err, p2pdomain.ErrMatchNotJoinable) && !errors.Is(err, p2pdomain.ErrMatchFull) {
		t.Errorf("expected match not joinable/full, got %v", err)
	}

	// 5. Expired private room
	expiredMatch, err := svc.CreateMatch(ctx, host, CreateMatchRequest{
		GameSlug:    "connect-four",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}
	// Backdate the created_at by 10 minutes
	_, err = db.Exec(`UPDATE p2p_matches SET created_at = $1 WHERE id = $2`, time.Now().Add(-10*time.Minute), expiredMatch.ID)
	if err != nil {
		t.Fatalf("failed to backdate match: %v", err)
	}

	_, err = svc.JoinMatch(ctx, joiner, expiredMatch.ID, "")
	if !errors.Is(err, p2pdomain.ErrInvitationExpired) {
		t.Errorf("expected ErrInvitationExpired, got %v", err)
	}
}

func TestP2PService_JoinMatch_FormattingAndDisambiguation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := newTestService(t, db, false)
	ctx := context.Background()

	host1 := createTestUserWithWallet(t, db, 100)
	host2 := createTestUserWithWallet(t, db, 100)
	joiner := createTestUserWithWallet(t, db, 100)

	// Create match 1: Tic-Tac-Toe
	m1, err := svc.CreateMatch(ctx, host1, CreateMatchRequest{
		GameSlug:    "tic-tac-toe",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch 1 failed: %v", err)
	}

	// Create match 2: Rock-Paper-Scissors
	m2, err := svc.CreateMatch(ctx, host2, CreateMatchRequest{
		GameSlug:    "rock-paper-scissors",
		MatchType:   p2pdomain.TypePrivateRoom,
		StakeAmount: 0,
	})
	if err != nil {
		t.Fatalf("CreateMatch 2 failed: %v", err)
	}

	// 1. Host 1 cannot join own match via code
	_, err = svc.JoinMatch(ctx, host1, "", m1.InviteCode)
	if !errors.Is(err, p2pdomain.ErrCannotJoinOwnMatch) {
		t.Errorf("expected ErrCannotJoinOwnMatch when host uses code, got %v", err)
	}

	// 2. Nonexistent code returns ErrMatchNotFound
	_, err = svc.JoinMatch(ctx, joiner, "", "ZZZZ99")
	if !errors.Is(err, p2pdomain.ErrMatchNotFound) {
		t.Errorf("expected ErrMatchNotFound for nonexistent code, got %v", err)
	}

	// 3. Joiner joins match 2 using lowercase and whitespace
	codeFormatted := "  " + m2.InviteCode + "  "
	joinedView, err := svc.JoinMatch(ctx, joiner, "", codeFormatted)
	if err != nil {
		t.Fatalf("JoinMatch with formatted code failed: %v", err)
	}
	if joinedView.ID != m2.ID {
		t.Errorf("expected joined match ID %s, got %s", m2.ID, joinedView.ID)
	}
	if joinedView.GameSlug != "rock-paper-scissors" {
		t.Errorf("expected game_slug rock-paper-scissors, got %s", joinedView.GameSlug)
	}
	if joinedView.Status != p2pdomain.StatusReady {
		t.Errorf("expected status ready, got %s", joinedView.Status)
	}

	// Match 1 is untouched and still waiting
	summary1, err := svc.GetInvite(ctx, m1.InviteCode)
	if err != nil {
		t.Fatalf("GetInvite for match 1 failed: %v", err)
	}
	if summary1.Status != p2pdomain.StatusWaiting {
		t.Errorf("expected match 1 status waiting, got %s", summary1.Status)
	}
}
