package application

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	p2pdomain "github.com/raffle-app/backend/internal/p2p/domain"
)

// clearWaitingRooms isolates matchmaking assertions from rooms left behind by
// earlier tests sharing the development database.
func clearWaitingRooms(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`UPDATE p2p_matches SET status = 'cancelled' WHERE status = 'waiting'`); err != nil {
		t.Fatalf("failed to clear waiting rooms: %v", err)
	}
}

func TestCompetitionModes_FreeQuickPlayNeverJoinsPaidRoom(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	host := createTestUserWithWallet(t, db, 500)
	freePlayer := createTestUserWithWallet(t, db, 0)

	paid, err := svc.CreateMatch(ctx, host, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     50,
	})
	if err != nil {
		t.Fatalf("paid create failed: %v", err)
	}
	if paid.CompetitionMode != p2pdomain.ModeEarn || paid.StakeAmount != 50 {
		t.Fatalf("expected a staked room, got mode=%s stake=%.2f", paid.CompetitionMode, paid.StakeAmount)
	}

	free, err := svc.CreateMatch(ctx, freePlayer, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeFun,
		StakeAmount:     0,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("free quick play failed: %v", err)
	}
	if free.ID == paid.ID {
		t.Fatal("free quick play joined a paid room")
	}
	if free.CompetitionMode != p2pdomain.ModeFun || free.StakeAmount != 0 {
		t.Fatalf("expected a free room, got mode=%s stake=%.2f", free.CompetitionMode, free.StakeAmount)
	}

	// The paid room must still be waiting for a staked opponent.
	room, err := svc.GetMatchState(ctx, host, paid.ID)
	if err != nil {
		t.Fatalf("failed to reload paid room: %v", err)
	}
	if room.Status != p2pdomain.StatusWaiting || len(room.Players) != 1 {
		t.Fatalf("paid room was disturbed: status=%s players=%d", room.Status, len(room.Players))
	}
}

func TestCompetitionModes_MatchmakingRequiresIdenticalStake(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	host := createTestUserWithWallet(t, db, 500)
	differentStake := createTestUserWithWallet(t, db, 500)
	sameStake := createTestUserWithWallet(t, db, 500)

	waiting, err := svc.CreateMatch(ctx, host, CreateMatchRequest{
		GameSlug:        "connect-four",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     50,
	})
	if err != nil {
		t.Fatalf("host create failed: %v", err)
	}

	wrong, err := svc.CreateMatch(ctx, differentStake, CreateMatchRequest{
		GameSlug:        "connect-four",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     100,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("different-stake create failed: %v", err)
	}
	if wrong.ID == waiting.ID || wrong.Status != p2pdomain.StatusWaiting {
		t.Fatalf("different stakes matched: wrong=%s host=%s status=%s", wrong.ID, waiting.ID, wrong.Status)
	}

	joined, err := svc.CreateMatch(ctx, sameStake, CreateMatchRequest{
		GameSlug:        "connect-four",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     50,
		AutoMatch:       true,
	})
	if err != nil {
		t.Fatalf("same-stake match failed: %v", err)
	}
	if joined.ID != waiting.ID || joined.Status != p2pdomain.StatusReady {
		t.Fatalf("identical stakes did not match: got id=%s status=%s", joined.ID, joined.Status)
	}
	if joined.PrizePool != 100 {
		t.Fatalf("expected 100.00 prize pool, got %.2f", joined.PrizePool)
	}
}

func TestCompetitionModes_LobbiesAreSeparated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	paidHost := createTestUserWithWallet(t, db, 500)
	freeHost := createTestUserWithWallet(t, db, 0)

	paid, err := svc.CreateMatch(ctx, paidHost, CreateMatchRequest{
		GameSlug:        "rock-paper-scissors",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     30,
	})
	if err != nil {
		t.Fatalf("paid create failed: %v", err)
	}
	free, err := svc.CreateMatch(ctx, freeHost, CreateMatchRequest{
		GameSlug:        "rock-paper-scissors",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeFun,
	})
	if err != nil {
		t.Fatalf("free create failed: %v", err)
	}

	funRooms, err := svc.ListWaitingMatches(ctx, "", p2pdomain.ModeFun, 50)
	if err != nil {
		t.Fatalf("fun lobby failed: %v", err)
	}
	for _, room := range funRooms {
		if room.CompetitionMode != p2pdomain.ModeFun || room.StakeAmount != 0 {
			t.Fatalf("paid room leaked into fun lobby: %s mode=%s stake=%.2f", room.ID, room.CompetitionMode, room.StakeAmount)
		}
	}
	if !containsMatch(funRooms, free.ID) || containsMatch(funRooms, paid.ID) {
		t.Fatalf("fun lobby contents are wrong")
	}

	earnRooms, err := svc.ListWaitingMatches(ctx, "", p2pdomain.ModeEarn, 50)
	if err != nil {
		t.Fatalf("earn lobby failed: %v", err)
	}
	for _, room := range earnRooms {
		if room.CompetitionMode != p2pdomain.ModeEarn || room.StakeAmount <= 0 {
			t.Fatalf("free room leaked into earn lobby: %s mode=%s stake=%.2f", room.ID, room.CompetitionMode, room.StakeAmount)
		}
	}
	if !containsMatch(earnRooms, paid.ID) || containsMatch(earnRooms, free.ID) {
		t.Fatalf("earn lobby contents are wrong")
	}

	if _, err := svc.ListWaitingMatches(ctx, "", "invalid", 10); !errors.Is(err, p2pdomain.ErrInvalidMode) {
		t.Fatalf("expected ErrInvalidMode, got %v", err)
	}

	if _, err := svc.CreateMatch(ctx, paidHost, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		CompetitionMode: p2pdomain.ModeFun,
		StakeAmount:     10,
	}); !errors.Is(err, p2pdomain.ErrInvalidMode) {
		t.Fatalf("free mode with a stake must be rejected, got %v", err)
	}
	if _, err := svc.CreateMatch(ctx, paidHost, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     0,
	}); !errors.Is(err, p2pdomain.ErrInvalidStake) {
		t.Fatalf("paid mode without a stake must be rejected, got %v", err)
	}
}

