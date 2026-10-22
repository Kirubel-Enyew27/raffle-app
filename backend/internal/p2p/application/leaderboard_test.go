package application

import (
	"context"
	"database/sql"
	"testing"
	"time"

	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
)

func insertFinalizedMatch(t *testing.T, db *sql.DB, gameSlug string, user1, user2 string, winnerUserID *string, status string, completedAt time.Time) string {
	t.Helper()
	var winnerVal interface{}
	if winnerUserID != nil {
		winnerVal = *winnerUserID
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

	_, err = db.Exec(`INSERT INTO p2p_match_players (id, match_id, user_id, player_index, status, joined_at)
		VALUES (uuid_generate_v4(), $1, $2, 0, 'active', $3)`, matchID, user1, completedAt)
	if err != nil {
		t.Fatalf("failed to insert player 1: %v", err)
	}

	_, err = db.Exec(`INSERT INTO p2p_match_players (id, match_id, user_id, player_index, status, joined_at)
		VALUES (uuid_generate_v4(), $1, $2, 1, 'active', $3)`, matchID, user2, completedAt)
	if err != nil {
		t.Fatalf("failed to insert player 2: %v", err)
	}

	return matchID
}

func TestP2PService_LeaderboardStatisticsAndStreaks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 100.0)
	userB := createTestUserWithWallet(t, db, 100.0)
	userC := createTestUserWithWallet(t, db, 100.0)

	// Set full name for User A to test display_name handling
	_, _ = db.Exec(`UPDATE users SET full_name = 'Alice Champion' WHERE id = $1`, userA)

	gameRepo := p2prepo.NewGameRepo(db)
	matchRepo := p2prepo.NewMatchRepo(db)
	playerRepo := p2prepo.NewPlayerRepo(db)
	moveRepo := p2prepo.NewMoveRepo(db)
	settingsRepo := p2prepo.NewSettingsRepo(db)
	escrowRepo := p2prepo.NewEscrowRepo(db, "")

	svc := NewP2PService(gameRepo, matchRepo, playerRepo, moveRepo, settingsRepo, escrowRepo, nil, nil)

	baseTime := time.Now().UTC().Add(-2 * time.Hour)

	// User A vs User B:
	// M1: User A wins
	insertFinalizedMatch(t, db, "connect-four", userA, userB, &userA, p2pdomain.StatusCompleted, baseTime.Add(10*time.Minute))
	// M2: User A wins
	insertFinalizedMatch(t, db, "connect-four", userA, userB, &userA, p2pdomain.StatusCompleted, baseTime.Add(20*time.Minute))
	// M3: User A wins (via forfeit)
	insertFinalizedMatch(t, db, "connect-four", userA, userB, &userA, p2pdomain.StatusForfeited, baseTime.Add(30*time.Minute))
	// M4: Tied match (Draw - breaks streak)
	insertFinalizedMatch(t, db, "connect-four", userA, userB, nil, p2pdomain.StatusTied, baseTime.Add(40*time.Minute))
	// M5: User A wins
	insertFinalizedMatch(t, db, "connect-four", userA, userB, &userA, p2pdomain.StatusCompleted, baseTime.Add(50*time.Minute))

	// User B vs User C:
	// M6: User B wins
	insertFinalizedMatch(t, db, "connect-four", userB, userC, &userB, p2pdomain.StatusCompleted, baseTime.Add(60*time.Minute))

	// Also insert an unfinished match and a cancelled match (must be excluded)
	_, _ = db.Exec(`INSERT INTO p2p_matches (id, game_slug, status, match_type, created_by, created_at, updated_at)
		VALUES (uuid_generate_v4(), 'connect-four', 'in_progress', 'public_queue', $1, NOW(), NOW())`, userA)
	_, _ = db.Exec(`INSERT INTO p2p_matches (id, game_slug, status, match_type, created_by, created_at, updated_at)
		VALUES (uuid_generate_v4(), 'connect-four', 'cancelled', 'public_queue', $1, NOW(), NOW())`, userA)

	// Fetch all_time leaderboard for connect-four
	cf := "connect-four"
	res, err := svc.GetLeaderboard(ctx, p2pdomain.LeaderboardFilter{
		Period: p2pdomain.PeriodAllTime,
		Game:   &cf,
		SortBy: p2pdomain.SortByWins,
		Page:   1,
		Limit:  100,
		UserID: &userA,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	if len(res.Players) < 3 {
		t.Fatalf("expected at least 3 players, got %d", len(res.Players))
	}

	// Verify User A stats
	var playerA *p2pdomain.LeaderboardPlayer
	var playerB *p2pdomain.LeaderboardPlayer
	var playerC *p2pdomain.LeaderboardPlayer

	for _, p := range res.Players {
		if p.PlayerID == userA {
			playerA = p
		} else if p.PlayerID == userB {
			playerB = p
		} else if p.PlayerID == userC {
			playerC = p
		}
	}

	if playerA == nil || playerB == nil || playerC == nil {
		t.Fatalf("expected all test users in leaderboard")
	}

	// User A assertions:
	if playerA.DisplayName != "Alice Champion" {
		t.Errorf("expected DisplayName 'Alice Champion', got '%s'", playerA.DisplayName)
	}
	if playerA.MatchesPlayed != 5 {
		t.Errorf("User A matches_played expected 5, got %d", playerA.MatchesPlayed)
	}
	if playerA.Wins != 4 {
		t.Errorf("User A wins expected 4, got %d", playerA.Wins)
	}
	if playerA.Losses != 0 {
		t.Errorf("User A losses expected 0, got %d", playerA.Losses)
	}
	if playerA.Draws != 1 {
		t.Errorf("User A draws expected 1, got %d", playerA.Draws)
	}
	if playerA.WinRate != 80.0 {
		t.Errorf("User A win_rate expected 80.0, got %v", playerA.WinRate)
	}
	if playerA.CurrentWinStreak != 1 {
		t.Errorf("User A current_win_streak expected 1, got %d", playerA.CurrentWinStreak)
	}
	if playerA.LongestWinStreak != 3 {
		t.Errorf("User A longest_win_streak expected 3, got %d", playerA.LongestWinStreak)
	}

	// User B assertions:
	// User B played 6 matches (5 against A, 1 against C)
	if playerB.MatchesPlayed != 6 {
		t.Errorf("User B matches_played expected 6, got %d", playerB.MatchesPlayed)
	}
	if playerB.Wins != 1 {
		t.Errorf("User B wins expected 1, got %d", playerB.Wins)
	}
	if playerB.Losses != 4 {
		t.Errorf("User B losses expected 4, got %d", playerB.Losses)
	}
	if playerB.Draws != 1 {
		t.Errorf("User B draws expected 1, got %d", playerB.Draws)
	}
	if playerB.CurrentWinStreak != 1 {
		t.Errorf("User B current_win_streak expected 1, got %d", playerB.CurrentWinStreak)
	}

	// User C assertions:
	if playerC.MatchesPlayed != 1 {
		t.Errorf("User C matches_played expected 1, got %d", playerC.MatchesPlayed)
	}
	if playerC.Wins != 0 {
		t.Errorf("User C wins expected 0, got %d", playerC.Wins)
	}
	if playerC.Losses != 1 {
		t.Errorf("User C losses expected 1, got %d", playerC.Losses)
	}
	if playerC.CurrentWinStreak != 0 || playerC.LongestWinStreak != 0 {
		t.Errorf("User C streaks expected 0/0, got %d/%d", playerC.CurrentWinStreak, playerC.LongestWinStreak)
	}

	// Verify current_player was populated
	if res.CurrentPlayer == nil {
		t.Fatalf("expected CurrentPlayer to be populated")
	}
	if res.CurrentPlayer.PlayerID != userA {
		t.Fatalf("expected CurrentPlayer ID %s, got %s", userA, res.CurrentPlayer.PlayerID)
	}
	if res.CurrentPlayer.Rank != playerA.Rank {
		t.Fatalf("expected CurrentPlayer rank %d, got %d", playerA.Rank, res.CurrentPlayer.Rank)
	}
}

func TestP2PService_LeaderboardGameFilterAndNonExistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 100.0)
	userB := createTestUserWithWallet(t, db, 100.0)

	// Tic-Tac-Toe match
	insertFinalizedMatch(t, db, "tic-tac-toe", userA, userB, &userA, p2pdomain.StatusCompleted, time.Now().UTC())

	svc := NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		nil, nil,
	)

	// Non-existent game slug
	badGame := "non-existent-game-slug-999"
	_, err := svc.GetLeaderboard(ctx, p2pdomain.LeaderboardFilter{
		Period: p2pdomain.PeriodAllTime,
		Game:   &badGame,
		SortBy: p2pdomain.SortByWins,
		Page:   1,
		Limit:  10,
	})
	if err != p2pdomain.ErrGameNotFound {
		t.Fatalf("expected ErrGameNotFound for bad game, got %v", err)
	}
}

