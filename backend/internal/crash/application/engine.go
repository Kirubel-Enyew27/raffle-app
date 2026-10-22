package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/crash/domain"
	realtime "github.com/raffle-app/backend/internal/realtime"
	"github.com/raffle-app/backend/pkg/money"
	"go.uber.org/zap"
)

// defaultLeadershipRetry is how long a passive instance waits between attempts
// to take the round clock, and how long a leaderless instance waits before
// retrying after a database error. The engine keeps it in a field so tests can
// shorten it.
const defaultLeadershipRetry = 3 * time.Second

// Publisher is the slice of the realtime service the round engine needs. It is
// an interface so the engine's event contract (which event carries which state)
// can be asserted in tests without a Redis connection.
type Publisher interface {
	Publish(ctx context.Context, eventType string, userID string, role string, payload interface{}) error
	PublishToChannel(ctx context.Context, channel string, eventType string, payload interface{}) error
}

// Engine is the single authority for crash rounds: it owns the state machine,
// the multiplier clock, the crash moment and settlement.
//
// Only one engine may run per database. It takes a PostgreSQL advisory lock
// before running the clock; while another instance holds it this instance
// stays passive and simply serves HTTP (round state is always read from the
// database, so a passive instance still answers every query correctly), and it
// keeps retrying so it can take over if the leader dies. That prevents two
// processes from running two competing clocks for the same round without
// turning the first failure into a permanently game-less deployment.
//
// Everything the engine does is derived from rows already in the database
// (round timeline + committed seed) and is idempotent, so a crash, restart or
// redeploy mid-round recovers on the next pass:
//   - a BETTING round whose deadline passed starts immediately, back-dated to
//     the deadline, so the multiplier is correct even if we were asleep;
//   - a RUNNING round whose crash instant passed crashes and settles at once;
//   - settlement replays harmlessly (lost-bet updates match zero rows and the
//     totals are recomputed from the bets table rather than incremented).
type Engine struct {
	rounds   domain.RoundRepository
	bets     domain.BetRepository
	source   domain.CrashPointSource
	service  *CrashService
	audit    *auditapp.AuditService
	realtime Publisher
	locker   domain.Locker
	clock    Clock
	cfg      domain.RoundConfig
	log      *zap.Logger

	// leadershipRetry is the delay between attempts to take the round clock.
	leadershipRetry time.Duration

	// autoCashout records when the round's auto cash-out targets were last
	// scanned. Auto cash-outs are paid at the player's configured target
	// multiplier, so noticing one a fraction of a second late never changes the
	// amount paid - which means the scan does not have to run on every tick. It
	// is only touched from the single Run goroutine.
	autoCashout autoCashoutSweep
}

// autoCashoutSweep records the last auto cash-out scan for a round, so the scan
// runs at most once per interval instead of on every engine tick (20 times a
// second at the default 50 ms tick).
type autoCashoutSweep struct {
	roundID string
	last    time.Time
}

// defaultAutoCashoutInterval is used when no broadcast interval is configured.
const defaultAutoCashoutInterval = 250 * time.Millisecond

func NewEngine(
	rounds domain.RoundRepository,
	bets domain.BetRepository,
	source domain.CrashPointSource,
	service *CrashService,
	locker domain.Locker,
	cfg domain.RoundConfig,
	log *zap.Logger,
) *Engine {
	return &Engine{
		rounds:          rounds,
		bets:            bets,
		source:          source,
		service:         service,
		locker:          locker,
		clock:           systemClock{},
		cfg:             cfg,
		log:             log,
		leadershipRetry: defaultLeadershipRetry,
	}
}

func (e *Engine) SetAuditService(audit *auditapp.AuditService) { e.audit = audit }

// SetRealtimeService enables live round events. A nil service keeps the engine
// silent: storing it in the interface field would make it compare non-nil.
func (e *Engine) SetRealtimeService(rt *realtime.Service) {
	if rt == nil {
		e.realtime = nil
		return
	}
	e.realtime = rt
}

// SetPublisher injects an alternative event publisher (tests only).
func (e *Engine) SetPublisher(p Publisher) { e.realtime = p }
func (e *Engine) SetClock(clock Clock)     { e.clock = clock }

