package domain

import (
	"math"
	"testing"
	"time"

	"github.com/raffle-app/backend/pkg/money"
)

func TestStateMachineOrder(t *testing.T) {
	// The documented lifecycle must be exactly this sequence.
	want := []string{StateWaiting, StateBetting, StateRunning, StateCrashed, StateSettlement, StateNextRound}

	state := StateWaiting
	for i, expected := range want {
		if state != expected {
			t.Fatalf("step %d: state is %s, want %s", i, state, expected)
		}
		next, ok := NextState(state)
		if i < len(want)-1 {
			if !ok {
				t.Fatalf("transition from %s should be valid", state)
			}
			state = next
		} else if ok {
			t.Fatalf("NEXT_ROUND must be terminal, but it transitioned to %s", next)
		}
	}
}

func TestStatePredicates(t *testing.T) {
	if !IsLiveRoundState(StateWaiting) || !IsLiveRoundState(StateBetting) || !IsLiveRoundState(StateRunning) {
		t.Error("WAITING/BETTING/RUNNING are the live states")
	}
	if IsLiveRoundState(StateCrashed) || IsLiveRoundState(StateSettlement) || IsLiveRoundState(StateNextRound) {
		t.Error("post-crash states must not be live, or two rounds could move money at once")
	}
	if !IsFinishedRoundState(StateCrashed) || !IsFinishedRoundState(StateSettlement) || !IsFinishedRoundState(StateNextRound) {
		t.Error("post-crash states are finished states")
	}
	if IsFinishedRoundState(StateRunning) {
		t.Error("a running round is not finished")
	}
}

func TestBetStatusTerminal(t *testing.T) {
	if IsTerminalBetStatus(BetStatusActive) {
		t.Error("active must not be terminal")
	}
	if !IsTerminalBetStatus(BetStatusCashedOut) || !IsTerminalBetStatus(BetStatusLost) {
		t.Error("cashed_out and lost must be terminal")
	}
}

func TestBettingOpenWindow(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Second)
	round := &Round{State: StateBetting, BettingStartsAt: start, BettingEndsAt: end}

	if round.BettingOpen(start.Add(-time.Second)) {
		t.Error("betting must not be open before the window starts")
	}
	if !round.BettingOpen(start) {
		t.Error("betting must be open at the start of the window")
	}
	if !round.BettingOpen(end.Add(-time.Millisecond)) {
		t.Error("betting must be open until the deadline")
	}
	if round.BettingOpen(end) {
		t.Error("betting must be closed at the deadline")
	}

	round.State = StateRunning
	if round.BettingOpen(start.Add(time.Second)) {
		t.Error("betting must be closed once the round is running")
	}
}

func TestElapsedAndFairnessReveal(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	round := &Round{State: StateRunning, StartedAt: &start}

	if got := round.Elapsed(start.Add(3 * time.Second)); got != 3*time.Second {
		t.Errorf("elapsed = %v, want 3s", got)
	}

	crashed := start.Add(5 * time.Second)
	round.CrashedAt = &crashed
	// After a crash the elapsed time is frozen at the crash instant.
	if got := round.Elapsed(start.Add(30 * time.Second)); got != 5*time.Second {
		t.Errorf("elapsed after crash = %v, want 5s", got)
	}

	// The seed must stay sealed while the round can still move money.
	if round.FairnessRevealed() {
		t.Error("a running round must not reveal its seed")
	}
	round.State = StateCrashed
	round.CrashMultiplierBP = 250
	if !round.FairnessRevealed() {
		t.Error("a finished round should reveal its seed")
	}
	round.State = StateNextRound
	if !round.FairnessRevealed() {
		t.Error("a closed round should reveal its seed")
	}
	// A finished round without an outcome is not verifiable yet.
	round.CrashMultiplierBP = 0
	if round.FairnessRevealed() {
		t.Error("seed must not be revealed when no crash point was recorded")
	}
}

func TestPayoutForUsesIntegerMath(t *testing.T) {
	bet := &Bet{StakeCents: 3333}
	got := bet.PayoutFor(107)
	if got.String() != "35.66" {
		t.Errorf("payout = %s, want 35.66 (floor of 35.6631)", got)
	}
}

