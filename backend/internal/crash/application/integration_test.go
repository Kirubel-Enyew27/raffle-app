package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/crash/domain"
	crashinfra "github.com/raffle-app/backend/internal/crash/infrastructure"
	crashrepo "github.com/raffle-app/backend/internal/crash/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	appcrypto "github.com/raffle-app/backend/pkg/crypto"
	"github.com/raffle-app/backend/pkg/database"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/money"
	"go.uber.org/zap"
)

// These tests run against the real PostgreSQL instance because the guarantees
// they check (row locks, unique constraints, conditional updates, atomic
// settlement) live in the database and cannot be faked meaningfully. When no
// database is reachable the whole file skips, so `go test ./...` still works in
// environments without one.

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type testEnv struct {
	db     *sql.DB
	room   string
	cfg    domain.RoundConfig
	rounds *crashrepo.RoundRepo
	bets   *crashrepo.BetRepo
	svc    *CrashService
	engine *Engine
	source *crashinfra.CrashPointSource
	clock  *testClock
}

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("no configuration available: %v", err)
	}
	cfg.Database.Host = envOr("CRASH_TEST_DB_HOST", cfg.Database.Host)
	cfg.Database.Port = envOr("CRASH_TEST_DB_PORT", cfg.Database.Port)
	cfg.Database.User = envOr("CRASH_TEST_DB_USER", cfg.Database.User)
	cfg.Database.Password = envOr("CRASH_TEST_DB_PASSWORD", cfg.Database.Password)
	cfg.Database.DBName = envOr("CRASH_TEST_DB_NAME", cfg.Database.DBName)

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		t.Skipf("database not reachable, skipping integration test: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("database not reachable, skipping integration test: %v", err)
	}
	return db
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db := testDatabase(t)

	room := fmt.Sprintf("test-%d", time.Now().UnixNano())

	cfg := domain.DefaultConfig()
	cfg.Room = room
	cfg.WaitingSeconds = 1
	cfg.BettingSeconds = 60
	cfg.CrashedSeconds = 0
	cfg.NextRoundSeconds = 0

	source := crashinfra.NewCrashPointSource(cfg.HouseEdgeBP, cfg.MaxMultiplierBP)
	rounds := crashrepo.NewRoundRepo(db)
	bets := crashrepo.NewBetRepo(db)

	svc := NewCrashService(rounds, bets, crashrepo.NewWalletRepo(db), source, cfg)
	svc.SetPlayerRepository(crashrepo.NewPlayerRepo(db))

	clock := &testClock{now: time.Now().UTC()}
	svc.SetClock(clock)

	logger := zap.NewNop()
	if os.Getenv("CRASH_TEST_VERBOSE") != "" {
		logger, _ = zap.NewDevelopment()
	}
	engine := NewEngine(rounds, bets, source, svc, nil, cfg, logger)
	engine.SetClock(clock)

	env := &testEnv{
		db: db, room: room, cfg: cfg,
		rounds: rounds, bets: bets, svc: svc, engine: engine,
		source: source, clock: clock,
	}

	// The room is unique per test, so the live server's engine (which drives the
	// "default" room) never interferes and vice versa. Users cascade to wallets,
	// ledger rows and bets, so deleting them cleans up the test's money trail.
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM game_rounds WHERE room = $1`, room)
		_ = db.Close()
	})

	return env
}

// --- fixtures -------------------------------------------------------------

func (e *testEnv) newUser(t *testing.T, balance float64) string {
	t.Helper()
	email := fmt.Sprintf("crash-test-%d-%s@example.com", time.Now().UnixNano(), randomID()[:8])
	var userID string
	err := e.db.QueryRow(`INSERT INTO users (email, password_hash, role, full_name)
		VALUES ($1, 'x', 'user', 'Test Player') RETURNING id`, email).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := e.db.Exec(`INSERT INTO wallets (user_id, balance, currency) VALUES ($1, $2, 'ETB')`, userID, balance); err != nil {
		t.Fatalf("insert wallet: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.Exec(`DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

