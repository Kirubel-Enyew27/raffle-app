package domain

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/raffle-app/backend/pkg/money"
)

// Sentinel errors. Application-level code maps these onto HTTP responses so the
// rule that failed is never ambiguous.
var (
	ErrNoLiveRound   = errors.New("no live crash round")
	ErrDuplicateBet  = errors.New("bet already placed for this round")
	ErrBetNotActive  = errors.New("bet is not active")
	ErrRoundFinished = errors.New("round is finished")
)

// RoundRepository persists rounds and the authoritative timeline.
type RoundRepository interface {
	// Create inserts a new round.
	Create(ctx context.Context, round *Round) error

	// FindByID loads a round by id.
	FindByID(ctx context.Context, id string) (*Round, error)

	// FindLive returns the single money-moving round for a room (WAITING,
	// BETTING or RUNNING), or ErrNoLiveRound. Betting and cash-out use this.
	FindLive(ctx context.Context, room string) (*Round, error)

	// FindLiveForUpdate locks the live round for a room inside tx.
	FindLiveForUpdate(ctx context.Context, tx *sql.Tx, room string) (*Round, error)

	// FindCurrent returns the round the engine should be driving: the newest
	// round that has not finished its whole lifecycle yet, which includes the
	// post-crash states CRASHED and SETTLEMENT. The engine must use this (not
	// FindLive) or a crashed round would never be settled.
	FindCurrent(ctx context.Context, room string) (*Round, error)

	// FindByRoomAndState returns rounds in a given state, newest first.
	FindByRoomAndState(ctx context.Context, room string, states []string, limit int) ([]*Round, error)

	// MarkBettingStartedTx moves WAITING -> BETTING. Reports whether it applied.
	MarkBettingStartedTx(ctx context.Context, tx *sql.Tx, roundID string) (bool, error)

	// MarkRunningTx moves BETTING -> RUNNING and stamps started_at.
	MarkRunningTx(ctx context.Context, tx *sql.Tx, roundID string, startedAt time.Time) (bool, error)

	// MarkCrashedTx moves RUNNING -> CRASHED, storing the crash multiplier.
	MarkCrashedTx(ctx context.Context, tx *sql.Tx, roundID string, crashBP int, crashedAt time.Time) (bool, error)

	// MarkSettledTx moves CRASHED -> SETTLEMENT and stores the final totals.
	MarkSettledTx(ctx context.Context, tx *sql.Tx, roundID string, totalStakeCents, totalPayoutCents int64, settledAt time.Time) (bool, error)

	// MarkSettledP2PTx moves CRASHED -> SETTLEMENT and stores P2P-specific totals.
	MarkSettledP2PTx(ctx context.Context, tx *sql.Tx, roundID string, totalStakeCents, totalPayoutCents, prizePoolCents, totalFeeCents int64, settledAt time.Time) (bool, error)

	// MarkNextRoundTx moves SETTLEMENT -> NEXT_ROUND.
	MarkNextRoundTx(ctx context.Context, tx *sql.Tx, roundID string) (bool, error)

	// Recent returns settled rounds newest first, for the history strip.
	Recent(ctx context.Context, room string, limit int) ([]*Round, error)

	// AdjustTotalsTx accumulates stake/payout/bet counters on the round.
	AdjustTotalsTx(ctx context.Context, tx *sql.Tx, roundID string, stakeDelta, payoutDelta int64, betDelta, cashoutDelta int) error

	// NextRoundNumber allocates the next round number.
	NextRoundNumber(ctx context.Context) (int64, error)

	// BeginTx opens a transaction on the underlying pool.
	BeginTx(ctx context.Context) (*sql.Tx, error)
}

// RoundTotals are absolute totals recomputed from the bets table. Settlement
// writes these instead of incrementing counters so replaying a settlement can
// never double-count.
type RoundTotals struct {
	StakeCents   int64
	PayoutCents  int64
	BetCount     int64
	CashoutCount int64
}

// P2PSettlementBet carries the data needed to settle one winner in P2P mode.
type P2PSettlementBet struct {
	BetID            string
	UserID           string
	WalletID         string
	StakeCents       money.Cents
	CashoutBP        int
	GrossPayoutCents money.Cents
	GrossProfitCents money.Cents
	ServiceFeeCents  money.Cents
	NetPayoutCents   money.Cents
}