// SetLeadershipRetry shortens the delay between leadership attempts (tests).
func (e *Engine) SetLeadershipRetry(d time.Duration) {
	if d > 0 {
		e.leadershipRetry = d
	}
}

// Run owns the round loop until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	if !e.cfg.Enabled {
		e.log.Info("crash engine disabled by configuration")
		return
	}

	release, ok := e.acquireLeadership(ctx)
	if !ok {
		e.log.Info("crash engine stopping before it could take the round clock")
		return
	}
	defer release()

	defer func() {
		if r := recover(); r != nil {
			e.log.Error("crash engine panicked", zap.Any("panic", r))
		}
	}()

	// Close out anything the previous process left mid-flight before starting
	// the clock: a crashed-but-unsettled round must not leave bets active, and
	// there may be a backlog of them after a restart or an outage.
	e.recoverUnfinished(ctx)

	ticker := time.NewTicker(e.cfg.TickInterval)
	defer ticker.Stop()

	var lastBroadcast time.Time
	for {
		select {
		case <-ctx.Done():
			e.log.Info("crash engine stopping")
			return
		case <-ticker.C:
		}

		// A panicking pass must not kill the game; the next tick recovers from
		// the persisted state.
		func() {
			defer func() {
				if r := recover(); r != nil {
					e.log.Error("crash engine pass panicked", zap.Any("panic", r))
				}
			}()
			lastBroadcast = e.step(ctx, lastBroadcast)
		}()
	}
}

// acquireLeadership blocks until this instance owns the engine lock or ctx is
// cancelled.
//
// Neither a lock held by a peer nor a transient database error is fatal: both
// are retried, so a passive instance takes over when the leader dies and a
// blip at boot no longer leaves the deployment without a round clock until the
// next restart. The first outcome of each kind is logged at info/warn level and
// repeats drop to debug so a busy standby does not flood the log.
func (e *Engine) acquireLeadership(ctx context.Context) (func(), bool) {
	if e.locker == nil {
		return func() {}, true
	}

	retry := e.leadershipRetry
	if retry <= 0 {
		retry = defaultLeadershipRetry
	}

	for attempt := 1; ; attempt++ {
		acquired, release, err := e.locker.Acquire(ctx, "crash_engine")
		switch {
		case err != nil:
			if attempt == 1 {
				e.log.Warn("crash engine could not reach its leadership lock; retrying",
					zap.Duration("retry_in", retry), zap.Error(err))
			} else {
				e.log.Debug("crash engine leadership attempt failed; retrying",
					zap.Int("attempt", attempt), zap.Error(err))
			}
		case !acquired:
			if attempt == 1 {
				e.log.Info("another instance owns the crash engine lock; waiting to take over",
					zap.Duration("retry_in", retry))
			} else {
				e.log.Debug("crash engine lock still held elsewhere", zap.Int("attempt", attempt))
			}
		default:
			e.log.Info("crash engine acquired leadership", zap.Int("attempt", attempt))
			return release, true
		}

		select {
		case <-ctx.Done():
			return func() {}, false
		case <-time.After(retry):
		}
	}
}

// recoverUnfinished settles and closes every round that is past its crash but
// never finished its lifecycle.
//
// Settlement is idempotent (lost-bet updates match only active rows and totals
// are recomputed from the bets table), so running this on every startup is safe
// even when a round was already partly settled by a previous process.
func (e *Engine) recoverUnfinished(ctx context.Context) {
	stale, err := e.rounds.FindByRoomAndState(ctx, e.cfg.Room,
		[]string{domain.StateCrashed, domain.StateSettlement}, 500)
	if err != nil {
		e.log.Error("crash engine could not scan for unfinished rounds", zap.Error(err))
		return
	}
	if len(stale) == 0 {
		return
	}

	e.log.Info("crash engine recovering unfinished rounds", zap.Int("count", len(stale)))
	for _, round := range stale {
		if round.State == domain.StateCrashed {
			if err := e.settle(ctx, round); err != nil {
				e.log.Error("crash engine failed to settle a recovered round",
					zap.Int64("round_number", round.RoundNumber), zap.Error(err))
				continue
			}
		}
		if err := e.transition(ctx, round.ID, func(tx *sql.Tx) (bool, error) {
			return e.rounds.MarkNextRoundTx(ctx, tx, round.ID)
		}); err != nil {
			e.log.Error("crash engine failed to close a recovered round",
				zap.Int64("round_number", round.RoundNumber), zap.Error(err))
		}
	}
}