func TestCompetitionModes_InsufficientBalancePreventsParticipation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	poor := createTestUserWithWallet(t, db, 5) // 5.00 ETB

	_, err := svc.CreateMatch(ctx, poor, CreateMatchRequest{
		GameSlug:        "tic-tac-toe",
		MatchType:       p2pdomain.TypePublicQueue,
		CompetitionMode: p2pdomain.ModeEarn,
		StakeAmount:     10,
	})
	if !errors.Is(err, p2pdomain.ErrInsufficientStake) {
		t.Fatalf("expected ErrInsufficientStake, got %v", err)
	}

	var reserved float64
	if err := db.QueryRow(`SELECT reserved_balance FROM wallets WHERE user_id = $1`, poor).Scan(&reserved); err != nil {
		t.Fatalf("failed to read wallet: %v", err)
	}
	if reserved != 0 {
		t.Fatalf("rejected stake must not reserve funds, got %.2f", reserved)
	}
}

func TestCompetitionModes_BothPlayersMustConfirmBeforeStart(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	creator, joiner, matchID := setupReadyMatch(t, db, svc, "tic-tac-toe", 50)

	// Both stakes are reserved when each player accepts the challenge...
	var reserved int
	if err := db.QueryRow(`SELECT COUNT(*) FROM wallet_transactions
		WHERE reference = 'p2p_stake:' || $1 AND status = 'reserved'`, matchID).Scan(&reserved); err != nil {
		t.Fatalf("failed to count reservations: %v", err)
	}
	if reserved != 2 {
		t.Fatalf("expected two reserved stakes, got %d", reserved)
	}

	// ...but one confirmation may not start a paid match.
	view, err := svc.ReadyUp(ctx, creator, matchID)
	if err != nil {
		t.Fatalf("creator confirmation failed: %v", err)
	}
	if view.Status != p2pdomain.StatusReady || view.StartedAt != nil {
		t.Fatalf("paid match started on one confirmation: status=%s started=%v", view.Status, view.StartedAt)
	}

	started, err := svc.ReadyUp(ctx, joiner, matchID)
	if err != nil {
		t.Fatalf("joiner confirmation failed: %v", err)
	}
	if started.Status != p2pdomain.StatusInProgress || started.StartedAt == nil {
		t.Fatalf("match did not start after both confirmations: %s", started.Status)
	}
}