// BetRepository persists player bets and enforces exactly-once settlement.
type BetRepository interface {
	// CreateTx inserts a bet. A duplicate (round, user) surfaces as ErrDuplicateBet.
	CreateTx(ctx context.Context, tx *sql.Tx, bet *Bet) error

	// FindByRoundAndUser loads a player's bet in a round.
	FindByRoundAndUser(ctx context.Context, roundID, userID string) (*Bet, error)

	// FindByRoundAndUserTx loads a player's bet with a row lock inside tx.
	FindByRoundAndUserTx(ctx context.Context, tx *sql.Tx, roundID, userID string) (*Bet, error)

	// FindByRound returns every bet in a round, newest first (public activity).
	FindByRound(ctx context.Context, roundID string) ([]*Bet, error)

	// FindActiveByRound returns the still-active bets for a round.
	FindActiveByRound(ctx context.Context, roundID string) ([]*Bet, error)

	// FindCashedOutByRound returns bets in cashed_out status for a round,
	// used at P2P settlement to identify the winners.
	FindCashedOutByRound(ctx context.Context, roundID string) ([]*Bet, error)

	// FindByUser returns a player's bets, newest first.
	FindByUser(ctx context.Context, userID string, limit, offset int) ([]*Bet, error)

	// CountByUser counts a player's bets.
	CountByUser(ctx context.Context, userID string) (int64, error)

	// TotalsForRoundTx recomputes absolute totals from the persisted bets inside
	// the settlement transaction.
	TotalsForRoundTx(ctx context.Context, tx *sql.Tx, roundID string) (*RoundTotals, error)

	// MarkCashedOutTx flips an active bet to cashed_out, recording the
	// multiplier at cash-out. The status predicate is the exactly-once guard.
	// In P2P mode payout columns are 0 at this point (set at settlement).
	MarkCashedOutTx(ctx context.Context, tx *sql.Tx, betID string, multiplierBP int, payoutCents int64, creditTxID string, at time.Time) (bool, error)

	// MarkAllLostTx flips every still-active bet in a round to lost and returns
	// how many rows changed. Re-running matches zero rows, so crash settlement
	// is idempotent and safe to replay after a restart.
	MarkAllLostTx(ctx context.Context, tx *sql.Tx, roundID string, at time.Time) (int64, error)

	// SettleP2PWinnerTx writes the P2P settlement figures onto a cashed-out bet
	// and links the fee transaction. Idempotent: re-running on an already-settled
	// bet matches zero rows.
	SettleP2PWinnerTx(ctx context.Context, tx *sql.Tx, betID string, grossPayout, grossProfit, serviceFee, netPayout int64, creditTxID, feeTxID string, settledAt time.Time) (bool, error)

	// BeginTx opens a transaction on the underlying pool.
	BeginTx(ctx context.Context) (*sql.Tx, error)
}

// WalletRepository is the slice of the existing wallet the crash game needs.
// It mirrors the ticket module's wallet adapter (same tables, same helpers) so
// the game reuses the existing ledger instead of inventing a second one.
type WalletRepository interface {
	// EnsureWalletTx returns the player's wallet row locked FOR UPDATE, creating
	// it first if the player has never had one (the same lazy creation the
	// wallet service performs).
	EnsureWalletTx(ctx context.Context, tx *sql.Tx, userID string) (*Wallet, error)
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error
	CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *WalletTransaction) error
	// CurrencyFor reads the currency of the player's existing wallet without
	// locking it. An empty string means the player has no wallet yet, in which
	// case the caller falls back to the default the game uses when it creates
	// one, so the UI never shows the balance in two different currencies.
	CurrencyFor(ctx context.Context, userID string) (string, error)
}

// SettingsRepository reads and writes the admin-configurable crash game
// settings stored in the crash_settings table.
type SettingsRepository interface {
	// Get retrieves a setting value by key.
	Get(ctx context.Context, key string) (string, error)
	// GetAll retrieves all settings as a key→value map.
	GetAll(ctx context.Context) (map[string]string, error)
	// Set writes a setting value and records who changed it.
	Set(ctx context.Context, key, value, updatedByUserID string) error
}

// Wallet mirrors the wallets row (subset used by the game).
type Wallet struct {
	ID       string
	UserID   string
	Balance  float64
	Currency string
}

// WalletTransaction mirrors the wallet_transactions row.
type WalletTransaction struct {
	ID            string
	WalletID      string
	UserID        string
	Type          string
	Status        string
	Amount        float64
	BalanceBefore float64
	BalanceAfter  float64
	Reference     string
	Description   string
	Metadata      map[string]interface{}
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PlayerInfo resolves the display name shown in the live multiplayer table.
type PlayerInfo struct {
	UserID   string
	FullName string
	Email    string
}

// PlayerRepository resolves public display names without exposing private data.
type PlayerRepository interface {
	FindByID(ctx context.Context, userID string) (*PlayerInfo, error)
	// FindByIDs resolves many players in one round trip. The live table and the
	// state snapshot need a name per bet, and doing that one query at a time
	// turns a single screen refresh into an N+1 query storm.
	FindByIDs(ctx context.Context, userIDs []string) (map[string]*PlayerInfo, error)
}

// Locker provides the single-authority guarantee across application instances.
// Only the instance holding the lock runs the round engine.
type Locker interface {
	// Acquire attempts to take the engine lock. The returned release function
	// must be called on shutdown.
	Acquire(ctx context.Context, key string) (acquired bool, release func(), err error)
}

// CrashPointSource generates crash points and their commit-reveal material.
type CrashPointSource interface {
	// Generate creates a fresh server seed, its public commitment and the crash
	// point that seed produces, without revealing the seed.
	Generate(nonce int64) (*SeedCommitment, error)

	// CrashPoint recomputes the crash point for a revealed seed (verification).
	CrashPoint(serverSeed string, nonce int64) int

	// Verify checks a revealed seed against a published commitment.
	Verify(serverSeed, serverSeedHash string) bool
}

// SeedCommitment is the commit-reveal material for one round. CrashMultiplierBP
// stays server-side until the round crashes.
type SeedCommitment struct {
	ServerSeed        string
	ServerSeedHash    string
	Nonce             int64
	CrashMultiplierBP int
}