// step advances the state machine once and reports the last broadcast time.
func (e *Engine) step(ctx context.Context, lastBroadcast time.Time) time.Time {
	// FindCurrent (not FindLive) so a crashed round is still settled and closed
	// before the next round is created.
	round, err := e.rounds.FindCurrent(ctx, e.cfg.Room)
	if err != nil {
		if err != domain.ErrNoLiveRound {
			e.log.Error("crash engine failed to load the current round", zap.Error(err))
			return lastBroadcast
		}
		e.log.Info("crash engine has no current round; starting one", zap.String("room", e.cfg.Room))
		if _, cerr := e.createRound(ctx); cerr != nil {
			e.log.Error("crash engine failed to create a round", zap.Error(cerr))
		}
		return lastBroadcast
	}

	switch round.State {
	case domain.StateWaiting:
		if !e.clock.Now().Before(round.BettingStartsAt) {
			if err := e.transition(ctx, round.ID, func(tx *sql.Tx) (bool, error) {
				return e.rounds.MarkBettingStartedTx(ctx, tx, round.ID)
			}); err != nil {
				e.log.Error("crash engine failed to open betting", zap.Error(err))
				return lastBroadcast
			}
			// Keep the local copy in step with the state just persisted so the
			// event advertises the state the round is actually in.
			round.State = domain.StateBetting
			e.publishRound(ctx, "crash.betting_open", round, map[string]interface{}{
				"betting_ends_at": round.BettingEndsAt,
			})
		}

	case domain.StateBetting:
		if !e.clock.Now().Before(round.BettingEndsAt) {
			// Start exactly at the advertised deadline rather than "now", so a
			// late tick (or a restart) cannot shift the multiplier curve.
			startedAt := round.BettingEndsAt
			if err := e.transition(ctx, round.ID, func(tx *sql.Tx) (bool, error) {
				return e.rounds.MarkRunningTx(ctx, tx, round.ID, startedAt)
			}); err != nil {
				e.log.Error("crash engine failed to start the round", zap.Error(err))
				return lastBroadcast
			}
			round.State = domain.StateRunning
			round.StartedAt = &startedAt
			e.publishRound(ctx, "crash.round_started", round, map[string]interface{}{
				"started_at": startedAt,
			})
			return e.clock.Now()
		}

	case domain.StateRunning:
		return e.driveRunning(ctx, round, lastBroadcast)

	case domain.StateCrashed:
		if round.CrashedAt == nil {
			e.log.Warn("crash round has no crash instant recorded; cannot settle",
				zap.Int64("round_number", round.RoundNumber))
			return lastBroadcast
		}
		// Settle only after the crash has been on screen for the configured
		// window. The tolerance absorbs timestamptz rounding (see
		// domain.StorageTolerance) so a zero-length window still settles.
		deadline := round.CrashedAt.Add(secondsToDuration(e.cfg.CrashedSeconds))
		if e.clock.Now().Add(domain.StorageTolerance).Before(deadline) {
			return lastBroadcast
		}
		if err := e.settle(ctx, round); err != nil {
			e.log.Error("crash engine failed to settle round", zap.Error(err))
		}
		return lastBroadcast

	case domain.StateSettlement:
		settledAt := round.SettledAt
		if settledAt == nil {
			t := e.clock.Now()
			settledAt = &t
		}
		deadline := settledAt.Add(secondsToDuration(e.cfg.NextRoundSeconds))
		if e.clock.Now().Add(domain.StorageTolerance).Before(deadline) {
			return lastBroadcast
		}
		if err := e.transition(ctx, round.ID, func(tx *sql.Tx) (bool, error) {
			return e.rounds.MarkNextRoundTx(ctx, tx, round.ID)
		}); err != nil {
			e.log.Error("crash engine failed to close the round", zap.Error(err))
		}
	}

	return lastBroadcast
}