func TestMultiplierAtCurve(t *testing.T) {
	const growth = 0.07

	if got := MultiplierAt(0, growth); got != 100 {
		t.Errorf("at t=0 multiplier = %d, want 100", got)
	}
	if got := MultiplierAt(-5, growth); got != 100 {
		t.Errorf("negative elapsed must floor at 1.00x, got %d", got)
	}

	// e^(0.07 * 10) = 2.0138 -> 2.01x
	if got := MultiplierAt(10, growth); got != 201 {
		t.Errorf("at t=10s multiplier = %d, want 201", got)
	}
}

func TestMultiplierIsMonotonic(t *testing.T) {
	prev := 0
	for elapsed := 0.0; elapsed < 120; elapsed += 0.05 {
		got := MultiplierAt(elapsed, 0.07)
		if got < prev {
			t.Fatalf("multiplier decreased at t=%.2f: %d < %d", elapsed, got, prev)
		}
		prev = got
	}
}

func TestDurationForMultiplierInvertsCurve(t *testing.T) {
	const growth = 0.07
	for _, bp := range []int{100, 130, 200, 500, 1000, 100000} {
		seconds := DurationForMultiplier(bp, growth)
		// The curve at that instant should be within one tick of the target.
		got := MultiplierAt(seconds, growth)
		if got != bp && got != bp-1 {
			t.Errorf("DurationForMultiplier(%d) -> t=%.4f gives %d", bp, seconds, got)
		}
	}
	if DurationForMultiplier(100, growth) != 0 {
		t.Error("1.00x happens instantly")
	}
	if DurationForMultiplier(500, 0) != 0 {
		t.Error("a non-positive growth rate must not divide by zero")
	}
}

