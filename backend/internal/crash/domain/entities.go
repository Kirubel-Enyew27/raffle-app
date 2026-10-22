package domain

import (
	"time"

	"github.com/raffle-app/backend/pkg/money"
)

// Round states. The engine moves strictly through this order:
//
//	WAITING -> BETTING -> RUNNING -> CRASHED -> SETTLEMENT -> NEXT_ROUND
//
// NEXT_ROUND is the short result-display window; once it elapses the engine
// creates the next round in WAITING.
const (
	StateWaiting    = "WAITING"
	StateBetting    = "BETTING"
	StateRunning    = "RUNNING"
	StateCrashed    = "CRASHED"
	StateSettlement = "SETTLEMENT"
	StateNextRound  = "NEXT_ROUND"
)

// Bet states. "active" is the only non-terminal state.
const (
	BetStatusActive    = "active"
	BetStatusCashedOut = "cashed_out"
	BetStatusLost      = "lost"
)

// TerminalBetStatuses are the states a bet can no longer leave.
func IsTerminalBetStatus(status string) bool {
	return status == BetStatusCashedOut || status == BetStatusLost
}

// IsLiveRoundState reports whether a round is still allowed to move money
// (and therefore occupies the single live-round slot for its room).
func IsLiveRoundState(state string) bool {
	return state == StateWaiting || state == StateBetting || state == StateRunning
}

// IsFinishedRoundState reports whether a round can no longer accept money and
// its outcome is final.
func IsFinishedRoundState(state string) bool {
	return state == StateCrashed || state == StateSettlement || state == StateNextRound
}

// NextState returns the successor state and whether the transition is valid.
// The engine is the only caller; keeping the table here makes the ordering
// testable without touching the database.
func NextState(state string) (string, bool) {
	switch state {
	case StateWaiting:
		return StateBetting, true
	case StateBetting:
		return StateRunning, true
	case StateRunning:
		return StateCrashed, true
	case StateCrashed:
		return StateSettlement, true
	case StateSettlement:
		return StateNextRound, true
	default:
		return "", false
	}
}

// Round is one shared crash round. Every connected player in a room sees the
// same round, multiplier timeline and crash point.
type Round struct {
	ID          string
	Room        string
	RoundNumber int64
	State       string

	// Commit-reveal fairness metadata. ServerSeed must never leave the server
	// while the round is live.
	ServerSeed            string
	ServerSeedHash        string
	Nonce                 int64
	CommitmentPublishedAt time.Time

	// Authoritative timeline.
	BettingStartsAt time.Time
	BettingEndsAt   time.Time
	StartedAt       *time.Time
	CrashedAt       *time.Time
	SettledAt       *time.Time

	// Outcome, derived from the seed at crash time.
	CrashMultiplierBP int

	TotalStakeCents  money.Cents
	TotalPayoutCents money.Cents
	BetCount         int
	CashoutCount     int

	// P2P fields — snapshotted from settings at round creation time so that a
	// settings change never affects a round already in flight.
	P2PMode         bool
	PrizePoolCents  money.Cents
	ServiceFeeBP    int
	MinParticipants int
	TotalFeeCents   money.Cents

	CreatedAt time.Time
	UpdatedAt time.Time
}

// BettingOpen reports whether new bets are accepted at time now.
func (r *Round) BettingOpen(now time.Time) bool {
	return r.State == StateBetting && !now.Before(r.BettingStartsAt) && now.Before(r.BettingEndsAt)
}

// IsRunning reports whether players may cash out at time now.
func (r *Round) IsRunning(now time.Time) bool {
	return r.State == StateRunning && r.StartedAt != nil && !now.Before(*r.StartedAt)
}

// Elapsed returns how long the multiplier has been climbing.
func (r *Round) Elapsed(now time.Time) time.Duration {
	if r.StartedAt == nil {
		return 0
	}
	if r.CrashedAt != nil && r.CrashedAt.Before(now) {
		return r.CrashedAt.Sub(*r.StartedAt)
	}
	return now.Sub(*r.StartedAt)
}

// FairnessRevealed reports whether the server seed may be disclosed. The seed
// is only disclosed once the round outcome is final, so a live round can never
// leak its crash point.
func (r *Round) FairnessRevealed() bool {
	return IsFinishedRoundState(r.State) && r.CrashMultiplierBP > 0
}

// HasMinParticipants reports whether the round has enough bets to start.
// When MinParticipants is 0 (unset) we use 1 as the floor.
func (r *Round) HasMinParticipants() bool {
	min := r.MinParticipants
	if min <= 0 {
		min = 1
	}
	return r.BetCount >= min
}