// driveRunning advances a running round: auto cash-outs, broadcast ticks, and
// the crash itself.
func (e *Engine) driveRunning(ctx context.Context, round *domain.Round, lastBroadcast time.Time) time.Time {
	if round.StartedAt == nil {
		return lastBroadcast
	}

	crashBP := e.source.CrashPoint(round.ServerSeed, round.Nonce)
	now := e.clock.Now()
	crashAt := domain.CrashAt(*round.StartedAt, crashBP, e.cfg.GrowthRate)
	current := domain.MultiplierAt(now.Sub(*round.StartedAt).Seconds(), e.cfg.GrowthRate)

	// The round is over the moment the multiplier reaches the committed crash
	// point, or the crash instant (derived from the persisted start time) has
	// passed. Checking the clock as well as the multiplier matters because a
	// tick can arrive late, and the multiplier must never be shown or honoured
	// past the crash.
	if current >= crashBP || !now.Add(domain.StorageTolerance).Before(crashAt) {
		e.crash(ctx, round, crashBP, crashAt)
		return lastBroadcast
	}

	e.runAutoCashouts(ctx, round, current)

	if e.realtime != nil && now.Sub(lastBroadcast) >= e.cfg.BroadcastInterval {
		payload := map[string]interface{}{
			"round_id":      round.ID,
			"round_number":  round.RoundNumber,
			"multiplier":    domain.MultiplierValue(current),
			"multiplier_bp": current,
			"elapsed_ms":    now.Sub(*round.StartedAt).Milliseconds(),
			"server_time":   now,
		}
		if err := e.realtime.PublishToChannel(ctx, Channel, "crash.tick", payload); err != nil {
			e.log.Debug("crash tick publish failed", zap.Error(err))
		}
		return now
	}

	return lastBroadcast
}

// runAutoCashouts pays out every active bet whose auto cash-out target has been
// reached. Failures are logged and retried on the next tick; the payout itself
// is idempotent so a retry cannot double-pay.
func (e *Engine) runAutoCashouts(ctx context.Context, round *domain.Round, currentBP int) {
	now := e.clock.Now()
	interval := e.cfg.BroadcastInterval
	if interval <= 0 {
		interval = defaultAutoCashoutInterval
	}
	if e.autoCashout.roundID == round.ID && now.Sub(e.autoCashout.last) < interval {
		return
	}
	e.autoCashout = autoCashoutSweep{roundID: round.ID, last: now}

	bets, err := e.bets.FindActiveByRound(ctx, round.ID)
	if err != nil {
		e.log.Error("crash engine failed to load active bets", zap.Error(err))
		return
	}
	for _, bet := range bets {
		if bet.AutoCashoutBP <= 0 || currentBP < bet.AutoCashoutBP {
			continue
		}
		// The payout uses the bet's own target, not the observed multiplier, so a
		// late sweep can never pay more than the player asked for.
		if _, err := e.service.payOutBet(ctx, round, bet.UserID, bet.AutoCashoutBP, true); err != nil {
			e.log.Warn("crash engine auto cash-out failed",
				zap.String("bet_id", bet.ID), zap.Int("multiplier_bp", bet.AutoCashoutBP), zap.Error(err))
		}
	}
}

// crash transitions RUNNING -> CRASHED and publishes the outcome.
//
// crashAt is the instant the multiplier reached the committed crash point,
// derived from the round's persisted start time rather than from when this pass
// happened to notice. Recording the derived instant keeps the stored crash time
// consistent with the multiplier curve a client can recompute, and after a long
// outage it lets the round settle immediately instead of replaying the full
// result window.
func (e *Engine) crash(ctx context.Context, round *domain.Round, crashBP int, crashAt time.Time) {
	if crashAt.After(e.clock.Now()) {
		crashAt = e.clock.Now()
	}
	applied := false
	if err := e.transition(ctx, round.ID, func(tx *sql.Tx) (bool, error) {
		ok, err := e.rounds.MarkCrashedTx(ctx, tx, round.ID, crashBP, crashAt)
		applied = ok
		return ok, err
	}); err != nil {
		e.log.Error("crash engine failed to record the crash", zap.Error(err))
		return
	}
	if !applied {
		return
	}

	// Update the in-memory round before announcing the crash, so the event
	// advertises the state the round is actually in.
	round.State = domain.StateCrashed
	round.CrashedAt = &crashAt
	round.CrashMultiplierBP = crashBP

	e.publishRound(ctx, "crash.crashed", round, map[string]interface{}{
		"crash_multiplier":    domain.MultiplierValue(crashBP),
		"crash_multiplier_bp": crashBP,
		"crashed_at":          crashAt,
		"server_time":         e.clock.Now(),
		"server_seed":         round.ServerSeed,
		"server_seed_hash":    round.ServerSeedHash,
	})
	e.recordAudit(ctx, "crash_round_crashed", round.ID,
		fmt.Sprintf("crash round %d crashed at %.2fx", round.RoundNumber, domain.MultiplierValue(crashBP)))
}