func TestMultiplierValueAndParsing(t *testing.T) {
	if got := MultiplierValue(247); math.Abs(got-2.47) > 1e-9 {
		t.Errorf("MultiplierValue(247) = %v, want 2.47", got)
	}
	cases := []struct {
		in   float64
		want int
	}{
		{1.0, 100},
		{1.3, 130},
		{2.47, 247},
		{1.005, 100}, // rounds to two decimals, then truncates
		{0.5, 0},     // below 1.00x is rejected
		{math.NaN(), 0},
		{math.Inf(1), 0},
	}
	for _, c := range cases {
		if got := MultiplierToBasisPoints(c.in); got != c.want {
			t.Errorf("MultiplierToBasisPoints(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDisplayNameMasking(t *testing.T) {
	cases := []struct {
		name, email, want string
	}{
		{"Samuel Tesfaye", "s@example.com", "Samuel Tesfaye"},
		{"", "kirubel@example.com", "ki***@example.com"},
		{"", "ab@example.com", "a***@example.com"},
		{"", "x@example.com", "x***@example.com"},
		{"", "", "Player"},
		{"  ", "  ", "Player"},
		{"", "nodomain", "no***"},
	}
	for _, c := range cases {
		if got := DisplayName(c.name, c.email); got != c.want {
			t.Errorf("DisplayName(%q,%q) = %q, want %q", c.name, c.email, got, c.want)
		}
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("default config must be valid: %v", err)
	}
}

func TestConfigValidationRejectsBadValues(t *testing.T) {
	base := func() RoundConfig { return DefaultConfig() }

	cases := map[string]func(c *RoundConfig){
		"empty room":          func(c *RoundConfig) { c.Room = "" },
		"zero betting window": func(c *RoundConfig) { c.BettingSeconds = 0 },
		"negative waiting":    func(c *RoundConfig) { c.WaitingSeconds = -1 },
		"zero growth":         func(c *RoundConfig) { c.GrowthRate = 0 },
		"edge too high":       func(c *RoundConfig) { c.HouseEdgeBP = 10000 },
		"edge negative":       func(c *RoundConfig) { c.HouseEdgeBP = -1 },
		"max below 1x":        func(c *RoundConfig) { c.MaxMultiplierBP = 50 },
		"zero min bet":        func(c *RoundConfig) { c.MinBetCents = 0 },
		"inverted limits": func(c *RoundConfig) {
			c.MinBetCents = 500
			c.MaxBetCents = 100
		},
		"zero tick": func(c *RoundConfig) { c.TickInterval = 0 },
	}

	for name, mutate := range cases {
		cfg := base()
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

// ---------------------------------------------------------------------------
// P2P prize distribution and fee calculation
// ---------------------------------------------------------------------------

func TestP2PGrossReturnProportional(t *testing.T) {
	// Two winners: Alice cashed at 3.00x, Bob at 1.00x.
	// Prize pool = 1000. Total BP weight = 300 + 100 = 400.
	// Alice gross = 1000 * 300 / 400 = 750.
	// Bob gross   = 1000 * 100 / 400 = 250.
	pool := money.Cents(100000) // 1000.00
	aliceGross := P2PGrossReturn(pool, 300, 400)
	bobGross := P2PGrossReturn(pool, 100, 400)

	if aliceGross != 75000 {
		t.Errorf("Alice gross = %d, want 75000", aliceGross)
	}
	if bobGross != 25000 {
		t.Errorf("Bob gross = %d, want 25000", bobGross)
	}
	// Total payouts must not exceed prize pool.
	if aliceGross+bobGross > pool {
		t.Errorf("total payouts %d exceed pool %d", aliceGross+bobGross, pool)
	}
}

func TestP2PGrossReturnSingleWinner(t *testing.T) {
	// Single winner gets the full prize pool.
	pool := money.Cents(100000)
	gross := P2PGrossReturn(pool, 250, 250)
	if gross != pool {
		t.Errorf("single winner gross = %d, want %d", gross, pool)
	}
}

func TestP2PGrossReturnZeroWeight(t *testing.T) {
	pool := money.Cents(100000)
	if got := P2PGrossReturn(pool, 0, 400); got != 0 {
		t.Errorf("zero cashout BP should yield 0, got %d", got)
	}
	if got := P2PGrossReturn(pool, 300, 0); got != 0 {
		t.Errorf("zero total BP should yield 0, got %d", got)
	}
}

func TestP2PServiceFeeOnProfit(t *testing.T) {
	// Example from the spec:
	// gross_return = 1000, stake = 100, gross_profit = 900.
	// fee 5% of 900 = 45. net = 955.
	stake := money.Cents(10000)  // 100.00
	gross := money.Cents(100000) // 1000.00
	profit := gross - stake      // 900.00
	fee := P2PServiceFee(profit, 500)
	net := gross - fee

	if fee != 4500 {
		t.Errorf("fee = %d, want 4500 (45.00)", fee)
	}
	if net != 95500 {
		t.Errorf("net = %d, want 95500 (955.00)", net)
	}
}

func TestP2PServiceFeeZeroWhenNoProfit(t *testing.T) {
	// Player cashed out at exactly their stake: no profit, no fee.
	stake := money.Cents(10000)
	gross := stake // break-even
	profit := gross - stake
	if got := P2PServiceFee(profit, 500); got != 0 {
		t.Errorf("fee on zero profit = %d, want 0", got)
	}

	// Player's gross return is less than their stake (rare in practice but
	// possible when many cashed out and the pool is split thin).
	profit = money.Cents(-100)
	if got := P2PServiceFee(profit, 500); got != 0 {
		t.Errorf("fee on negative profit = %d, want 0", got)
	}
}

func TestP2PServiceFeeZeroWhenFeeBPZero(t *testing.T) {
	profit := money.Cents(90000)
	if got := P2PServiceFee(profit, 0); got != 0 {
		t.Errorf("fee with 0 bp = %d, want 0", got)
	}
}

func TestP2PNetPayoutNeverExceedsGross(t *testing.T) {
	pool := money.Cents(50000)
	stake := money.Cents(10000)
	gross := P2PGrossReturn(pool, 200, 200)
	profit := gross - stake
	if profit < 0 {
		profit = 0
	}
	fee := P2PServiceFee(profit, 500)
	net := gross - fee
	if net > gross {
		t.Errorf("net %d > gross %d", net, gross)
	}
	if net < 0 {
		t.Errorf("net %d is negative", net)
	}
}

func TestP2PMultipleWinnersPayoutsWithinPool(t *testing.T) {
	pool := money.Cents(300000) // 3000.00
	// Five players cashed out at different multipliers.
	cashouts := []int{150, 200, 250, 300, 100} // BP
	totalBP := 0
	for _, bp := range cashouts {
		totalBP += bp
	}

	var totalPayout money.Cents
	for _, bp := range cashouts {
		gross := P2PGrossReturn(pool, bp, totalBP)
		totalPayout += gross
	}

	// Integer division truncation means total may be up to len(cashouts) cents
	// below the pool (each bet loses at most 1 cent of sub-cent remainder).
	if totalPayout > pool {
		t.Errorf("total payout %d exceeds pool %d", totalPayout, pool)
	}
	maxRemainder := money.Cents(len(cashouts))
	if pool-totalPayout > maxRemainder {
		t.Errorf("too much remainder: pool=%d payout=%d diff=%d", pool, totalPayout, pool-totalPayout)
	}
}