// Bet is one player's stake in one round.
type Bet struct {
	ID       string
	RoundID  string
	UserID   string
	WalletID string

	StakeCents  money.Cents
	StakeAmount float64

	Status string

	AutoCashoutBP       int
	CashoutMultiplierBP int
	PayoutCents         money.Cents
	PayoutAmount        float64

	// P2P settlement fields. Populated only for cashed-out bets at settlement.
	GrossPayoutCents money.Cents
	GrossProfitCents money.Cents
	ServiceFeeCents  money.Cents
	NetPayoutCents   money.Cents

	DebitTxID  string
	CreditTxID string
	FeeTxID    string

	CashedOutAt *time.Time
	SettledAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsActive reports whether the bet can still be cashed out.
func (b *Bet) IsActive() bool {
	return b.Status == BetStatusActive
}

// PayoutFor computes the exact payout for cashing out at multiplierBP.
// NOTE: In P2P mode this is used only to record the cash-out multiplier for
// the proportional distribution; the actual payout is computed at settlement.
func (b *Bet) PayoutFor(multiplierBP int) money.Cents {
	return b.StakeCents.MulByBasisPoints(multiplierBP)
}

// P2PGrossReturn computes the winner's gross share of the prize pool given the
// total cashout multiplier weight across all winners.
//
//	gross_return = prize_pool_cents * cashout_multiplier_bp / total_cashout_bp_sum
//
// Integer-only; any remainder is dropped (sub-cent, favours the platform).
func P2PGrossReturn(prizePoolCents money.Cents, cashoutBP int, totalCashoutBPSum int) money.Cents {
	if totalCashoutBPSum <= 0 || cashoutBP <= 0 {
		return 0
	}
	// Use int64 to avoid overflow: pool up to ~10^10 * cashout up to ~10^5
	// product < 10^15, well within int64 range.
	return money.Cents(int64(prizePoolCents) * int64(cashoutBP) / int64(totalCashoutBPSum))
}

// P2PServiceFee computes the platform service fee on a winner's gross profit.
//
//	fee = max(0, gross_profit) * service_fee_bp / 10000
//
// Returns 0 when gross profit is zero or negative (player did not profit).
func P2PServiceFee(grossProfitCents money.Cents, serviceFeeBP int) money.Cents {
	if grossProfitCents <= 0 || serviceFeeBP <= 0 {
		return 0
	}
	return money.Cents(int64(grossProfitCents) * int64(serviceFeeBP) / 10000)
}

// RoundConfig holds every timing and limit knob for the game. Values come from
// the environment (see pkg/config) so nothing is hardcoded in the engine.
type RoundConfig struct {
	Room string

	WaitingSeconds   float64
	BettingSeconds   float64
	CrashedSeconds   float64
	NextRoundSeconds float64

	// GrowthRate is the exponential base per second: multiplier(t) = e^(rate*t).
	GrowthRate float64

	// HouseEdgeBP is still used to shape the crash point distribution (keeps
	// the curve the same). In P2P mode it no longer represents platform revenue
	// since no house money is at stake; revenue comes from service fees only.
	HouseEdgeBP int

	// MaxMultiplierBP caps how long a round can run (100000 == 1000.00x).
	MaxMultiplierBP int

	MinBetCents money.Cents
	MaxBetCents money.Cents

	// TickInterval is how often the engine advances/observes a running round.
	TickInterval time.Duration
	// BroadcastInterval is how often multiplier snapshots are published.
	BroadcastInterval time.Duration

	Enabled bool

	// P2P configuration — snapshotted per round at creation time.
	ServiceFeeBP    int // default 500 (5.00%)
	MinParticipants int // default 2
	MaxParticipants int // default 100

	// PlatformUserID is the user account that receives service fee credits.
	// If empty the platform fee is recorded in wallet_transactions only (no
	// wallet balance is credited to a specific account).
	PlatformUserID string
}

// DefaultConfig mirrors the config package defaults; it exists so tests and the
// service can construct a sane configuration without touching the environment.
func DefaultConfig() RoundConfig {
	return RoundConfig{
		Room:              "default",
		WaitingSeconds:    5,
		BettingSeconds:    30,
		CrashedSeconds:    5,
		NextRoundSeconds:  3,
		GrowthRate:        0.07,
		HouseEdgeBP:       300,
		MaxMultiplierBP:   100000,
		MinBetCents:       money.Cents(1000),    // 10.00
		MaxBetCents:       money.Cents(1000000), // 10,000.00
		TickInterval:      50 * time.Millisecond,
		BroadcastInterval: 250 * time.Millisecond,
		Enabled:           true,
		ServiceFeeBP:      500,
		MinParticipants:   2,
		MaxParticipants:   100,
	}
}

// Validate rejects obviously broken configuration rather than letting the
// engine misbehave at runtime.
func (c RoundConfig) Validate() error {
	switch {
	case c.Room == "":
		return errConfig("room must not be empty")
	case c.BettingSeconds <= 0:
		return errConfig("betting window must be positive")
	case c.WaitingSeconds < 0 || c.CrashedSeconds < 0 || c.NextRoundSeconds < 0:
		return errConfig("display windows must not be negative")
	case c.GrowthRate <= 0:
		return errConfig("growth rate must be positive")
	case c.HouseEdgeBP < 0 || c.HouseEdgeBP >= 10000:
		return errConfig("house edge must be in [0, 10000) basis points")
	case c.MaxMultiplierBP < 100:
		return errConfig("max multiplier must be at least 1.00x")
	case c.MinBetCents <= 0:
		return errConfig("minimum stake must be positive")
	case c.MaxBetCents < c.MinBetCents:
		return errConfig("maximum stake must be >= minimum stake")
	case c.TickInterval <= 0:
		return errConfig("tick interval must be positive")
	case c.ServiceFeeBP < 0 || c.ServiceFeeBP >= 10000:
		return errConfig("service fee must be in [0, 10000) basis points")
	case c.MinParticipants < 1:
		return errConfig("minimum participants must be at least 1")
	}
	return nil
}

type configError string

func (e configError) Error() string { return string(e) }

func errConfig(msg string) error { return configError("crash config: " + msg) }