// newUserWithoutWallet creates a player who has never had a wallet row.
func (e *testEnv) newUserWithoutWallet(t *testing.T) string {
	t.Helper()
	var userID string
	err := e.db.QueryRow(`INSERT INTO users (email, password_hash, role, full_name)
		VALUES ($1, 'x', 'user', 'No Wallet') RETURNING id`,
		fmt.Sprintf("crash-nowallet-%d-%s@example.com", time.Now().UnixNano(), randomID()[:8])).Scan(&userID)
	if err != nil {
		t.Fatalf("insert wallet-less user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.Exec(`DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

// renameUser gives a player a distinctive display name so name resolution can be
// asserted per player.
func (e *testEnv) renameUser(t *testing.T, userID, fullName string) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE users SET full_name = $1 WHERE id = $2`, fullName, userID); err != nil {
		t.Fatalf("rename user: %v", err)
	}
}

func (e *testEnv) balance(t *testing.T, userID string) float64 {
	t.Helper()
	var b float64
	if err := e.db.QueryRow(`SELECT balance FROM wallets WHERE user_id = $1`, userID).Scan(&b); err != nil {
		t.Fatalf("read balance: %v", err)
	}
	return b
}

func (e *testEnv) countLedger(t *testing.T, userID, txType string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM wallet_transactions WHERE user_id = $1 AND type = $2`,
		userID, txType).Scan(&n); err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	return n
}

// seedWithCrashAtLeast finds a seed whose crash point reaches at least targetBP
// so a test can cash out comfortably before the crash.
func (e *testEnv) seedWithCrashAtLeast(t *testing.T, targetBP int) (seed string, crashBP int) {
	t.Helper()
	for i := 0; i < 500000; i++ {
		candidate, err := crashinfra.RandomSeed()
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		if bp := e.source.CrashPoint(candidate, 1); bp >= targetBP {
			return candidate, bp
		}
	}
	t.Fatalf("could not find a seed crashing at or above %d", targetBP)
	return "", 0
}

func (e *testEnv) seedCrashingImmediately(t *testing.T) string {
	t.Helper()
	for i := 0; i < 500000; i++ {
		candidate, err := crashinfra.RandomSeed()
		if err != nil {
			t.Fatal(err)
		}
		if e.source.CrashPoint(candidate, 1) == 100 {
			return candidate
		}
	}
	t.Skip("could not find an immediate-crash seed")
	return ""
}

// insertRound writes a round with exact timestamps so tests control the clock
// instead of waiting on real durations.
// Sets p2p_mode=FALSE so legacy tests use the direct-payout path.
func (e *testEnv) insertRound(t *testing.T, state string, bettingStart, bettingEnd time.Time, startedAt *time.Time, seed string, crashBP int) string {
	t.Helper()
	var id string
	err := e.db.QueryRow(`INSERT INTO game_rounds (
			room, round_number, state, server_seed, server_seed_hash, nonce, commitment_published_at,
			betting_starts_at, betting_ends_at, started_at, crash_multiplier_bp,
			p2p_mode, service_fee_bp, min_participants
		) VALUES ($1, nextval('crash_round_number_seq'), $2, $3, $4, 1, $5, $6, $7, $8, $9, FALSE, 500, 2) RETURNING id`,
		e.room, state, seed, appcrypto.SHA256(seed), bettingStart, bettingStart, bettingEnd, startedAt, nullableBP(crashBP),
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert round: %v", err)
	}
	return id
}

// insertP2PRound writes a P2P round for tests that exercise the P2P settlement path.
func (e *testEnv) insertP2PRound(t *testing.T, state string, bettingStart, bettingEnd time.Time, startedAt *time.Time, seed string, crashBP int, serviceFeeBP int) string {
	t.Helper()
	var id string
	err := e.db.QueryRow(`INSERT INTO game_rounds (
			room, round_number, state, server_seed, server_seed_hash, nonce, commitment_published_at,
			betting_starts_at, betting_ends_at, started_at, crash_multiplier_bp,
			p2p_mode, service_fee_bp, min_participants
		) VALUES ($1, nextval('crash_round_number_seq'), $2, $3, $4, 1, $5, $6, $7, $8, $9, TRUE, $10, 2) RETURNING id`,
		e.room, state, seed, appcrypto.SHA256(seed), bettingStart, bettingStart, bettingEnd, startedAt, nullableBP(crashBP), serviceFeeBP,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert P2P round: %v", err)
	}
	return id
}

func nullableBP(bp int) interface{} {
	if bp == 0 {
		return nil
	}
	return bp
}

func (e *testEnv) roundState(t *testing.T, roundID string) string {
	t.Helper()
	var state string
	if err := e.db.QueryRow(`SELECT state FROM game_rounds WHERE id = $1`, roundID).Scan(&state); err != nil {
		t.Fatalf("read round state: %v", err)
	}
	return state
}

func (e *testEnv) stepEngine(t *testing.T) {
	t.Helper()
	e.engine.step(context.Background(), time.Time{})
}

// insertBetDirect records a fully-formed bet (wallet debit, ledger row and bet
// row) for tests that need a round which is already running.
func (e *testEnv) insertBetDirect(t *testing.T, userID string, stakeCents int64) *domain.Bet {
	t.Helper()
	return e.insertBetDirectWithAuto(t, userID, stakeCents, 0)
}

func (e *testEnv) insertBetDirectWithAuto(t *testing.T, userID string, stakeCents int64, autoBP int) *domain.Bet {
	t.Helper()

	round, err := e.rounds.FindCurrent(context.Background(), e.room)
	if err != nil {
		t.Fatalf("find round for direct bet: %v", err)
	}

	tx, err := e.bets.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	var walletID string
	var balance float64
	if err := tx.QueryRow(`SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&walletID, &balance); err != nil {
		t.Fatalf("find wallet: %v", err)
	}

	stake := money.Cents(stakeCents)
	if money.FromFloat(balance) < stake {
		t.Fatalf("test wallet %v cannot cover stake %v", balance, stake.Float())
	}
	newBalance := money.FromFloat(balance) - stake
	ledgerID := randomID()

	if _, err := tx.Exec(`INSERT INTO wallet_transactions
			(id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at)
			VALUES ($1,$2,$3,$4,'completed',$5,$6,$7,$8,'test stake','null',NOW(),NOW())`,
		ledgerID, walletID, userID, TxTypeBet, stake.Float(), balance, newBalance.Float(), "crash_bet:"+ledgerID); err != nil {
		t.Fatalf("insert ledger: %v", err)
	}

	bet := &domain.Bet{
		ID:            randomID(),
		RoundID:       round.ID,
		UserID:        userID,
		WalletID:      walletID,
		StakeCents:    stake,
		StakeAmount:   stake.Float(),
		Status:        domain.BetStatusActive,
		AutoCashoutBP: autoBP,
		DebitTxID:     ledgerID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := e.bets.CreateTx(context.Background(), tx, bet); err != nil {
		t.Fatalf("insert bet: %v", err)
	}
	if _, err := tx.Exec(`UPDATE wallets SET balance = $1, updated_at = NOW() WHERE id = $2`, newBalance.Float(), walletID); err != nil {
		t.Fatalf("update wallet: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit bet: %v", err)
	}
	return bet
}

func randomID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

func durationForMultiplier(bp int, growth float64) time.Duration {
	return time.Duration(domain.DurationForMultiplier(bp, growth) * float64(time.Second))
}

// fixedAt is a clock pinned to one instant.
type fixedAt struct{ t time.Time }

func (f *fixedAt) Now() time.Time { return f.t }

// ---------------------------------------------------------------------------
// 1. A player successfully places a valid bet
// ---------------------------------------------------------------------------

func TestIntegration_PlaceBetDebitsWallet(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	start := env.clock.Now().Add(-time.Second)
	env.insertRound(t, domain.StateBetting, start, env.clock.Now().Add(30*time.Second), nil, seed, 0)

	before := env.balance(t, user)
	bet, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, "key-1")
	if err != nil {
		t.Fatalf("place bet: %v", err)
	}
	if bet.Status != domain.BetStatusActive {
		t.Errorf("new bet status = %s, want active", bet.Status)
	}
	if bet.Stake != 100 {
		t.Errorf("stake = %v, want 100", bet.Stake)
	}
	if bet.ID == "" || len(bet.ID) != 36 {
		t.Errorf("bet id %q is not a canonical uuid", bet.ID)
	}

	after := env.balance(t, user)
	if after != before-100 {
		t.Errorf("balance %v -> %v, want a 100 debit", before, after)
	}
	if got := env.countLedger(t, user, TxTypeBet); got != 1 {
		t.Errorf("crash_bet ledger rows = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// 2. An unauthenticated user cannot bet or cash out
// ---------------------------------------------------------------------------

func TestIntegration_PlaceBetRequiresUser(t *testing.T) {
	env := newTestEnv(t)
	if _, err := env.svc.PlaceBet(context.Background(), "", money.Cents(10000), 0, ""); err != apperrors.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestIntegration_CashOutRequiresUser(t *testing.T) {
	env := newTestEnv(t)
	if _, err := env.svc.CashOut(context.Background(), ""); err != apperrors.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 3. Insufficient funds
// ---------------------------------------------------------------------------

func TestIntegration_PlaceBetInsufficientFunds(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 5)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	start := env.clock.Now().Add(-time.Second)
	env.insertRound(t, domain.StateBetting, start, env.clock.Now().Add(30*time.Second), nil, seed, 0)

	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, ""); err != apperrors.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	if got := env.balance(t, user); got != 5 {
		t.Errorf("balance changed to %v on a rejected bet", got)
	}
	if got := env.countLedger(t, user, TxTypeBet); got != 0 {
		t.Errorf("rejected bet wrote %d ledger rows", got)
	}
}

func TestIntegration_PlaceBetValidatesAmount(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	start := env.clock.Now().Add(-time.Second)
	env.insertRound(t, domain.StateBetting, start, env.clock.Now().Add(30*time.Second), nil, seed, 0)

	// Below the configured minimum.
	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(100), 0, ""); err == nil {
		t.Error("a stake below the minimum must be rejected")
	}
	// Above the configured maximum.
	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(999999999), 0, ""); err == nil {
		t.Error("a stake above the maximum must be rejected")
	}
	// Zero and negative.
	if _, err := env.svc.PlaceBet(context.Background(), user, 0, 0, ""); err == nil {
		t.Error("a zero stake must be rejected")
	}
	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(-500), 0, ""); err == nil {
		t.Error("a negative stake must be rejected")
	}
	// An auto cash-out below 1.00x is nonsense.
	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 50, ""); err == nil {
		t.Error("an auto cash-out below 1.00x must be rejected")
	}
	if got := env.balance(t, user); got != 500 {
		t.Errorf("balance changed to %v after rejected bets", got)
	}
}

// ---------------------------------------------------------------------------
// 4. Betting after the deadline is refused
// ---------------------------------------------------------------------------

func TestIntegration_PlaceBetAfterDeadlineRejected(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	end := env.clock.Now().Add(-time.Second) // the window closed a second ago
	env.insertRound(t, domain.StateBetting, end.Add(-30*time.Second), end, nil, seed, 0)

	_, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, "")
	if err == nil {
		t.Fatal("expected the bet to be rejected after the deadline")
	}
	if !apperrors.IsCode(err, "BETTING_CLOSED") {
		t.Errorf("expected BETTING_CLOSED, got %v", err)
	}
	if got := env.balance(t, user); got != 500 {
		t.Errorf("balance changed to %v", got)
	}
}

func TestIntegration_PlaceBetRejectedWhileRoundIsRunning(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 300)

	startedAt := env.clock.Now().Add(-time.Second)
	end := env.clock.Now().Add(-2 * time.Second)
	env.insertRound(t, domain.StateRunning, end.Add(-30*time.Second), end, &startedAt, seed, 0)

	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, ""); err == nil {
		t.Fatal("expected the bet to be rejected once the round is running")
	}
}

func TestIntegration_PlaceBetWithoutRoundRejected(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	// No round exists in this room at all.
	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, ""); err == nil {
		t.Fatal("expected a clear refusal when no round is open")
	}
}

// ---------------------------------------------------------------------------
// 5 & 8. Duplicate requests never charge twice
// ---------------------------------------------------------------------------

func TestIntegration_DuplicateBetRejectedByConstraint(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	start := env.clock.Now().Add(-time.Second)
	env.insertRound(t, domain.StateBetting, start, env.clock.Now().Add(30*time.Second), nil, seed, 0)

	if _, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, "k1"); err != nil {
		t.Fatalf("first bet: %v", err)
	}
	// A different idempotency key so the replay cache cannot hide the conflict:
	// the database constraint must be the thing that stops it.
	_, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, "k2")
	if err == nil {
		t.Fatal("expected the second bet in the same round to be rejected")
	}
	if !apperrors.IsCode(err, "ALREADY_BET") {
		t.Errorf("expected ALREADY_BET, got %v", err)
	}
	if got := env.countLedger(t, user, TxTypeBet); got != 1 {
		t.Errorf("ledger rows = %d, want exactly 1", got)
	}
	if got := env.balance(t, user); got != 400 {
		t.Errorf("balance = %v, want 400 (debited once)", got)
	}
}

func TestIntegration_ConcurrentBetsChargeOnce(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	start := env.clock.Now().Add(-time.Second)
	env.insertRound(t, domain.StateBetting, start, env.clock.Now().Add(30*time.Second), nil, seed, 0)

	const attempts = 6
	var wg sync.WaitGroup
	successes := make([]bool, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := env.svc.PlaceBet(context.Background(), user, money.Cents(10000), 0, fmt.Sprintf("key-%d", idx))
			successes[idx] = err == nil
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, s := range successes {
		if s {
			ok++
		}
	}
	if ok != 1 {
		t.Errorf("%d concurrent bets succeeded, want exactly 1", ok)
	}
	if got := env.countLedger(t, user, TxTypeBet); got != 1 {
		t.Errorf("ledger rows = %d, want exactly 1", got)
	}
	if got := env.balance(t, user); got != 400 {
		t.Errorf("balance = %v, want 400", got)
	}
}

// ---------------------------------------------------------------------------
// 5. A player cashes out before the crash
// ---------------------------------------------------------------------------

func TestIntegration_CashOutBeforeCrashCreditsPayout(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, crashBP := env.seedWithCrashAtLeast(t, 300)

	startedAt := env.clock.Now().Add(-5 * time.Second)
	env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 10000)

	settled, err := env.svc.CashOut(context.Background(), user)
	if err != nil {
		t.Fatalf("cash out: %v", err)
	}
	if settled.Status != domain.BetStatusCashedOut {
		t.Fatalf("status = %s, want cashed_out", settled.Status)
	}

	expectedBP := domain.MultiplierAt(5, env.cfg.GrowthRate)
	if crashBP <= expectedBP {
		t.Fatalf("test seed crashed at %d bp, below the cash-out multiplier %d bp", crashBP, expectedBP)
	}
	if settled.CashoutMultiplier != domain.MultiplierValue(expectedBP) {
		t.Errorf("cash-out multiplier = %v, want %v", settled.CashoutMultiplier, domain.MultiplierValue(expectedBP))
	}
	expectedPayout := money.Cents(10000).MulByBasisPoints(expectedBP).Float()
	if settled.Payout != expectedPayout {
		t.Errorf("payout = %v, want %v", settled.Payout, expectedPayout)
	}
	if got := env.balance(t, user); got != 500-100+expectedPayout {
		t.Errorf("balance = %v, want %v", got, 500-100+expectedPayout)
	}
	if got := env.countLedger(t, user, TxTypePayout); got != 1 {
		t.Errorf("payout ledger rows = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// 6. A player cannot cash out after the crash
// ---------------------------------------------------------------------------

func TestIntegration_CashOutAfterCrashRejected(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed := env.seedCrashingImmediately(t) // crashes at 1.00x

	startedAt := env.clock.Now().Add(-time.Second)
	env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 10000)

	_, err := env.svc.CashOut(context.Background(), user)
	if err == nil {
		t.Fatal("expected cash-out after the crash to be refused")
	}
	if !apperrors.IsCode(err, "ROUND_CRASHED") {
		t.Errorf("expected ROUND_CRASHED, got %v", err)
	}
	if got := env.countLedger(t, user, TxTypePayout); got != 0 {
		t.Errorf("payout ledger rows = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// 7 & 9. A bet cannot be cashed out twice, even concurrently
// ---------------------------------------------------------------------------

func TestIntegration_CashOutTwicePaysOnce(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 300)

	startedAt := env.clock.Now().Add(-2 * time.Second)
	env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 10000)

	first, err := env.svc.CashOut(context.Background(), user)
	if err != nil {
		t.Fatalf("first cash out: %v", err)
	}
	second, err := env.svc.CashOut(context.Background(), user)
	if err != nil {
		t.Fatalf("second cash out should be an idempotent success, got %v", err)
	}

	if first.Payout != second.Payout || first.CashoutMultiplier != second.CashoutMultiplier {
		t.Errorf("second cash-out changed the result: %v/%v then %v/%v",
			first.CashoutMultiplier, first.Payout, second.CashoutMultiplier, second.Payout)
	}
	if got := env.countLedger(t, user, TxTypePayout); got != 1 {
		t.Errorf("payout ledger rows = %d, want exactly 1", got)
	}
	if got := env.balance(t, user); got != 500-100+first.Payout {
		t.Errorf("balance = %v, want %v", got, 500-100+first.Payout)
	}
}

func TestIntegration_ConcurrentCashOutsPayOnce(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 2000) // long window: timing never decides the outcome

	startedAt := env.clock.Now().Add(-2 * time.Second)
	env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 10000)

	const attempts = 8
	var wg sync.WaitGroup
	results := make([]*BetView, attempts)
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = env.svc.CashOut(context.Background(), user)
		}(i)
	}
	wg.Wait()

	var payout float64
	for i, r := range results {
		if errs[i] != nil {
			continue
		}
		if r.Status != domain.BetStatusCashedOut {
			t.Errorf("attempt %d returned status %s", i, r.Status)
		}
		payout = r.Payout
	}
	if got := env.countLedger(t, user, TxTypePayout); got != 1 {
		t.Fatalf("payout ledger rows = %d, want exactly 1", got)
	}
	if got := env.balance(t, user); got != 500-100+payout {
		t.Errorf("balance = %v, want %v", got, 500-100+payout)
	}
}

// ---------------------------------------------------------------------------
// 10. Uncashed bets are settled as losses
// ---------------------------------------------------------------------------

func TestIntegration_UncashedBetsSettleAsLosses(t *testing.T) {
	env := newTestEnv(t)
	winner := env.newUser(t, 500)
	loser := env.newUser(t, 500)
	seed, crashBP := env.seedWithCrashAtLeast(t, 300)

	startedAt := env.clock.Now().Add(-3 * time.Second)
	roundID := env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, winner, 10000)
	env.insertBetDirect(t, loser, 10000)

	// One player cashes out just past 2.00x, the other rides into the crash.
	// The clock sits a hair beyond the boundary so the multiplier is at least
	// 2.00x whatever the floating-point rounding of the inverse curve, and the
	// expected money is then derived from the multiplier the server recorded
	// rather than from an assumed exact 2.00x.
	const targetBP = 200
	cashoutAt := startedAt.Add(durationForMultiplier(targetBP, env.cfg.GrowthRate) + time.Millisecond)
	env.svc.SetClock(&fixedAt{t: cashoutAt})
	settled, err := env.svc.CashOut(context.Background(), winner)
	if err != nil {
		t.Fatalf("cash out: %v", err)
	}
	env.svc.SetClock(env.clock)
	if gotBP := int(money.FromFloat(settled.CashoutMultiplier)); gotBP < targetBP {
		t.Fatalf("cash-out multiplier = %d bp, want at least %d bp", gotBP, targetBP)
	}
	expectedPayoutCents := int64(money.Cents(10000).MulByBasisPoints(int(money.FromFloat(settled.CashoutMultiplier))))
	expectedPayout := money.Cents(expectedPayoutCents).Float()
	if settled.Payout != expectedPayout {
		t.Errorf("payout = %v, want %v (stake * recorded multiplier)", settled.Payout, expectedPayout)
	}

	crashAt := startedAt.Add(durationForMultiplier(crashBP, env.cfg.GrowthRate))
	env.clock.Set(crashAt.Add(time.Millisecond))
	env.stepEngine(t) // RUNNING -> CRASHED
	env.stepEngine(t) // CRASHED -> SETTLEMENT

	var loserStatus string
	if err := env.db.QueryRow(`SELECT status FROM game_bets WHERE round_id = $1 AND user_id = $2`, roundID, loser).Scan(&loserStatus); err != nil {
		t.Fatalf("read losing bet: %v", err)
	}
	if loserStatus != domain.BetStatusLost {
		t.Errorf("uncashed bet status = %s, want lost", loserStatus)
	}

	var state string
	var totalPayout int64
	if err := env.db.QueryRow(`SELECT state, total_payout_cents FROM game_rounds WHERE id = $1`, roundID).Scan(&state, &totalPayout); err != nil {
		t.Fatalf("read round: %v", err)
	}
	if state != domain.StateSettlement && state != domain.StateNextRound {
		t.Errorf("round state = %s, want settled", state)
	}
	if totalPayout != expectedPayoutCents {
		t.Errorf("round total payout = %d, want %d (only the winner)", totalPayout, expectedPayoutCents)
	}

	var active int
	if err := env.db.QueryRow(`SELECT COUNT(*) FROM game_bets WHERE round_id = $1 AND status = 'active'`, roundID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Errorf("%d bets still active after settlement", active)
	}

	// The loser's stake is gone; the winner is up by the payout.
	if got := env.balance(t, loser); got != 400 {
		t.Errorf("loser balance = %v, want 400", got)
	}
	if got := env.balance(t, winner); got != 500-100+expectedPayout {
		t.Errorf("winner balance = %v, want %v", got, 500-100+expectedPayout)
	}
}

// ---------------------------------------------------------------------------
// 11 & 14. Transitions happen in order; recovery cannot double-settle
// ---------------------------------------------------------------------------

func TestIntegration_RoundTransitionsFollowStateMachine(t *testing.T) {
	env := newTestEnv(t)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	now := env.clock.Now()
	id := env.insertRound(t, domain.StateWaiting, now.Add(-time.Second), now.Add(5*time.Second), nil, seed, 0)

	steps := []struct {
		want  string
		after time.Duration
	}{
		{domain.StateBetting, 0},
		{domain.StateRunning, 6 * time.Second},
		{domain.StateCrashed, 60 * time.Second},
	}
	for _, s := range steps {
		env.clock.Advance(s.after)
		env.stepEngine(t)
		if got := env.roundState(t, id); got != s.want {
			t.Fatalf("state = %s, want %s", got, s.want)
		}
	}

	env.clock.Advance(2 * time.Second)
	env.stepEngine(t)
	if got := env.roundState(t, id); got != domain.StateSettlement {
		t.Fatalf("state = %s, want SETTLEMENT", got)
	}
	env.clock.Advance(2 * time.Second)
	env.stepEngine(t)
	if got := env.roundState(t, id); got != domain.StateNextRound {
		t.Fatalf("state = %s, want NEXT_ROUND", got)
	}

	// A closed round must never go back to moving money.
	before := env.roundState(t, id)
	env.clock.Advance(30 * time.Second)
	env.stepEngine(t)
	if got := env.roundState(t, id); got != before {
		t.Errorf("a closed round changed state to %s", got)
	}
}

func TestIntegration_RecoveryDoesNotDoubleSettle(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)

	// A seed committing to a 1.00x crash makes the crash instant unambiguously
	// past, whatever the growth curve. Picking a merely "high enough" crash point
	// here would make the case dependent on how much time the round has run.
	seed := env.seedCrashingImmediately(t)
	crashBP := env.source.CrashPoint(seed, 1)

	startedAt := env.clock.Now().Add(-30 * time.Second)
	roundID := env.insertRound(t, domain.StateRunning, startedAt.Add(-60*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 10000)

	// The crash instant is long past, so the engine catches up immediately.
	env.stepEngine(t) // RUNNING -> CRASHED
	env.stepEngine(t) // CRASHED -> SETTLEMENT
	env.stepEngine(t) // idempotent replay
	env.stepEngine(t)

	var state string
	var totalStake, totalPayout int64
	var storedBP int
	if err := env.db.QueryRow(`SELECT state, total_stake_cents, total_payout_cents, COALESCE(crash_multiplier_bp,0)
		FROM game_rounds WHERE id = $1`, roundID).Scan(&state, &totalStake, &totalPayout, &storedBP); err != nil {
		t.Fatal(err)
	}
	if state != domain.StateSettlement && state != domain.StateNextRound {
		t.Fatalf("state = %s, want settled", state)
	}
	if storedBP != crashBP {
		t.Errorf("stored crash multiplier = %d, want %d (the seed-committed value)", storedBP, crashBP)
	}
	if totalStake != 10000 {
		t.Errorf("total stake = %d, want 10000 (not doubled by replay)", totalStake)
	}
	if totalPayout != 0 {
		t.Errorf("total payout = %d, want 0 (nobody cashed out)", totalPayout)
	}
	if got := env.balance(t, user); got != 400 {
		t.Errorf("balance = %v, want 400 (the stake is debited exactly once)", got)
	}
}

func TestIntegration_RecoverUnfinishedSweepsBacklog(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 200)

	// Two rounds left crashed by a previous process.
	var ids []string
	for i := 0; i < 2; i++ {
		startedAt := env.clock.Now().Add(-time.Duration(60+i*10) * time.Second)
		id := env.insertRound(t, domain.StateCrashed, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 250)
		ids = append(ids, id)
	}
	env.insertBetDirect(t, user, 10000)

	env.engine.recoverUnfinished(context.Background())

	for _, id := range ids {
		if got := env.roundState(t, id); got != domain.StateNextRound {
			t.Errorf("round %s state = %s, want NEXT_ROUND", id, got)
		}
	}

	var active int
	if err := env.db.QueryRow(`SELECT COUNT(*) FROM game_bets WHERE user_id = $1 AND status = 'active'`, user).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Errorf("%d bets left active after the recovery sweep", active)
	}
	if got := env.balance(t, user); got != 400 {
		t.Errorf("balance = %v, want 400", got)
	}
}

// ---------------------------------------------------------------------------
// Auto cash-out
// ---------------------------------------------------------------------------

func TestIntegration_AutoCashOutPaysExactlyTheTarget(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 400)

	startedAt := env.clock.Now().Add(-time.Second)
	roundID := env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirectWithAuto(t, user, 10000, 150)

	// Move past 1.50x and let the engine notice. Because the engine settles at
	// exactly the target, the payout must not drift with the tick interval.
	env.clock.Set(startedAt.Add(durationForMultiplier(200, env.cfg.GrowthRate)))
	env.stepEngine(t)

	var status string
	var multiplierBP int
	var payout float64
	if err := env.db.QueryRow(`SELECT status, COALESCE(cashout_multiplier_bp,0), payout_amount
		FROM game_bets WHERE round_id = $1 AND user_id = $2`, roundID, user).Scan(&status, &multiplierBP, &payout); err != nil {
		t.Fatal(err)
	}
	if status != domain.BetStatusCashedOut {
		t.Fatalf("auto cash-out status = %s, want cashed_out", status)
	}
	if multiplierBP != 150 {
		t.Errorf("auto cash-out settled at %d bp, want exactly 150", multiplierBP)
	}
	if payout != 150 {
		t.Errorf("payout = %v, want 150.00", payout)
	}
	if got := env.balance(t, user); got != 550 {
		t.Errorf("balance = %v, want 550", got)
	}
}

// ---------------------------------------------------------------------------
// 12 & 13. One authoritative round for everyone; reconnects recover state
// ---------------------------------------------------------------------------

func TestIntegration_AllClientsSeeTheSameAuthoritativeRound(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 500)
	bob := env.newUser(t, 500)
	seed, _ := env.seedWithCrashAtLeast(t, 500)

	startedAt := env.clock.Now().Add(-2 * time.Second)
	roundID := env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, alice, 10000)

	aliceState, err := env.svc.GetState(context.Background(), alice)
	if err != nil {
		t.Fatalf("alice state: %v", err)
	}
	bobState, err := env.svc.GetState(context.Background(), bob)
	if err != nil {
		t.Fatalf("bob state: %v", err)
	}

	if aliceState.Round == nil || bobState.Round == nil {
		t.Fatal("both clients must see the live round")
	}
	if aliceState.Round.ID != bobState.Round.ID || bobState.Round.ID != roundID {
		t.Errorf("clients disagree on the round: %s vs %s", aliceState.Round.ID, bobState.Round.ID)
	}
	if aliceState.Multiplier != bobState.Multiplier {
		t.Errorf("clients disagree on the multiplier: %v vs %v", aliceState.Multiplier, bobState.Multiplier)
	}
	if aliceState.Round.ServerSeed != "" {
		t.Error("the server seed leaked while the round was live")
	}

	// Reconnecting clients recover their own bet and the public player table.
	if bobState.MyBet != nil {
		t.Error("bob has no bet, so MyBet must be empty")
	}
	if aliceState.MyBet == nil {
		t.Fatal("alice must recover her bet after reconnecting")
	}
	if aliceState.MyBet.ID == "" || aliceState.MyBet.Stake != 100 {
		t.Errorf("recovered bet looks wrong: %+v", aliceState.MyBet)
	}
	if len(bobState.LiveBets) != 1 {
		t.Fatalf("live table has %d entries, want 1", len(bobState.LiveBets))
	}
	if bobState.LiveBets[0].DisplayName != "Test Player" {
		t.Errorf("display name = %q, want the player's name", bobState.LiveBets[0].DisplayName)
	}

	// After the round ends the seed is revealed and independently verifiable.
	env.clock.Set(startedAt.Add(120 * time.Second))
	env.stepEngine(t)
	env.stepEngine(t)

	proof, err := env.svc.VerifyRound(context.Background(), roundID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !proof.Verified || !proof.SeedMatchesCommit {
		t.Errorf("commit-reveal proof failed: %+v", proof)
	}
	if proof.CrashMultiplier != domain.MultiplierValue(env.source.CrashPoint(proof.ServerSeed, proof.Nonce)) {
		t.Error("the revealed seed must reproduce the recorded crash multiplier")
	}
}

func TestIntegration_VerifyRefusesLiveRound(t *testing.T) {
	env := newTestEnv(t)
	seed, _ := env.seedWithCrashAtLeast(t, 200)
	now := env.clock.Now()
	startedAt := now.Add(-time.Second)
	id := env.insertRound(t, domain.StateRunning, now.Add(-30*time.Second), startedAt, &startedAt, seed, 0)

	_, err := env.svc.VerifyRound(context.Background(), id)
	if err == nil {
		t.Fatal("verification must refuse a round whose seed is still sealed")
	}
	if !apperrors.IsCode(err, "ROUND_LIVE") {
		t.Errorf("expected ROUND_LIVE, got %v", err)
	}
}

func TestIntegration_HistoryAndMyBets(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 500)

	// Crashing at 1.00x makes the round over the moment it starts, so the engine
	// settles it on the ticks below regardless of the configured growth rate.
	seed := env.seedCrashingImmediately(t)
	crashBP := env.source.CrashPoint(seed, 1)

	startedAt := env.clock.Now().Add(-30 * time.Second)
	roundID := env.insertRound(t, domain.StateRunning, startedAt.Add(-60*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 10000)

	env.stepEngine(t) // crash
	env.stepEngine(t) // settle

	history, err := env.svc.History(context.Background(), 5)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("history must contain the settled round")
	}
	found := false
	for _, r := range history {
		if r.ID == roundID {
			found = true
			if r.CrashMultiplier != domain.MultiplierValue(crashBP) {
				t.Errorf("history crash multiplier = %v, want %v", r.CrashMultiplier, domain.MultiplierValue(crashBP))
			}
			if r.ServerSeed == "" {
				t.Error("a settled round must expose its seed in history")
			}
		}
	}
	if !found {
		t.Error("the settled round is missing from history")
	}

	bets, total, err := env.svc.MyBets(context.Background(), user, 10, 0)
	if err != nil {
		t.Fatalf("my bets: %v", err)
	}
	if total != 1 || len(bets) != 1 {
		t.Fatalf("my-bets returned %d rows (total %d), want 1", len(bets), total)
	}
	if bets[0].Status != domain.BetStatusLost {
		t.Errorf("bet status = %s, want lost", bets[0].Status)
	}
}

// ---------------------------------------------------------------------------
// 15. Monetary precision
// ---------------------------------------------------------------------------

func TestIntegration_MonetaryPrecisionOnCashOut(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 100)
	seed, _ := env.seedWithCrashAtLeast(t, 2000)

	startedAt := env.clock.Now().Add(-1 * time.Second)
	roundID := env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 3333) // a deliberately awkward stake: 33.33

	env.clock.Set(startedAt.Add(5 * time.Second))
	settled, err := env.svc.CashOut(context.Background(), user)
	if err != nil {
		t.Fatalf("cash out: %v", err)
	}

	expectedBP := domain.MultiplierAt(5, env.cfg.GrowthRate)
	expected := money.Cents(3333).MulByBasisPoints(expectedBP).Float()
	if settled.Payout != expected {
		t.Errorf("payout = %v, want exactly %v", settled.Payout, expected)
	}
	if cents := money.FromFloat(settled.Payout); cents.Float() != settled.Payout {
		t.Errorf("payout %v is not an exact cent amount", settled.Payout)
	}
	if got := env.balance(t, user); got != 100-33.33+expected {
		t.Errorf("balance = %v, want %v", got, 100-33.33+expected)
	}

	// The persisted ledger row and the round totals must agree with the payout.
	var ledgerAmount float64
	if err := env.db.QueryRow(`SELECT amount FROM wallet_transactions
		WHERE user_id = $1 AND type = $2`, user, TxTypePayout).Scan(&ledgerAmount); err != nil {
		t.Fatalf("read payout ledger row: %v", err)
	}
	if ledgerAmount != settled.Payout {
		t.Errorf("ledger amount %v != settled payout %v", ledgerAmount, settled.Payout)
	}

	var payoutCents int64
	if err := env.db.QueryRow(`SELECT COALESCE(SUM(payout_cents),0) FROM game_bets WHERE round_id = $1`, roundID).Scan(&payoutCents); err != nil {
		t.Fatal(err)
	}
	if payoutCents != int64(money.FromFloat(settled.Payout)) {
		t.Errorf("bet payout cents %d != settled payout %d", payoutCents, int64(money.FromFloat(settled.Payout)))
	}
}

// ---------------------------------------------------------------------------
// Wallet integration
// ---------------------------------------------------------------------------

func TestIntegration_BetAndPayoutUseTheExistingWalletLedger(t *testing.T) {
	env := newTestEnv(t)
	user := env.newUser(t, 1000)
	seed, _ := env.seedWithCrashAtLeast(t, 300)

	startedAt := env.clock.Now().Add(-3 * time.Second)
	env.insertRound(t, domain.StateRunning, startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0)
	env.insertBetDirect(t, user, 25000) // 250.00

	settled, err := env.svc.CashOut(context.Background(), user)
	if err != nil {
		t.Fatalf("cash out: %v", err)
	}

	expectedBP := domain.MultiplierAt(3, env.cfg.GrowthRate)
	if settled.Payout != money.Cents(25000).MulByBasisPoints(expectedBP).Float() {
		t.Errorf("payout = %v, want %v", settled.Payout, money.Cents(25000).MulByBasisPoints(expectedBP).Float())
	}

	// Ledger rows carry balance_before/balance_after so the wallet history is
	// consistent with the balance itself.
	rows, err := env.db.Query(`SELECT type, amount, balance_before, balance_after FROM wallet_transactions
		WHERE user_id = $1 ORDER BY created_at`, user)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var txType string
		var amount, before, after float64
		if err := rows.Scan(&txType, &amount, &before, &after); err != nil {
			t.Fatal(err)
		}
		count++
		switch txType {
		case TxTypeBet:
			if before-amount != after {
				t.Errorf("bet ledger row is inconsistent: %v - %v != %v", before, amount, after)
			}
		case TxTypePayout:
			if before+amount != after {
				t.Errorf("payout ledger row is inconsistent: %v + %v != %v", before, amount, after)
			}
		}
	}
	if count != 2 {
		t.Errorf("expected one stake row and one payout row, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Leadership, event contract, player names, currency and sweep cadence
// ---------------------------------------------------------------------------

// scriptedLocker stands in for the PostgreSQL advisory lock: it refuses (or
// errors) a configured number of times before granting, which is what a peer
// instance holding the lock, or a brief database outage at boot, looks like.
type scriptedLocker struct {
	mu       sync.Mutex
	attempts int
	refusals int
	failWith error
}

func (l *scriptedLocker) Acquire(context.Context, string) (bool, func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts++
	if l.attempts <= l.refusals {
		if l.failWith != nil {
			return false, nil, l.failWith
		}
		return false, func() {}, nil
	}
	return true, func() {}, nil
}

func (l *scriptedLocker) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.attempts
}

// recordedEvent captures one published realtime event.
type recordedEvent struct {
	channel string
	kind    string
	payload map[string]interface{}
}

// recordingPublisher records engine events so the event contract can be
// asserted without a Redis connection.
type recordingPublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (p *recordingPublisher) record(channel, kind string, payload interface{}) {
	fields, _ := payload.(map[string]interface{})
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, recordedEvent{channel: channel, kind: kind, payload: fields})
}

func (p *recordingPublisher) Publish(_ context.Context, kind, _, _ string, payload interface{}) error {
	p.record("", kind, payload)
	return nil
}

func (p *recordingPublisher) PublishToChannel(_ context.Context, channel, kind string, payload interface{}) error {
	p.record(channel, kind, payload)
	return nil
}

func (p *recordingPublisher) find(kind string) map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.events {
		if e.kind == kind {
			return e.payload
		}
	}
	return nil
}

// engineTakingOverAfterRetries builds an engine whose lock is refused/errors the
// given number of times, runs it, and asserts it eventually starts the clock.
func engineTakingOverAfterRetries(t *testing.T, locker *scriptedLocker) {
	t.Helper()
	env := newTestEnv(t)

	engine := NewEngine(env.rounds, env.bets, env.source, env.svc, locker, env.cfg, zap.NewNop())
	engine.SetClock(env.clock)
	engine.SetLeadershipRetry(5 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Run(ctx)
	}()

	var roundID string
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		if round, err := env.rounds.FindCurrent(context.Background(), env.room); err == nil {
			roundID = round.ID
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if roundID == "" {
		t.Fatal("engine never took the round clock after being refused it")
	}
	if got := locker.count(); got <= locker.refusals {
		t.Fatalf("engine gave up after %d lock attempts (it was refused %d times)", got, locker.refusals)
	}
	if state := env.roundState(t, roundID); state != domain.StateWaiting && state != domain.StateBetting {
		t.Fatalf("round started in state %s", state)
	}
}

func TestIntegration_EngineKeepsRetryingWhileAPeerHoldsTheLock(t *testing.T) {
	engineTakingOverAfterRetries(t, &scriptedLocker{refusals: 2})
}

func TestIntegration_EngineRetriesAfterATransientLockError(t *testing.T) {
	// A database error at boot must not leave the deployment with no engine
	// until someone restarts the process.
	engineTakingOverAfterRetries(t, &scriptedLocker{
		refusals: 1,
		failWith: errors.New("dial tcp: connection reset by peer"),
	})
}

func TestIntegration_BettingOpenEventAdvertisesBettingState(t *testing.T) {
	env := newTestEnv(t)
	publisher := &recordingPublisher{}
	env.engine.SetPublisher(publisher)

	seed, _ := env.seedWithCrashAtLeast(t, 200)
	now := env.clock.Now()
	id := env.insertRound(t, domain.StateWaiting, now.Add(-time.Second), now.Add(5*time.Second), nil, seed, 0)

	env.stepEngine(t)

	if state := env.roundState(t, id); state != domain.StateBetting {
		t.Fatalf("round state = %s, want BETTING", state)
	}
	payload := publisher.find("crash.betting_open")
	if payload == nil {
		t.Fatal("no crash.betting_open event published")
	}
	if payload["state"] != domain.StateBetting {
		t.Fatalf("crash.betting_open advertised state %v, want %s", payload["state"], domain.StateBetting)
	}
	if payload["round_id"] != id {
		t.Fatalf("crash.betting_open carried round %v, want %s", payload["round_id"], id)
	}
	if payload["betting_ends_at"] == nil {
		t.Fatal("crash.betting_open did not carry the betting deadline")
	}
}

func TestIntegration_PositionalPlayersResolveTheirNames(t *testing.T) {
	env := newTestEnv(t)
	first := env.newUser(t, 1000)
	second := env.newUser(t, 1000)
	env.renameUser(t, first, "Amina Bekele")
	env.renameUser(t, second, "Bereket Tesfaye")

	seed, crashBP := env.seedWithCrashAtLeast(t, 500)
	now := env.clock.Now()
	startedAt := now
	env.insertRound(t, domain.StateRunning, now.Add(-time.Minute), now, &startedAt, seed, crashBP)
	env.insertBetDirect(t, first, 100)
	env.insertBetDirect(t, second, 200)

	snapshot, err := env.svc.GetState(context.Background(), first)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	names := map[string]bool{}
	for _, bet := range snapshot.LiveBets {
		names[bet.DisplayName] = true
	}
	if !names["Amina Bekele"] || !names["Bereket Tesfaye"] {
		t.Fatalf("live table names = %v, want both players", names)
	}
	if names["Player"] {
		t.Fatal("a player row fell back to the placeholder name")
	}
	if snapshot.MyBet == nil || snapshot.MyBet.UserID != first {
		t.Fatal("the caller's own bet is missing from the snapshot")
	}
}

func TestIntegration_SnapshotReportsTheWalletCurrency(t *testing.T) {
	env := newTestEnv(t)
	player := env.newUser(t, 500)
	if _, err := env.db.Exec(`UPDATE wallets SET currency = 'USD' WHERE user_id = $1`, player); err != nil {
		t.Fatalf("set wallet currency: %v", err)
	}

	snapshot, err := env.svc.GetState(context.Background(), player)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if snapshot.Config.Currency != "USD" {
		t.Fatalf("currency = %q, want the wallet's own USD", snapshot.Config.Currency)
	}

	// A player who has never had a wallet sees the game's default instead of an
	// empty currency.
	noWallet := env.newUserWithoutWallet(t)
	guestSnapshot, err := env.svc.GetState(context.Background(), noWallet)
	if err != nil {
		t.Fatalf("get state for wallet-less player: %v", err)
	}
	if guestSnapshot.Config.Currency != defaultCurrency {
		t.Fatalf("currency = %q, want %q", guestSnapshot.Config.Currency, defaultCurrency)
	}
}

func TestIntegration_AutoCashoutSweepIsSpacedOutButNotMissed(t *testing.T) {
	env := newTestEnv(t)
	player := env.newUser(t, 1000)

	seed, crashBP := env.seedWithCrashAtLeast(t, 400)
	now := env.clock.Now()
	startedAt := now
	roundID := env.insertRound(t, domain.StateRunning, now.Add(-time.Minute), now, &startedAt, seed, crashBP)

	// The first pass records a sweep for this round.
	env.stepEngine(t)
	firstSweep := env.engine.autoCashout
	if firstSweep.roundID != roundID {
		t.Fatalf("sweep recorded for round %q, want %s", firstSweep.roundID, roundID)
	}

	// A second pass inside the interval must not scan the bets table again.
	env.stepEngine(t)
	if env.engine.autoCashout.last != firstSweep.last {
		t.Fatal("the auto cash-out sweep ran again inside its interval")
	}

	// A bet whose target is reached later must still be paid, and paid exactly
	// at the target it asked for.
	bet := env.insertBetDirectWithAuto(t, player, 1000, 150)
	env.clock.Advance(time.Duration(domain.DurationForMultiplier(150, env.cfg.GrowthRate)*float64(time.Second)) + time.Second)
	env.stepEngine(t)

	settled, err := env.bets.FindByRoundAndUser(context.Background(), roundID, player)
	if err != nil {
		t.Fatalf("reload bet: %v", err)
	}
	if settled.Status != domain.BetStatusCashedOut {
		t.Fatalf("bet status = %s, want cashed_out", settled.Status)
	}
	if settled.CashoutMultiplierBP != 150 {
		t.Fatalf("cash-out multiplier = %d, want the 150bp target", settled.CashoutMultiplierBP)
	}
	if got := env.countLedger(t, player, TxTypePayout); got != 1 {
		t.Fatalf("payout ledger rows = %d, want 1", got)
	}
	if settled.StakeCents != bet.StakeCents {
		t.Fatalf("settled stake = %d, want the bet's own %d", settled.StakeCents, bet.StakeCents)
	}
}

func TestIntegration_CrashRecordsTheDerivedCrashInstant(t *testing.T) {
	env := newTestEnv(t)
	publisher := &recordingPublisher{}
	env.engine.SetPublisher(publisher)

	seed, crashBP := env.seedWithCrashAtLeast(t, 200)
	now := env.clock.Now()
	startedAt := now
	roundID := env.insertRound(t, domain.StateRunning, now.Add(-time.Minute), now, &startedAt, seed, crashBP)

	// The engine notices the crash well after the multiplier passed the crash
	// point (any crash point, so the delay is derived from this round's own
	// crash instant); the recorded instant must still be the derived one.
	crashAfter := time.Duration(domain.DurationForMultiplier(crashBP, env.cfg.GrowthRate)*float64(time.Second)) + 30*time.Second
	env.clock.Advance(crashAfter)
	env.stepEngine(t)

	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("reload round: %v", err)
	}
	if round.CrashedAt == nil {
		t.Fatalf("the crash instant was not recorded (state %s, crash multiplier %d, want %d)",
			round.State, round.CrashMultiplierBP, crashBP)
	}
	want := domain.CrashAt(startedAt, crashBP, env.cfg.GrowthRate)
	if diff := round.CrashedAt.Sub(want); diff > 2*time.Second || diff < -2*time.Second {
		t.Fatalf("crashed_at = %v, want the derived instant %v (off by %v)", round.CrashedAt, want, diff)
	}
	if round.CrashMultiplierBP != crashBP {
		t.Fatalf("crash multiplier = %d, want %d", round.CrashMultiplierBP, crashBP)
	}

	// The announced event must carry the finished state and the same instant.
	payload := publisher.find("crash.crashed")
	if payload == nil {
		t.Fatal("no crash.crashed event published")
	}
	if payload["state"] != domain.StateCrashed {
		t.Fatalf("crash.crashed advertised state %v, want %s", payload["state"], domain.StateCrashed)
	}
	if payload["crash_multiplier_bp"] != crashBP {
		t.Fatalf("crash.crashed advertised %v, want %d", payload["crash_multiplier_bp"], crashBP)
	}
	announced, ok := payload["crashed_at"].(time.Time)
	if !ok {
		t.Fatalf("crash.crashed carried crashed_at as %T", payload["crashed_at"])
	}
	if diff := announced.Sub(want); diff > 2*time.Second || diff < -2*time.Second {
		t.Fatalf("announced crash instant = %v, want the derived %v (off by %v)", announced, want, diff)
	}
}

// ==========================================================================
// P2P settlement integration tests
// ==========================================================================

// helper: insert a P2P bet directly bypassing the betting window
func (e *testEnv) insertP2PBetDirect(t *testing.T, roundID, userID string, stakeCents int64) *domain.Bet {
	t.Helper()
	tx, err := e.bets.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	var walletID string
	var balance float64
	if err := tx.QueryRow(`SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&walletID, &balance); err != nil {
		t.Fatalf("find wallet: %v", err)
	}
	stake := money.Cents(stakeCents)
	if money.FromFloat(balance) < stake {
		t.Fatalf("wallet cannot cover stake")
	}
	newBalance := money.FromFloat(balance) - stake
	ledgerID := randomID()
	if _, err := tx.Exec(`INSERT INTO wallet_transactions
		(id,wallet_id,user_id,type,status,amount,balance_before,balance_after,reference,description,metadata,created_at,updated_at)
		VALUES ($1,$2,$3,'crash_bet','completed',$4,$5,$6,$7,'p2p test stake','null',NOW(),NOW())`,
		ledgerID, walletID, userID, stake.Float(), balance, newBalance.Float(), "crash_bet:"+ledgerID); err != nil {
		t.Fatalf("insert ledger: %v", err)
	}
	bet := &domain.Bet{
		ID:         randomID(),
		RoundID:    roundID,
		UserID:     userID,
		WalletID:   walletID,
		StakeCents: stake, StakeAmount: stake.Float(),
		Status: domain.BetStatusActive, DebitTxID: ledgerID,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := e.bets.CreateTx(context.Background(), tx, bet); err != nil {
		t.Fatalf("insert bet: %v", err)
	}
	if _, err := tx.Exec(`UPDATE wallets SET balance=$1,updated_at=NOW() WHERE id=$2`, newBalance.Float(), walletID); err != nil {
		t.Fatalf("update wallet: %v", err)
	}
	// Update prize_pool_cents on the round.
	if _, err := tx.Exec(`UPDATE game_rounds SET prize_pool_cents=prize_pool_cents+$1, bet_count=bet_count+1, updated_at=NOW() WHERE id=$2`, stakeCents, roundID); err != nil {
		t.Fatalf("update prize pool: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit P2P bet: %v", err)
	}
	return bet
}

// markCashedOutP2P sets a bet to cashed_out status with a given multiplier (no wallet credit, deferred).
func (e *testEnv) markCashedOutP2P(t *testing.T, roundID, userID string, multiplierBP int) {
	t.Helper()
	tx, err := e.bets.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE game_bets SET
		status='cashed_out', cashout_multiplier_bp=$1,
		payout_cents=0, payout_amount=0, cashed_out_at=NOW(), settled_at=NOW(), updated_at=NOW()
		WHERE round_id=$2 AND user_id=$3 AND status='active'`, multiplierBP, roundID, userID); err != nil {
		t.Fatalf("mark cashed out P2P: %v", err)
	}
	if _, err := tx.Exec(`UPDATE game_rounds SET cashout_count=cashout_count+1,updated_at=NOW() WHERE id=$1`, roundID); err != nil {
		t.Fatalf("update cashout count: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// ---------------------------------------------------------------------------
// P2P-1: Prize pool equals sum of stakes; platform has no stake
// ---------------------------------------------------------------------------

func TestIntegration_P2PPrizePoolEqualsStakes(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 1000)
	bob := env.newUser(t, 1000)

	seed, crashBP := env.seedWithCrashAtLeast(t, 400)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateRunning,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 0, 500)

	env.insertP2PBetDirect(t, roundID, alice, 10000) // 100.00
	env.insertP2PBetDirect(t, roundID, bob, 20000)   // 200.00

	var prizePool int64
	if err := env.db.QueryRow(`SELECT prize_pool_cents FROM game_rounds WHERE id=$1`, roundID).Scan(&prizePool); err != nil {
		t.Fatalf("read prize pool: %v", err)
	}
	if prizePool != 30000 {
		t.Errorf("prize_pool_cents = %d, want 30000", prizePool)
	}

	_ = crashBP // used by the seed helper
}

// ---------------------------------------------------------------------------
// P2P-2: Single winner gets prize pool minus fee on profit
// ---------------------------------------------------------------------------

func TestIntegration_P2PSingleWinnerGetsPoolMinusFee(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 1000)
	bob := env.newUser(t, 1000)

	seed, _ := env.seedWithCrashAtLeast(t, 300)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateCrashed,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 250, 500)

	// Both stake 100. Alice cashes out; Bob rides into crash.
	env.insertP2PBetDirect(t, roundID, alice, 10000)
	env.insertP2PBetDirect(t, roundID, bob, 10000)
	env.markCashedOutP2P(t, roundID, alice, 200) // Alice cashed at 2.00x

	// Load the round.
	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("find round: %v", err)
	}

	aliceBefore := env.balance(t, alice)
	bobBefore := env.balance(t, bob)

	result, err := env.svc.SettleP2PRound(context.Background(), round)
	if err != nil {
		t.Fatalf("SettleP2PRound: %v", err)
	}

	// Prize pool = 200.00 (two stakes of 100 each)
	// Alice is the sole winner, gross = 200.00
	// Gross profit = 200 - 100 = 100
	// Fee = 100 * 5% = 5.00
	// Net = 195.00
	if result.WinnerCount != 1 {
		t.Errorf("winner count = %d, want 1", result.WinnerCount)
	}
	if result.PrizePoolCents != 20000 {
		t.Errorf("prize_pool = %d, want 20000", result.PrizePoolCents)
	}
	if len(result.Winners) != 1 {
		t.Fatalf("winners len = %d, want 1", len(result.Winners))
	}
	w := result.Winners[0]
	if w.GrossPayoutCents != 20000 {
		t.Errorf("gross = %d, want 20000", w.GrossPayoutCents)
	}
	if w.GrossProfitCents != 10000 {
		t.Errorf("profit = %d, want 10000", w.GrossProfitCents)
	}
	if w.ServiceFeeCents != 500 {
		t.Errorf("fee = %d, want 500 (5.00)", w.ServiceFeeCents)
	}
	if w.NetPayoutCents != 19500 {
		t.Errorf("net = %d, want 19500 (195.00)", w.NetPayoutCents)
	}

	// Wallet check: Alice's balance must increase by exactly net_payout.
	aliceAfter := env.balance(t, alice)
	if aliceAfter != aliceBefore+195 {
		t.Errorf("Alice balance %v -> %v, expected +195 (net payout)", aliceBefore, aliceAfter)
	}
	// Bob lost; balance must be unchanged.
	if bobAfter := env.balance(t, bob); bobAfter != bobBefore {
		t.Errorf("Bob (loser) balance changed: %v -> %v", bobBefore, bobAfter)
	}

	// Fee ledger row must exist.
	if got := env.countLedger(t, alice, TxTypePayout); got < 1 {
		t.Errorf("payout ledger rows = %d, want ≥1", got)
	}
	if got := env.countLedger(t, alice, TxTypePlatformFee); got < 1 {
		t.Errorf("fee ledger rows = %d, want ≥1", got)
	}
}

// ---------------------------------------------------------------------------
// P2P-3: Loser pays no service fee
// ---------------------------------------------------------------------------

func TestIntegration_P2PLoserPaysNoFee(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 1000)
	bob := env.newUser(t, 1000)

	seed, _ := env.seedWithCrashAtLeast(t, 300)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateCrashed,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 250, 500)

	env.insertP2PBetDirect(t, roundID, alice, 10000)
	env.insertP2PBetDirect(t, roundID, bob, 10000)
	env.markCashedOutP2P(t, roundID, alice, 200)

	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("find round: %v", err)
	}
	bobBefore := env.balance(t, bob)

	if _, err := env.svc.SettleP2PRound(context.Background(), round); err != nil {
		t.Fatalf("SettleP2PRound: %v", err)
	}

	bobAfter := env.balance(t, bob)
	if bobAfter != bobBefore {
		t.Errorf("loser Bob: balance %v -> %v, expected no change", bobBefore, bobAfter)
	}
	// No fee ledger row for Bob.
	if got := env.countLedger(t, bob, TxTypePlatformFee); got != 0 {
		t.Errorf("loser has %d fee ledger rows, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// P2P-4: No winners — full pool goes to platform, no player receives funds
// ---------------------------------------------------------------------------

func TestIntegration_P2PNoWinnersPoolToPlatform(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 1000)
	bob := env.newUser(t, 1000)

	seed, _ := env.seedWithCrashAtLeast(t, 150)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateCrashed,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 120, 500)

	// Both lose — nobody cashes out.
	env.insertP2PBetDirect(t, roundID, alice, 10000)
	env.insertP2PBetDirect(t, roundID, bob, 10000)

	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("find round: %v", err)
	}
	aliceBefore := env.balance(t, alice)
	bobBefore := env.balance(t, bob)

	result, err := env.svc.SettleP2PRound(context.Background(), round)
	if err != nil {
		t.Fatalf("SettleP2PRound: %v", err)
	}
	if result.WinnerCount != 0 {
		t.Errorf("winner count = %d, want 0", result.WinnerCount)
	}
	if result.PlatformRevenueCents != 20000 {
		t.Errorf("platform revenue = %d, want 20000", result.PlatformRevenueCents)
	}
	// No player balance changes.
	if env.balance(t, alice) != aliceBefore {
		t.Error("Alice balance changed on no-winner round")
	}
	if env.balance(t, bob) != bobBefore {
		t.Error("Bob balance changed on no-winner round")
	}
}

// ---------------------------------------------------------------------------
// P2P-5: Multiple winners — proportional distribution stays within pool
// ---------------------------------------------------------------------------

func TestIntegration_P2PMultipleWinnersProportional(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 2000)
	bob := env.newUser(t, 2000)
	carol := env.newUser(t, 2000)

	seed, _ := env.seedWithCrashAtLeast(t, 500)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateCrashed,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 400, 500)

	// All three stake 100 each. Prize pool = 300.
	env.insertP2PBetDirect(t, roundID, alice, 10000)
	env.insertP2PBetDirect(t, roundID, bob, 10000)
	env.insertP2PBetDirect(t, roundID, carol, 10000)

	// Alice cashed at 3.00x (BP=300), Bob at 1.00x (BP=100). Carol crashed.
	env.markCashedOutP2P(t, roundID, alice, 300)
	env.markCashedOutP2P(t, roundID, bob, 100)

	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("find round: %v", err)
	}

	aliceBefore := env.balance(t, alice)
	bobBefore := env.balance(t, bob)
	carolBefore := env.balance(t, carol)

	result, err := env.svc.SettleP2PRound(context.Background(), round)
	if err != nil {
		t.Fatalf("SettleP2PRound: %v", err)
	}

	if result.WinnerCount != 2 {
		t.Errorf("winner count = %d, want 2", result.WinnerCount)
	}

	// Total BP = 300+100 = 400.
	// Alice gross = 300 * 300/400 = 225.00  profit=125  fee=6.25->625  net=21875
	// Bob gross   = 300 * 100/400 = 75.00   profit=-25 (below stake)  fee=0   net=75
	var totalNet money.Cents
	for _, w := range result.Winners {
		totalNet += w.NetPayoutCents
		if w.NetPayoutCents > result.PrizePoolCents {
			t.Errorf("winner net %d exceeds pool %d", w.NetPayoutCents, result.PrizePoolCents)
		}
	}
	if totalNet > result.PrizePoolCents {
		t.Errorf("total net payouts %d exceed pool %d", totalNet, result.PrizePoolCents)
	}

	// Alice gained, Bob gained (less than Alice), Carol unchanged.
	if env.balance(t, alice) <= aliceBefore {
		t.Error("Alice (higher cashout) should have gained")
	}
	if env.balance(t, bob) <= bobBefore {
		t.Error("Bob (lower cashout) should have gained (at least stake back)")
	}
	if env.balance(t, carol) != carolBefore {
		t.Error("Carol (crashed) should have unchanged balance")
	}
}

// ---------------------------------------------------------------------------
// P2P-6: Settlement is idempotent — re-running does not double-credit
// ---------------------------------------------------------------------------

func TestIntegration_P2PSettlementIsIdempotent(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 1000)
	bob := env.newUser(t, 1000)

	seed, _ := env.seedWithCrashAtLeast(t, 300)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateCrashed,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 250, 500)

	env.insertP2PBetDirect(t, roundID, alice, 10000)
	env.insertP2PBetDirect(t, roundID, bob, 10000)
	env.markCashedOutP2P(t, roundID, alice, 200)

	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("find round: %v", err)
	}

	if _, err := env.svc.SettleP2PRound(context.Background(), round); err != nil {
		t.Fatalf("first settlement: %v", err)
	}
	aliceAfterFirst := env.balance(t, alice)
	ledgerCountFirst := env.countLedger(t, alice, TxTypePayout)

	// Second call should be a no-op (idempotent).
	if _, err := env.svc.SettleP2PRound(context.Background(), round); err != nil {
		t.Fatalf("second settlement: %v", err)
	}
	aliceAfterSecond := env.balance(t, alice)
	ledgerCountSecond := env.countLedger(t, alice, TxTypePayout)

	if aliceAfterSecond != aliceAfterFirst {
		t.Errorf("balance changed on second settlement: %v -> %v", aliceAfterFirst, aliceAfterSecond)
	}
	if ledgerCountSecond != ledgerCountFirst {
		t.Errorf("ledger rows grew on second settlement: %d -> %d", ledgerCountFirst, ledgerCountSecond)
	}
}

// ---------------------------------------------------------------------------
// P2P-7: Zero fee when winner's gross return does not exceed stake
// ---------------------------------------------------------------------------

func TestIntegration_P2PNoFeeWhenNoProfit(t *testing.T) {
	env := newTestEnv(t)
	alice := env.newUser(t, 1000)
	bob := env.newUser(t, 1000)

	seed, _ := env.seedWithCrashAtLeast(t, 400)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateCrashed,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 300, 500)

	// Alice stakes 100, Bob stakes 200. Prize pool = 300.
	// Alice cashes at very low multiplier (BP=1). Bob has BP=300.
	// Total BP = 301.
	// Alice gross = 300 * 1/301 ≈ 0.99 < 100 stake => no profit, no fee.
	// Bob gross = 300 * 300/301 ≈ 299.00 > 200 stake => small profit.
	env.insertP2PBetDirect(t, roundID, alice, 10000)
	env.insertP2PBetDirect(t, roundID, bob, 20000)
	env.markCashedOutP2P(t, roundID, alice, 100) // BP=100 (1.00x)
	env.markCashedOutP2P(t, roundID, bob, 300)   // BP=300 (3.00x)

	// Override the prize pool to match what was actually staked.
	if _, err := env.db.Exec(`UPDATE game_rounds SET prize_pool_cents=30000 WHERE id=$1`, roundID); err != nil {
		t.Fatalf("set prize pool: %v", err)
	}

	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("find round: %v", err)
	}

	result, err := env.svc.SettleP2PRound(context.Background(), round)
	if err != nil {
		t.Fatalf("SettleP2PRound: %v", err)
	}

	for _, w := range result.Winners {
		if w.UserID == alice && w.ServiceFeeCents != 0 {
			t.Errorf("Alice (no profit) should have 0 fee, got %d", w.ServiceFeeCents)
		}
	}
}

// ---------------------------------------------------------------------------
// P2P-8: Net payouts never exceed available prize pool
// ---------------------------------------------------------------------------

func TestIntegration_P2PNetPayoutsNeverExceedPool(t *testing.T) {
	env := newTestEnv(t)
	users := make([]string, 5)
	for i := range users {
		users[i] = env.newUser(t, 2000)
	}

	seed, _ := env.seedWithCrashAtLeast(t, 600)
	startedAt := env.clock.Now().Add(-5 * time.Second)
	roundID := env.insertP2PRound(t, domain.StateCrashed,
		startedAt.Add(-30*time.Second), startedAt, &startedAt, seed, 500, 500)

	stakes := []int64{10000, 10000, 10000, 10000, 10000} // 5 × 100
	cashoutBPs := []int{500, 300, 200, 0, 0}             // first 3 cash out, last 2 crash
	for i, u := range users {
		env.insertP2PBetDirect(t, roundID, u, stakes[i])
		if cashoutBPs[i] > 0 {
			env.markCashedOutP2P(t, roundID, u, cashoutBPs[i])
		}
	}

	round, err := env.rounds.FindByID(context.Background(), roundID)
	if err != nil {
		t.Fatalf("find round: %v", err)
	}

	result, err := env.svc.SettleP2PRound(context.Background(), round)
	if err != nil {
		t.Fatalf("SettleP2PRound: %v", err)
	}

	if result.TotalNetPayoutCents > result.PrizePoolCents {
		t.Errorf("total net payout %d exceeds prize pool %d", result.TotalNetPayoutCents, result.PrizePoolCents)
	}
}