func TestCompetitionModes_QuoteUsesConfiguredFee(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, true)
	player := createTestUserWithWallet(t, db, 500)

	quote, err := svc.QuoteStake(ctx, player, "tic-tac-toe", 100)
	if err != nil {
		t.Fatalf("quote failed: %v", err)
	}
	if quote.PrizePool != 200 {
		t.Fatalf("expected 200.00 pool, got %.2f", quote.PrizePool)
	}
	if quote.ServiceFeeBP != 500 || quote.Fee != 10 {
		t.Fatalf("expected 5%% fee (10.00), got %d bp / %.2f", quote.ServiceFeeBP, quote.Fee)
	}
	if quote.PotentialPayout != 190 {
		t.Fatalf("expected 190.00 payout, got %.2f", quote.PotentialPayout)
	}
	if quote.AvailableBalance != 500 {
		t.Fatalf("expected 500.00 available, got %.2f", quote.AvailableBalance)
	}

	// Below the seeded minimum (10.00 ETB) is rejected by the backend.
	if _, err := svc.QuoteStake(ctx, player, "tic-tac-toe", 5); !errors.Is(err, p2pdomain.ErrInvalidStake) {
		t.Fatalf("expected ErrInvalidStake for a stake below the minimum, got %v", err)
	}
	// A stake that is not a permitted increment is rejected too.
	if _, err := svc.QuoteStake(ctx, player, "tic-tac-toe", 15); !errors.Is(err, p2pdomain.ErrInvalidStake) {
		t.Fatalf("expected ErrInvalidStake for a non-increment stake, got %v", err)
	}
}

func TestCompetitionModes_ConcurrentReservationsCannotOverspend(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	player := createTestUserWithWallet(t, db, 100)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateMatch(ctx, player, CreateMatchRequest{
				GameSlug:        "tic-tac-toe",
				MatchType:       p2pdomain.TypePublicQueue,
				CompetitionMode: p2pdomain.ModeEarn,
				StakeAmount:     100,
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes, insufficient := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, p2pdomain.ErrInsufficientStake):
			insufficient++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("expected exactly one reservation, got successes=%d insufficient=%d", successes, insufficient)
	}

	var reserved, balance float64
	if err := db.QueryRow(`SELECT reserved_balance, balance FROM wallets WHERE user_id = $1`, player).Scan(&reserved, &balance); err != nil {
		t.Fatalf("failed to read wallet: %v", err)
	}
	if reserved != 100 || balance != 100 {
		t.Fatalf("wallet overspent: balance=%.2f reserved=%.2f", balance, reserved)
	}
}

func TestCompetitionModes_DuplicateSettlementCannotPayTwice(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	creator, joiner, matchID := setupReadyMatch(t, db, svc, "tic-tac-toe", 50)

	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ready failed: %v", err)
	}
	if _, err := svc.ReadyUp(ctx, joiner, matchID); err != nil {
		t.Fatalf("joiner ready failed: %v", err)
	}

	if _, err := svc.ResignMatch(ctx, creator, matchID); err != nil {
		t.Fatalf("first settlement failed: %v", err)
	}
	var firstBalance float64
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, joiner).Scan(&firstBalance); err != nil {
		t.Fatalf("failed to read winner balance: %v", err)
	}

	if _, err := svc.ResignMatch(ctx, creator, matchID); err == nil {
		t.Fatal("duplicate settlement unexpectedly succeeded")
	}

	var secondBalance float64
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, joiner).Scan(&secondBalance); err != nil {
		t.Fatalf("failed to read winner balance: %v", err)
	}
	if secondBalance != firstBalance {
		t.Fatalf("duplicate settlement changed the balance: first=%.2f second=%.2f", firstBalance, secondBalance)
	}
}