// settle finalises a crashed round. For P2P rounds it runs the prize pool
// distribution before marking the round settled; for legacy rounds it uses the
// original payout approach.
//
// Idempotency contract: MarkAllLostTx matches only still-active rows, and
// MarkSettledP2PTx / MarkSettledTx write absolute totals. Re-running is safe.
func (e *Engine) settle(ctx context.Context, round *domain.Round) error {
	now := e.clock.Now()

	// 1. Mark every still-active bet as lost (idempotent).
	var lost int64
	if err := e.transition(ctx, round.ID, func(tx *sql.Tx) (bool, error) {
		n, err := e.bets.MarkAllLostTx(ctx, tx, round.ID, now)
		if err != nil {
			return false, fmt.Errorf("failed to mark losing bets: %w", err)
		}
		lost = n
		return true, nil
	}); err != nil {
		return fmt.Errorf("failed to mark losing bets: %w", err)
	}

	// 2. In P2P mode, distribute the prize pool among cashed-out players.
	var p2pResult *P2PSettlementResult
	if round.P2PMode {
		result, err := e.service.SettleP2PRound(ctx, round)
		if err != nil {
			return fmt.Errorf("P2P settlement failed for round %d: %w", round.RoundNumber, err)
		}
		p2pResult = result
	}

	// 3. Recompute authoritative totals from the bets table and move to SETTLEMENT.
	var totals *domain.RoundTotals
	applied := false

	if err := e.transition(ctx, round.ID, func(tx *sql.Tx) (bool, error) {
		t, err := e.bets.TotalsForRoundTx(ctx, tx, round.ID)
		if err != nil {
			return false, fmt.Errorf("failed to recompute round totals: %w", err)
		}
		totals = t

		if round.P2PMode && p2pResult != nil {
			ok, err := e.rounds.MarkSettledP2PTx(ctx, tx, round.ID,
				t.StakeCents, t.PayoutCents,
				int64(p2pResult.PrizePoolCents), int64(p2pResult.TotalFeeCents),
				now)
			applied = ok
			return ok, err
		}
		ok, err := e.rounds.MarkSettledTx(ctx, tx, round.ID, t.StakeCents, t.PayoutCents, now)
		applied = ok
		return ok, err
	}); err != nil {
		return fmt.Errorf("failed to settle round: %w", err)
	}
	if !applied {
		return nil // Already settled by another pass.
	}

	if lost > 0 {
		e.recordAudit(ctx, "crash_round_settled", round.ID,
			fmt.Sprintf("crash round %d settled: %d losing bets, stake %.2f, payout %.2f",
				round.RoundNumber, lost, money.Cents(totals.StakeCents).Float(), money.Cents(totals.PayoutCents).Float()))
	}

	if e.realtime != nil {
		payload := map[string]interface{}{
			"round_id":         round.ID,
			"round_number":     round.RoundNumber,
			"crash_multiplier": domain.MultiplierValue(round.CrashMultiplierBP),
			"total_stake":      money.Cents(totals.StakeCents).Float(),
			"total_payout":     money.Cents(totals.PayoutCents).Float(),
			"bet_count":        totals.BetCount,
			"cashout_count":    totals.CashoutCount,
			"settled_at":       now,
			"p2p":              round.P2PMode,
		}
		if p2pResult != nil {
			payload["prize_pool"] = p2pResult.PrizePoolCents.Float()
			payload["total_fee"] = p2pResult.TotalFeeCents.Float()
			payload["winner_count"] = p2pResult.WinnerCount
		}
		_ = e.realtime.PublishToChannel(ctx, Channel, "crash.settled", payload)
	}

	e.log.Info("crash round settled",
		zap.Int64("round_number", round.RoundNumber),
		zap.Int64("losing_bets", lost),
		zap.Int64("total_stake_cents", totals.StakeCents),
		zap.Int64("total_payout_cents", totals.PayoutCents),
		zap.Bool("p2p", round.P2PMode))
	return nil
}