func TestP2PService_LeaderboardSortingVariants(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userA := createTestUserWithWallet(t, db, 100.0)
	userB := createTestUserWithWallet(t, db, 100.0)

	now := time.Now().UTC()
	// userA wins 2 matches
	insertFinalizedMatch(t, db, "rock-paper-scissors", userA, userB, &userA, p2pdomain.StatusCompleted, now.Add(1*time.Minute))
	insertFinalizedMatch(t, db, "rock-paper-scissors", userA, userB, &userA, p2pdomain.StatusCompleted, now.Add(2*time.Minute))

	svc := NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		nil, nil,
	)

	rps := "rock-paper-scissors"

	// Sort by win_streak
	resStreak, err := svc.GetLeaderboard(ctx, p2pdomain.LeaderboardFilter{
		Period: p2pdomain.PeriodAllTime,
		Game:   &rps,
		SortBy: p2pdomain.SortByWinStreak,
		Page:   1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard sort_by win_streak failed: %v", err)
	}
	var rankA, rankB int
	for _, p := range resStreak.Players {
		if p.PlayerID == userA {
			rankA = p.Rank
		}
		if p.PlayerID == userB {
			rankB = p.Rank
		}
	}
	if rankA == 0 || rankB == 0 || rankA >= rankB {
		t.Errorf("expected userA rank (%d) < userB rank (%d) in sort_by win_streak", rankA, rankB)
	}

	// Sort by win_rate
	resRate, err := svc.GetLeaderboard(ctx, p2pdomain.LeaderboardFilter{
		Period: p2pdomain.PeriodAllTime,
		Game:   &rps,
		SortBy: p2pdomain.SortByWinRate,
		Page:   1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard sort_by win_rate failed: %v", err)
	}
	rankA = 0
	rankB = 0
	for _, p := range resRate.Players {
		if p.PlayerID == userA {
			rankA = p.Rank
		}
		if p.PlayerID == userB {
			rankB = p.Rank
		}
	}
	if rankA == 0 || rankB == 0 || rankA >= rankB {
		t.Errorf("expected userA rank (%d) < userB rank (%d) in sort_by win_rate", rankA, rankB)
	}
}

func TestP2PService_LeaderboardPeriodFiltering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userOld := createTestUserWithWallet(t, db, 100.0)
	userOpp := createTestUserWithWallet(t, db, 100.0)

	// Finalized match 3 days ago (outside daily, but within weekly/monthly if early in the week)
	threeDaysAgo := time.Now().UTC().Add(-72 * time.Hour)
	insertFinalizedMatch(t, db, "connect-four", userOld, userOpp, &userOld, p2pdomain.StatusCompleted, threeDaysAgo)

	svc := NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		nil, nil,
	)

	cf := "connect-four"

	// Daily query: should NOT contain userOld from 3 days ago
	resDaily, err := svc.GetLeaderboard(ctx, p2pdomain.LeaderboardFilter{
		Period: p2pdomain.PeriodDaily,
		Game:   &cf,
		SortBy: p2pdomain.SortByWins,
		Page:   1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard daily failed: %v", err)
	}
	for _, p := range resDaily.Players {
		if p.PlayerID == userOld {
			t.Errorf("expected userOld to be excluded from daily leaderboard, but found in results")
		}
	}

	// All-time query: MUST contain userOld
	resAll, err := svc.GetLeaderboard(ctx, p2pdomain.LeaderboardFilter{
		Period: p2pdomain.PeriodAllTime,
		Game:   &cf,
		SortBy: p2pdomain.SortByWins,
		Page:   1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard all_time failed: %v", err)
	}
	found := false
	for _, p := range resAll.Players {
		if p.PlayerID == userOld {
			found = true
			if p.Wins < 1 {
				t.Errorf("expected at least 1 win for userOld in all_time, got %d", p.Wins)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected userOld in all_time leaderboard")
	}
}

func TestP2PService_LeaderboardDeterministicTieBreaking(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user1 := createTestUserWithWallet(t, db, 100.0)
	user2 := createTestUserWithWallet(t, db, 100.0)
	dummy := createTestUserWithWallet(t, db, 100.0)

	// Both user1 and user2 win exactly 1 match
	now := time.Now().UTC()
	insertFinalizedMatch(t, db, "tic-tac-toe", user1, dummy, &user1, p2pdomain.StatusCompleted, now)
	insertFinalizedMatch(t, db, "tic-tac-toe", user2, dummy, &user2, p2pdomain.StatusCompleted, now)

	svc := NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		nil, nil,
	)

	ttt := "tic-tac-toe"
	res, err := svc.GetLeaderboard(ctx, p2pdomain.LeaderboardFilter{
		Period: p2pdomain.PeriodAllTime,
		Game:   &ttt,
		SortBy: p2pdomain.SortByWins,
		Page:   1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// Find user1 and user2 positions
	var rank1, rank2 int
	for _, p := range res.Players {
		if p.PlayerID == user1 {
			rank1 = p.Rank
		}
		if p.PlayerID == user2 {
			rank2 = p.Rank
		}
	}

	if rank1 == 0 || rank2 == 0 {
		t.Fatalf("both users should be ranked")
	}
	if rank1 == rank2 {
		t.Errorf("expected deterministic distinct sequential ranks, got both rank %d", rank1)
	}

	// Tie breaker: user_id ASC
	if user1 < user2 {
		if rank1 > rank2 {
			t.Errorf("expected user1 (%s) rank < user2 (%s) rank, got %d vs %d", user1, user2, rank1, rank2)
		}
	} else {
		if rank2 > rank1 {
			t.Errorf("expected user2 (%s) rank < user1 (%s) rank, got %d vs %d", user2, user1, rank2, rank1)
		}
	}
}