func TestAdminPaidSettings_RequireOperatorGateAndValidate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	svc := newTestService(t, db, false)
	adminID := createTestUserWithWallet(t, db, 0)

	// The deployment gate is closed, so an administrator cannot switch paid
	// play on unilaterally.
	if err := svc.UpdateAdminSettings(ctx, adminID, map[string]string{
		p2pdomain.SettingPaidEnabled: "true",
	}); !errors.Is(err, p2pdomain.ErrPaidNotAuthorized) {
		t.Fatalf("expected ErrPaidNotAuthorized, got %v", err)
	}

	// Opening the operator gate allows the admin toggle.
	svc.SetPaidMatchesPolicy(true, 0.05)
	if err := svc.UpdateAdminSettings(ctx, adminID, map[string]string{
		p2pdomain.SettingPaidEnabled:    "true",
		p2pdomain.SettingMaxStakeCents:  "100000",
		p2pdomain.SettingStakeIncrement: "500",
	}); err != nil {
		t.Fatalf("valid settings update failed: %v", err)
	}

	view, err := svc.GetAdminSettings(ctx)
	if err != nil {
		t.Fatalf("GetAdminSettings failed: %v", err)
	}
	if view.Settings[p2pdomain.SettingPaidEnabled] != "true" {
		t.Fatalf("paid toggle was not persisted: %q", view.Settings[p2pdomain.SettingPaidEnabled])
	}
	if !view.Config.PaidMatchesEnabled {
		t.Fatal("config did not reflect the enabled paid mode")
	}
	if view.Config.MaxStakeCents != 100000 || view.Config.StakeIncrementCents != 500 {
		t.Fatalf("stake settings were not applied: %+v", view.Config)
	}

	if err := svc.UpdateAdminSettings(ctx, adminID, map[string]string{
		p2pdomain.SettingDefaultFeeBP: "20000",
	}); !errors.Is(err, p2pdomain.ErrInvalidSetting) {
		t.Fatalf("expected ErrInvalidSetting for an impossible fee, got %v", err)
	}
	if err := svc.UpdateAdminSettings(ctx, adminID, map[string]string{
		p2pdomain.SettingMinStakeCents: "5000",
		p2pdomain.SettingMaxStakeCents: "1000",
	}); !errors.Is(err, p2pdomain.ErrInvalidSetting) {
		t.Fatalf("expected ErrInvalidSetting for inverted limits, got %v", err)
	}
	if err := svc.UpdateAdminSettings(ctx, adminID, map[string]string{
		"unknown_key": "1",
	}); !errors.Is(err, p2pdomain.ErrInvalidSetting) {
		t.Fatalf("expected ErrInvalidSetting for an unknown key, got %v", err)
	}

	// Restore the activated defaults for the shared test database.
	if err := svc.UpdateAdminSettings(ctx, adminID, map[string]string{
		p2pdomain.SettingMaxStakeCents:  "500000",
		p2pdomain.SettingStakeIncrement: "1000",
		p2pdomain.SettingPaidEnabled:    "true",
	}); err != nil {
		t.Fatalf("failed to restore defaults: %v", err)
	}
}

func TestAdminPaidMatches_ExposeLedgerAndSettlementStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	clearWaitingRooms(t, db)

	ctx := context.Background()
	svc := newTestService(t, db, true)
	creator, joiner, matchID := setupReadyMatch(t, db, svc, "tic-tac-toe", 50)

	if _, err := svc.ReadyUp(ctx, creator, matchID); err != nil {
		t.Fatalf("creator ready failed: %v", err)
	}
	if _, err := svc.ReadyUp(ctx, joiner, matchID); err != nil {
		t.Fatalf("joiner ready failed: %v", err)
	}
	if _, err := svc.ResignMatch(ctx, joiner, matchID); err != nil {
		t.Fatalf("settlement failed: %v", err)
	}

	matches, total, err := svc.ListPaidMatches(ctx, p2pdomain.MatchAdminFilter{})
	if err != nil {
		t.Fatalf("ListPaidMatches failed: %v", err)
	}
	if total < 1 {
		t.Fatal("expected the paid match in the admin list")
	}
	var found *AdminPaidMatch
	for _, candidate := range matches {
		if candidate.Match.ID == matchID {
			found = candidate
			break
		}
	}
	if found == nil {
		t.Fatal("settled paid match missing from the admin list")
	}
	if found.FinancialStatus != "settled" || found.NeedsAttention {
		t.Fatalf("expected a clean settled match, got status=%s attention=%v", found.FinancialStatus, found.NeedsAttention)
	}
	if len(found.Ledger) < 3 {
		t.Fatalf("expected reservation, payout and fee ledger rows, got %d", len(found.Ledger))
	}
}

func containsMatch(matches []*MatchView, id string) bool {
	for _, m := range matches {
		if m.ID == id {
			return true
		}
	}
	return false
}