// createRound opens the next round with a freshly committed seed. The seed hash
// is published here, before the betting window opens, which is what makes the
// later reveal meaningful. P2P configuration is snapshotted onto the round row
// so a settings change never affects an in-flight round.
func (e *Engine) createRound(ctx context.Context) (*domain.Round, error) {
	number, err := e.rounds.NextRoundNumber(ctx)
	if err != nil {
		return nil, err
	}

	commitment, err := e.source.Generate(1)
	if err != nil {
		return nil, err
	}

	now := e.clock.Now()
	round := &domain.Round{
		Room:                  e.cfg.Room,
		RoundNumber:           number,
		State:                 domain.StateWaiting,
		ServerSeed:            commitment.ServerSeed,
		ServerSeedHash:        commitment.ServerSeedHash,
		Nonce:                 commitment.Nonce,
		CommitmentPublishedAt: now,
		BettingStartsAt:       now.Add(secondsToDuration(e.cfg.WaitingSeconds)),
		BettingEndsAt:         now.Add(secondsToDuration(e.cfg.WaitingSeconds + e.cfg.BettingSeconds)),
		CreatedAt:             now,
		UpdatedAt:             now,
		// P2P snapshot: these fields are frozen from the live config so that
		// a settings change in the admin panel cannot retroactively alter an
		// in-flight round's fee or participant requirements.
		P2PMode:         true,
		ServiceFeeBP:    e.cfg.ServiceFeeBP,
		MinParticipants: e.cfg.MinParticipants,
	}

	if err := e.rounds.Create(ctx, round); err != nil {
		return nil, err
	}

	e.publishRound(ctx, "crash.round_created", round, map[string]interface{}{
		"betting_starts_at":       round.BettingStartsAt,
		"betting_ends_at":         round.BettingEndsAt,
		"server_seed_hash":        round.ServerSeedHash,
		"commitment_published_at": round.CommitmentPublishedAt,
		"growth_rate":             e.cfg.GrowthRate,
		"p2p_mode":                true,
		"service_fee_pct":         float64(e.cfg.ServiceFeeBP) / 100,
		"min_participants":        e.cfg.MinParticipants,
	})
	e.log.Info("crash round created",
		zap.Int64("round_number", round.RoundNumber),
		zap.String("round_id", round.ID),
		zap.Bool("p2p", true),
		zap.Int("service_fee_bp", e.cfg.ServiceFeeBP))
	return round, nil
}

// transition runs a guarded state change in its own transaction.
func (e *Engine) transition(ctx context.Context, roundID string, fn func(tx *sql.Tx) (bool, error)) error {
	tx, err := e.rounds.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	applied, err := fn(tx)
	if err != nil {
		return err
	}
	if !applied {
		return nil
	}
	return tx.Commit()
}

func (e *Engine) publishRound(ctx context.Context, eventType string, round *domain.Round, extra map[string]interface{}) {
	if e.realtime == nil {
		return
	}
	payload := map[string]interface{}{
		"round_id":     round.ID,
		"round_number": round.RoundNumber,
		"state":        round.State,
		"server_time":  e.clock.Now(),
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := e.realtime.PublishToChannel(ctx, Channel, eventType, payload); err != nil {
		e.log.Debug("crash publish failed", zap.String("event", eventType), zap.Error(err))
	}
}

func (e *Engine) recordAudit(ctx context.Context, action, resourceID, newValue string) {
	if e.audit == nil {
		return
	}
	actorType := "system"
	_ = e.audit.Record(ctx, nil, actorType, action, "game_round", &resourceID, "", nil, &newValue)
}
