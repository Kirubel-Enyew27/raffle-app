package application

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/crash/domain"
	realtime "github.com/raffle-app/backend/internal/realtime"
	appcontext "github.com/raffle-app/backend/pkg/context"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/money"
)

// Channel is the realtime channel the crash room publishes to. Only clients
// that explicitly subscribe to it receive live game traffic.
const Channel = "crash"

// defaultCurrency is used for display when the player has no wallet yet.
const defaultCurrency = "ETB"

// Ledger transaction types written into the existing wallet_transactions table.
const (
	TxTypeBet         = "crash_bet"
	TxTypePayout      = "crash_payout"
	TxTypePlatformFee = "crash_platform_fee"
	TxTypeRefund      = "crash_refund"
)

// IdempotencyStore is the existing Redis-backed idempotency helper.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
}

// Clock is injected so tests can drive timing deterministically.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// CrashService owns every player-initiated money movement in the crash game.
// The round engine (engine.go) only drives the clock and settles rounds.
type CrashService struct {
	rounds      domain.RoundRepository
	bets        domain.BetRepository
	wallets     domain.WalletRepository
	players     domain.PlayerRepository
	settings    domain.SettingsRepository
	source      domain.CrashPointSource
	audit       *auditapp.AuditService
	realtime    *realtime.Service
	idempotency IdempotencyStore
	clock       Clock
	cfg         domain.RoundConfig
}

func NewCrashService(
	rounds domain.RoundRepository,
	bets domain.BetRepository,
	wallets domain.WalletRepository,
	source domain.CrashPointSource,
	cfg domain.RoundConfig,
) *CrashService {
	if cfg.Room == "" {
		cfg.Room = "default"
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = domain.DefaultConfig().TickInterval
	}
	return &CrashService{
		rounds:  rounds,
		bets:    bets,
		wallets: wallets,
		source:  source,
		clock:   systemClock{},
		cfg:     cfg,
	}
}

// SetPlayerRepository enables display names in the live multiplayer table.
func (s *CrashService) SetPlayerRepository(repo domain.PlayerRepository) { s.players = repo }

// SetSettingsRepository enables reading/writing admin-configurable settings.
func (s *CrashService) SetSettingsRepository(repo domain.SettingsRepository) { s.settings = repo }

// SetAuditService enables audit records for game money movements.
func (s *CrashService) SetAuditService(audit *auditapp.AuditService) { s.audit = audit }

// SetRealtimeService enables live round/player events over SSE.
func (s *CrashService) SetRealtimeService(rt *realtime.Service) { s.realtime = rt }

// SetIdempotencyStore enables replay protection for bet placement.
func (s *CrashService) SetIdempotencyStore(store IdempotencyStore) { s.idempotency = store }

// SetClock overrides the clock (tests only).
func (s *CrashService) SetClock(clock Clock) { s.clock = clock }

// Config exposes the round configuration to the API layer and the engine.
func (s *CrashService) Config() domain.RoundConfig { return s.cfg }

// ---------------------------------------------------------------------------
// Placing a bet
// ---------------------------------------------------------------------------

// PlaceBet debits the stake and records the bet in a single transaction.
func (s *CrashService) PlaceBet(ctx context.Context, userID string, amountCents money.Cents, autoCashoutBP int, idempotencyKey string) (*BetView, error) {
	bet, err := s.placeBet(ctx, userID, amountCents, autoCashoutBP, idempotencyKey)
	if err != nil {
		return nil, err
	}
	view := s.betView(ctx, bet, s.displayName(ctx, userID))
	return &view, nil
}

func (s *CrashService) placeBet(ctx context.Context, userID string, amountCents money.Cents, autoCashoutBP int, idempotencyKey string) (*domain.Bet, error) {
	if userID == "" {
		return nil, apperrors.ErrUnauthorized
	}
	if !amountCents.Valid() {
		return nil, apperrors.WithField("INVALID_AMOUNT", "stake must be a positive amount with at most two decimals", 400, nil)
	}
	if amountCents < s.cfg.MinBetCents {
		return nil, apperrors.WithField("BET_TOO_SMALL", fmt.Sprintf("minimum stake is %s", s.cfg.MinBetCents), 400, nil)
	}
	if amountCents > s.cfg.MaxBetCents {
		return nil, apperrors.WithField("BET_TOO_LARGE", fmt.Sprintf("maximum stake is %s", s.cfg.MaxBetCents), 400, nil)
	}
	if autoCashoutBP != 0 {
		if autoCashoutBP < domain.MinMultiplierBP || autoCashoutBP > s.cfg.MaxMultiplierBP {
			return nil, apperrors.WithField("INVALID_AUTO_CASHOUT", "auto cash-out multiplier is out of range", 400, nil)
		}
	}

	// Replay protection: an identical request returns the original bet.
	cacheKey := ""
	if s.idempotency != nil && idempotencyKey != "" {
		cacheKey = "crash_bet:" + userID + ":" + idempotencyKey
		cached, err := s.idempotency.Get(ctx, cacheKey)
		if err != nil {
			return nil, apperrors.WithField("IDEMPOTENCY_ERROR", "failed to check idempotency key", 500, err)
		}
		if cached != "" {
			var bet domain.Bet
			if err := json.Unmarshal([]byte(cached), &bet); err == nil {
				return &bet, nil
			}
		}
	}

	tx, err := s.bets.BeginTx(ctx)
	if err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to begin transaction", 500, err)
	}
	defer tx.Rollback()

	round, err := s.rounds.FindLiveForUpdate(ctx, tx, s.cfg.Room)
	if err != nil {
		if err == domain.ErrNoLiveRound {
			return nil, apperrors.WithField("NO_ROUND", "no crash round is open for betting", 409, nil)
		}
		return nil, apperrors.WithField("DB_ERROR", "failed to load crash round", 500, err)
	}

	now := s.clock.Now()
	if round.State != domain.StateBetting {
		return nil, apperrors.WithField("BETTING_CLOSED", "betting is not open for this round", 409, nil)
	}
	if !now.Before(round.BettingEndsAt) {
		return nil, apperrors.WithField("BETTING_CLOSED", "the betting window for this round has closed", 409, nil)
	}
	if now.Before(round.BettingStartsAt) {
		return nil, apperrors.WithField("BETTING_NOT_OPEN", "betting has not opened yet", 409, nil)
	}

	// Enforce max participants for P2P rounds.
	if round.P2PMode && s.cfg.MaxParticipants > 0 && round.BetCount >= s.cfg.MaxParticipants {
		return nil, apperrors.WithField("ROOM_FULL", "this round has reached its maximum number of participants", 409, nil)
	}

	wallet, err := s.wallets.EnsureWalletTx(ctx, tx, userID)
	if err != nil {
		return nil, apperrors.WithField("WALLET_ERROR", "failed to load wallet", 500, err)
	}

	balance := money.FromFloat(wallet.Balance)
	if balance < amountCents {
		return nil, apperrors.ErrInsufficientFunds
	}
	newBalance := balance - amountCents

	bet := &domain.Bet{
		ID:            newID(),
		RoundID:       round.ID,
		UserID:        userID,
		WalletID:      wallet.ID,
		StakeCents:    amountCents,
		StakeAmount:   amountCents.Float(),
		Status:        domain.BetStatusActive,
		AutoCashoutBP: autoCashoutBP,
		DebitTxID:     newID(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	walletTx := &domain.WalletTransaction{
		ID:            bet.DebitTxID,
		WalletID:      wallet.ID,
		UserID:        userID,
		Type:          TxTypeBet,
		Status:        "completed",
		Amount:        amountCents.Float(),
		BalanceBefore: balance.Float(),
		BalanceAfter:  newBalance.Float(),
		Reference:     "crash_bet:" + bet.ID,
		Description:   fmt.Sprintf("Crash game stake (round %d)", round.RoundNumber),
		Metadata: map[string]interface{}{
			"round_id":     round.ID,
			"round_number": round.RoundNumber,
			"game":         "crash",
			"p2p":          round.P2PMode,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.wallets.CreateTransactionTx(ctx, tx, walletTx); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to record wallet transaction", 500, err)
	}
	if err := s.bets.CreateTx(ctx, tx, bet); err != nil {
		if err == domain.ErrDuplicateBet {
			return nil, apperrors.WithField("ALREADY_BET", "you already placed a bet in this round", 409, nil)
		}
		return nil, apperrors.WithField("DB_ERROR", "failed to record bet", 500, err)
	}
	if err := s.wallets.UpdateBalanceTx(ctx, tx, wallet.ID, newBalance.Float()); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to update wallet balance", 500, err)
	}
	if err := s.rounds.AdjustTotalsTx(ctx, tx, round.ID, int64(amountCents), 0, 1, 0); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to update round totals", 500, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to commit bet", 500, err)
	}

	if cacheKey != "" {
		if data, err := json.Marshal(bet); err == nil {
			_ = s.idempotency.Set(ctx, cacheKey, string(data))
		}
	}

	s.recordBetAudit(ctx, userID, "crash_bet_placed", bet, fmt.Sprintf("placed %.2f stake on crash round %d", bet.StakeAmount, round.RoundNumber))

	displayName := s.displayName(ctx, userID)
	if s.realtime != nil {
		newPrizePool := money.Cents(int64(round.PrizePoolCents) + int64(amountCents))
		_ = s.realtime.Publish(ctx, "wallet_update", userID, "", map[string]interface{}{
			"balance":  newBalance.Float(),
			"currency": wallet.Currency,
		})
		_ = s.realtime.Publish(ctx, "crash.bet_accepted", userID, "", s.betView(ctx, bet, displayName))
		_ = s.realtime.PublishToChannel(ctx, Channel, "crash.bet_placed", map[string]interface{}{
			"round_id":        round.ID,
			"round_number":    round.RoundNumber,
			"user_id":         userID,
			"display_name":    displayName,
			"stake":           bet.StakeAmount,
			"bet_count":       round.BetCount + 1,
			"total_stake":     money.Cents(int64(round.TotalStakeCents) + int64(amountCents)).Float(),
			"prize_pool":      newPrizePool.Float(),
			"service_fee_pct": float64(round.ServiceFeeBP) / 100,
		})
	}

	return bet, nil
}

// ---------------------------------------------------------------------------
// Cashing out
// ---------------------------------------------------------------------------

// CashOut records the player's cash-out multiplier. In P2P mode the actual
// wallet credit is deferred until settlement; this call only flips the bet to
// cashed_out and records the multiplier weight.
func (s *CrashService) CashOut(ctx context.Context, userID string) (*BetView, error) {
	bet, err := s.cashOut(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := s.betView(ctx, bet, s.displayName(ctx, userID))
	return &view, nil
}

func (s *CrashService) cashOut(ctx context.Context, userID string) (*domain.Bet, error) {
	if userID == "" {
		return nil, apperrors.ErrUnauthorized
	}

	round, err := s.rounds.FindLive(ctx, s.cfg.Room)
	if err != nil {
		if err == domain.ErrNoLiveRound {
			return nil, apperrors.WithField("NO_ROUND", "no crash round is running", 409, nil)
		}
		return nil, apperrors.WithField("DB_ERROR", "failed to load crash round", 500, err)
	}
	if round.State != domain.StateRunning {
		return nil, apperrors.WithField("ROUND_NOT_RUNNING", "the round is not running", 409, nil)
	}

	crashBP := s.source.CrashPoint(round.ServerSeed, round.Nonce)
	now := s.clock.Now()

	if round.StartedAt == nil {
		return nil, apperrors.WithField("ROUND_NOT_RUNNING", "the round has not started", 409, nil)
	}
	crashAt := domain.CrashAt(*round.StartedAt, crashBP, s.cfg.GrowthRate)
	if !now.Add(domain.StorageTolerance).Before(crashAt) {
		return nil, apperrors.WithField("ROUND_CRASHED", "the round has already crashed", 409, nil)
	}

	multiplierBP := domain.MultiplierAt(now.Sub(*round.StartedAt).Seconds(), s.cfg.GrowthRate)
	if multiplierBP >= crashBP {
		return nil, apperrors.WithField("ROUND_CRASHED", "the round has already crashed", 409, nil)
	}

	if round.P2PMode {
		// In P2P mode we record the cash-out multiplier now; the wallet credit
		// happens at settlement after the round crashes.
		return s.recordP2PCashOut(ctx, round, userID, multiplierBP)
	}
	// Legacy path: immediate payout.
	return s.payOutBet(ctx, round, userID, multiplierBP, false)
}

// recordP2PCashOut marks the bet as cashed_out with multiplierBP and notifies
// other clients. No money moves yet; settlement does that.
func (s *CrashService) recordP2PCashOut(ctx context.Context, round *domain.Round, userID string, multiplierBP int) (*domain.Bet, error) {
	now := s.clock.Now()

	tx, err := s.bets.BeginTx(ctx)
	if err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to begin transaction", 500, err)
	}
	defer tx.Rollback()

	bet, err := s.bets.FindByRoundAndUserTx(ctx, tx, round.ID, userID)
	if err != nil {
		if err == apperrors.ErrNotFound {
			return nil, apperrors.WithField("NO_BET", "you have no bet in this round", 404, nil)
		}
		return nil, apperrors.WithField("DB_ERROR", "failed to load bet", 500, err)
	}
	if !bet.IsActive() {
		if bet.Status == domain.BetStatusCashedOut {
			return bet, nil
		}
		return bet, apperrors.WithField("BET_LOST", "the bet was already lost", 409, nil)
	}

	// In P2P mode payout_cents stays 0 until settlement; we pass 0 here.
	applied, err := s.bets.MarkCashedOutTx(ctx, tx, bet.ID, multiplierBP, 0, "", now)
	if err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to record cash-out", 500, err)
	}
	if !applied {
		_ = tx.Rollback()
		settled, ferr := s.bets.FindByRoundAndUser(ctx, round.ID, userID)
		if ferr != nil {
			return nil, apperrors.WithField("DB_ERROR", "failed to reload bet", 500, ferr)
		}
		return settled, nil
	}

	if err := s.rounds.AdjustTotalsTx(ctx, tx, round.ID, 0, 0, 0, 1); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to update cashout count", 500, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to commit cash-out", 500, err)
	}

	bet.CashoutMultiplierBP = multiplierBP
	bet.Status = domain.BetStatusCashedOut
	bet.CashedOutAt = &now

	s.recordBetAudit(ctx, userID, "crash_cashout_recorded", bet,
		fmt.Sprintf("P2P cash-out recorded at %.2fx (payout deferred to settlement, round %d)", domain.MultiplierValue(multiplierBP), round.RoundNumber))

	displayName := s.displayName(ctx, userID)
	if s.realtime != nil {
		_ = s.realtime.Publish(ctx, "crash.cashout_confirmed", userID, "", s.betView(ctx, bet, displayName))
		_ = s.realtime.PublishToChannel(ctx, Channel, "crash.bet_cashed_out", map[string]interface{}{
			"round_id":     round.ID,
			"round_number": round.RoundNumber,
			"user_id":      userID,
			"display_name": displayName,
			"stake":        bet.StakeAmount,
			"multiplier":   domain.MultiplierValue(multiplierBP),
			"payout":       nil, // deferred
			"p2p":          true,
		})
	}

	return bet, nil
}

// payOutBet performs the single, atomic payout of one bet (legacy / non-P2P path).
func (s *CrashService) payOutBet(ctx context.Context, round *domain.Round, userID string, multiplierBP int, auto bool) (*domain.Bet, error) {
	now := s.clock.Now()

	tx, err := s.bets.BeginTx(ctx)
	if err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to begin transaction", 500, err)
	}
	defer tx.Rollback()

	bet, err := s.bets.FindByRoundAndUserTx(ctx, tx, round.ID, userID)
	if err != nil {
		if err == apperrors.ErrNotFound {
			return nil, apperrors.WithField("NO_BET", "you have no bet in this round", 404, nil)
		}
		return nil, apperrors.WithField("DB_ERROR", "failed to load bet", 500, err)
	}

	if !bet.IsActive() {
		if bet.Status == domain.BetStatusCashedOut {
			return bet, nil
		}
		return bet, apperrors.WithField("BET_LOST", "the bet was already lost", 409, nil)
	}

	payoutCents := bet.PayoutFor(multiplierBP)
	creditTxID := newID()

	wallet, err := s.wallets.EnsureWalletTx(ctx, tx, userID)
	if err != nil {
		return nil, apperrors.WithField("WALLET_ERROR", "failed to load wallet", 500, err)
	}
	balanceBefore := money.FromFloat(wallet.Balance)
	newBalance := balanceBefore + payoutCents

	walletTx := &domain.WalletTransaction{
		ID:            creditTxID,
		WalletID:      wallet.ID,
		UserID:        userID,
		Type:          TxTypePayout,
		Status:        "completed",
		Amount:        payoutCents.Float(),
		BalanceBefore: balanceBefore.Float(),
		BalanceAfter:  newBalance.Float(),
		Reference:     "crash_payout:" + bet.ID,
		Description:   fmt.Sprintf("Crash game win at %.2fx (round %d)", domain.MultiplierValue(multiplierBP), round.RoundNumber),
		Metadata: map[string]interface{}{
			"round_id":     round.ID,
			"round_number": round.RoundNumber,
			"game":         "crash",
			"multiplier":   domain.MultiplierValue(multiplierBP),
			"stake":        bet.StakeAmount,
			"auto":         auto,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.wallets.CreateTransactionTx(ctx, tx, walletTx); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to record payout", 500, err)
	}

	applied, err := s.bets.MarkCashedOutTx(ctx, tx, bet.ID, multiplierBP, int64(payoutCents), creditTxID, now)
	if err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to settle bet", 500, err)
	}
	if !applied {
		_ = tx.Rollback()
		settled, ferr := s.bets.FindByRoundAndUser(ctx, round.ID, userID)
		if ferr != nil {
			return nil, apperrors.WithField("DB_ERROR", "failed to reload bet", 500, ferr)
		}
		return settled, nil
	}

	if err := s.wallets.UpdateBalanceTx(ctx, tx, wallet.ID, newBalance.Float()); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to update wallet balance", 500, err)
	}
	if err := s.rounds.AdjustTotalsTx(ctx, tx, round.ID, 0, int64(payoutCents), 0, 1); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to update round totals", 500, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to commit cash-out", 500, err)
	}

	bet.CashoutMultiplierBP = multiplierBP
	bet.PayoutCents = payoutCents
	bet.PayoutAmount = payoutCents.Float()
	bet.Status = domain.BetStatusCashedOut
	bet.CreditTxID = creditTxID
	bet.CashedOutAt = &now
	bet.SettledAt = &now

	s.recordBetAudit(ctx, userID, "crash_cashout", bet,
		fmt.Sprintf("cashed out crash bet at %.2fx for %.2f (auto=%t)", domain.MultiplierValue(multiplierBP), bet.PayoutAmount, auto))

	displayName := s.displayName(ctx, userID)
	if s.realtime != nil {
		_ = s.realtime.Publish(ctx, "wallet_update", userID, "", map[string]interface{}{
			"balance":  newBalance.Float(),
			"currency": wallet.Currency,
		})
		_ = s.realtime.Publish(ctx, "crash.cashout_confirmed", userID, "", s.betView(ctx, bet, displayName))
		_ = s.realtime.PublishToChannel(ctx, Channel, "crash.bet_cashed_out", map[string]interface{}{
			"round_id":     round.ID,
			"round_number": round.RoundNumber,
			"user_id":      userID,
			"display_name": displayName,
			"stake":        bet.StakeAmount,
			"multiplier":   domain.MultiplierValue(multiplierBP),
			"payout":       bet.PayoutAmount,
			"auto":         auto,
		})
	}

	return bet, nil
}

// ---------------------------------------------------------------------------
// P2P Settlement
// ---------------------------------------------------------------------------

// SettleP2PRound distributes the prize pool among winners and deducts the
// service fee from each winner's profit. Called by the engine after the crash.
//
// Winner selection rules (documented):
//   - Winners are players whose bet status is "cashed_out".
//   - Each winner's gross return is proportional to their cash-out multiplier
//     weight: gross_return = prize_pool * cashout_bp / total_cashout_bp_sum
//   - Service fee = max(0, gross_return - stake) * service_fee_bp / 10000
//   - Net payout = gross_return - service_fee
//   - If no one cashed out: the full prize pool is credited to the platform
//     revenue account as a "crash_platform_fee" ledger entry, with no winner.
//   - Tie handling: all cashed-out bets are proportional by multiplier weight.
//     A last-survivor tie (two players cash out at the exact same multiplierBP)
//     splits that weight equally (because their bp values are equal, the ratio
//     is equal, giving each an equal fractional share of the pool).
//
// The entire settlement is idempotent: SettleP2PWinnerTx only writes when
// net_payout_cents = 0 (unset). Re-running the same round settlement matches
// zero rows and produces no duplicate credits.
func (s *CrashService) SettleP2PRound(ctx context.Context, round *domain.Round) (*P2PSettlementResult, error) {
	now := s.clock.Now()
	prizePool := round.PrizePoolCents // total of all accepted stakes

	winners, err := s.bets.FindCashedOutByRound(ctx, round.ID)
	if err != nil {
		return nil, fmt.Errorf("P2P settle: failed to load cashed-out bets: %w", err)
	}

	// Compute the total weight (sum of cashout multiplier basis points).
	var totalBP int
	for _, w := range winners {
		totalBP += w.CashoutMultiplierBP
	}

	result := &P2PSettlementResult{
		RoundID:        round.ID,
		PrizePoolCents: prizePool,
		ServiceFeeBP:   round.ServiceFeeBP,
		WinnerCount:    len(winners),
	}

	if len(winners) == 0 || totalBP == 0 {
		// No winners: full prize pool becomes platform revenue.
		if err := s.creditPlatformRevenue(ctx, round, prizePool, 0, 0, now, "no_winners"); err != nil {
			return nil, fmt.Errorf("P2P settle: failed to credit platform on no-winner round: %w", err)
		}
		result.PlatformRevenueCents = prizePool
		return result, nil
	}

	// Settle each winner.
	var totalNetPayout money.Cents
	var totalFee money.Cents
	var settledWinners []P2PWinnerResult

	for _, bet := range winners {
		// Proportional gross return.
		gross := domain.P2PGrossReturn(prizePool, bet.CashoutMultiplierBP, totalBP)
		profit := gross - bet.StakeCents
		if profit < 0 {
			profit = 0
		}
		fee := domain.P2PServiceFee(profit, round.ServiceFeeBP)
		net := gross - fee

		settled, err := s.settleSingleP2PWinner(ctx, round, bet, gross, profit, fee, net, now)
		if err != nil {
			return nil, fmt.Errorf("P2P settle: failed to settle winner %s: %w", bet.UserID, err)
		}

		totalNetPayout += net
		totalFee += fee
		settledWinners = append(settledWinners, *settled)
	}

	result.TotalFeeCents = totalFee
	result.TotalNetPayoutCents = totalNetPayout
	result.Winners = settledWinners

	// Publish settlement summary.
	if s.realtime != nil {
		winnerList := make([]map[string]interface{}, 0, len(settledWinners))
		for _, w := range settledWinners {
			winnerList = append(winnerList, map[string]interface{}{
				"user_id":            w.UserID,
				"display_name":       w.DisplayName,
				"stake":              w.StakeCents.Float(),
				"gross_return":       w.GrossPayoutCents.Float(),
				"gross_profit":       w.GrossProfitCents.Float(),
				"service_fee":        w.ServiceFeeCents.Float(),
				"service_fee_pct":    float64(round.ServiceFeeBP) / 100,
				"net_payout":         w.NetPayoutCents.Float(),
				"cashout_multiplier": domain.MultiplierValue(w.CashoutBP),
			})
		}
		_ = s.realtime.PublishToChannel(ctx, Channel, "crash.p2p_settled", map[string]interface{}{
			"round_id":         round.ID,
			"round_number":     round.RoundNumber,
			"prize_pool":       prizePool.Float(),
			"total_net_payout": totalNetPayout.Float(),
			"total_fee":        totalFee.Float(),
			"service_fee_pct":  float64(round.ServiceFeeBP) / 100,
			"winner_count":     len(settledWinners),
			"winners":          winnerList,
			"settled_at":       now,
		})
	}

	return result, nil
}

// settleSingleP2PWinner credits net_payout to one winner and records the fee.
func (s *CrashService) settleSingleP2PWinner(ctx context.Context, round *domain.Round, bet *domain.Bet, gross, profit, fee, net money.Cents, now time.Time) (*P2PWinnerResult, error) {
	tx, err := s.bets.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock the winner's wallet.
	wallet, err := s.wallets.EnsureWalletTx(ctx, tx, bet.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to lock wallet for winner %s: %w", bet.UserID, err)
	}

	balanceBefore := money.FromFloat(wallet.Balance)
	newBalance := balanceBefore + net

	creditTxID := newID()
	feeTxID := ""

	// Credit net payout.
	creditTx := &domain.WalletTransaction{
		ID:            creditTxID,
		WalletID:      wallet.ID,
		UserID:        bet.UserID,
		Type:          TxTypePayout,
		Status:        "completed",
		Amount:        net.Float(),
		BalanceBefore: balanceBefore.Float(),
		BalanceAfter:  newBalance.Float(),
		Reference:     "crash_payout:" + bet.ID,
		Description: fmt.Sprintf(
			"P2P crash win %.2fx: gross %.2f, fee %.2f (%.2f%%), net %.2f (round %d)",
			domain.MultiplierValue(bet.CashoutMultiplierBP),
			gross.Float(), fee.Float(), float64(round.ServiceFeeBP)/100, net.Float(),
			round.RoundNumber,
		),
		Metadata: map[string]interface{}{
			"round_id":           round.ID,
			"round_number":       round.RoundNumber,
			"game":               "crash",
			"p2p":                true,
			"cashout_multiplier": domain.MultiplierValue(bet.CashoutMultiplierBP),
			"gross_return":       gross.Float(),
			"gross_profit":       profit.Float(),
			"service_fee":        fee.Float(),
			"service_fee_bp":     round.ServiceFeeBP,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.wallets.CreateTransactionTx(ctx, tx, creditTx); err != nil {
		return nil, fmt.Errorf("failed to write credit tx: %w", err)
	}
	if err := s.wallets.UpdateBalanceTx(ctx, tx, wallet.ID, newBalance.Float()); err != nil {
		return nil, fmt.Errorf("failed to update winner balance: %w", err)
	}

	// Record service fee ledger entry. The fee is recorded against the winner's
	// wallet (which is already locked above) so the NOT NULL wallet_id constraint
	// is satisfied. The transaction type "crash_platform_fee" distinguishes it
	// from a payout and makes it queryable as platform revenue.
	if fee > 0 {
		feeTxID = newID()
		feeUserID := s.cfg.PlatformUserID
		if feeUserID == "" {
			feeUserID = bet.UserID // fall back to winner as the fee source
		}
		feeTx := &domain.WalletTransaction{
			ID:            feeTxID,
			WalletID:      wallet.ID, // winner's wallet (NOT NULL constraint)
			UserID:        feeUserID,
			Type:          TxTypePlatformFee,
			Status:        "completed",
			Amount:        fee.Float(),
			BalanceBefore: balanceBefore.Float(),
			BalanceAfter:  newBalance.Float(), // balance already includes the net credit
			Reference:     "crash_fee:" + bet.ID,
			Description:   fmt.Sprintf("P2P crash service fee from %s (round %d)", bet.UserID, round.RoundNumber),
			Metadata: map[string]interface{}{
				"round_id":       round.ID,
				"round_number":   round.RoundNumber,
				"from_user_id":   bet.UserID,
				"bet_id":         bet.ID,
				"service_fee_bp": round.ServiceFeeBP,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.wallets.CreateTransactionTx(ctx, tx, feeTx); err != nil {
			return nil, fmt.Errorf("failed to write fee tx: %w", err)
		}
	}

	// Write settlement figures onto the bet row. Idempotent: only writes when
	// net_payout_cents = 0 (unset).
	applied, err := s.bets.SettleP2PWinnerTx(ctx, tx, bet.ID,
		int64(gross), int64(profit), int64(fee), int64(net),
		creditTxID, feeTxID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to settle winner bet: %w", err)
	}
	if !applied {
		// Already settled in a previous pass (idempotent). Reload and return.
		_ = tx.Rollback()
		settled, ferr := s.bets.FindByRoundAndUser(ctx, round.ID, bet.UserID)
		if ferr != nil {
			return nil, ferr
		}
		displayName := s.displayName(ctx, bet.UserID)
		return &P2PWinnerResult{
			BetID:            settled.ID,
			UserID:           settled.UserID,
			DisplayName:      displayName,
			StakeCents:       settled.StakeCents,
			CashoutBP:        settled.CashoutMultiplierBP,
			GrossPayoutCents: settled.GrossPayoutCents,
			GrossProfitCents: settled.GrossProfitCents,
			ServiceFeeCents:  settled.ServiceFeeCents,
			NetPayoutCents:   settled.NetPayoutCents,
		}, nil
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit winner settlement: %w", err)
	}

	displayName := s.displayName(ctx, bet.UserID)
	s.recordBetAudit(ctx, bet.UserID, "crash_p2p_winner_settled", bet,
		fmt.Sprintf("P2P winner: gross=%.2f profit=%.2f fee=%.2f net=%.2f (round %d)",
			gross.Float(), profit.Float(), fee.Float(), net.Float(), round.RoundNumber))

	if s.realtime != nil {
		_ = s.realtime.Publish(ctx, "wallet_update", bet.UserID, "", map[string]interface{}{
			"balance":  newBalance.Float(),
			"currency": wallet.Currency,
		})
	}

	return &P2PWinnerResult{
		BetID:            bet.ID,
		UserID:           bet.UserID,
		DisplayName:      displayName,
		StakeCents:       bet.StakeCents,
		CashoutBP:        bet.CashoutMultiplierBP,
		GrossPayoutCents: gross,
		GrossProfitCents: profit,
		ServiceFeeCents:  fee,
		NetPayoutCents:   net,
	}, nil
}

// creditPlatformRevenue records the prize pool as platform revenue when there
// are no winners (e.g. everyone rode into the crash). The ledger entry is
// written against the first participant's wallet (which already holds the
// debited stake) so the NOT NULL wallet_id constraint is satisfied. The
// transaction type "crash_platform_fee" marks it as platform revenue.
func (s *CrashService) creditPlatformRevenue(ctx context.Context, round *domain.Round, amount money.Cents, _ int64, _ int64, now time.Time, reason string) error {
	if amount <= 0 {
		return nil
	}

	// Find any participant's wallet to satisfy the NOT NULL constraint on
	// wallet_transactions.wallet_id. The balance is not changed — the stakes
	// were already debited at bet time and are not refunded here.
	allBets, err := s.bets.FindByRound(ctx, round.ID)
	if err != nil {
		return fmt.Errorf("failed to load bets for platform revenue: %w", err)
	}
	if len(allBets) == 0 {
		return nil // no bets at all, nothing to record
	}
	firstBet := allBets[0]

	tx, err := s.bets.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	wallet, err := s.wallets.EnsureWalletTx(ctx, tx, firstBet.UserID)
	if err != nil {
		return fmt.Errorf("failed to load wallet for platform revenue: %w", err)
	}

	platformUserID := s.cfg.PlatformUserID
	if platformUserID == "" {
		platformUserID = firstBet.UserID
	}
	txID := newID()
	feeTx := &domain.WalletTransaction{
		ID:            txID,
		WalletID:      wallet.ID,
		UserID:        platformUserID,
		Type:          TxTypePlatformFee,
		Status:        "completed",
		Amount:        amount.Float(),
		BalanceBefore: wallet.Balance,
		BalanceAfter:  wallet.Balance, // no balance change — stake was already debited
		Reference:     "crash_pool:" + round.ID,
		Description:   fmt.Sprintf("P2P crash pool collected (round %d, reason=%s)", round.RoundNumber, reason),
		Metadata: map[string]interface{}{
			"round_id":     round.ID,
			"round_number": round.RoundNumber,
			"reason":       reason,
			"game":         "crash",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.wallets.CreateTransactionTx(ctx, tx, feeTx); err != nil {
		return err
	}
	return tx.Commit()
}

// P2PSettlementResult summarises the outcome of a P2P round settlement.
type P2PSettlementResult struct {
	RoundID              string
	PrizePoolCents       money.Cents
	ServiceFeeBP         int
	WinnerCount          int
	TotalNetPayoutCents  money.Cents
	TotalFeeCents        money.Cents
	PlatformRevenueCents money.Cents
	Winners              []P2PWinnerResult
}

// P2PWinnerResult is one winner's settlement breakdown.
type P2PWinnerResult struct {
	BetID            string
	UserID           string
	DisplayName      string
	StakeCents       money.Cents
	CashoutBP        int
	GrossPayoutCents money.Cents
	GrossProfitCents money.Cents
	ServiceFeeCents  money.Cents
	NetPayoutCents   money.Cents
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// StateSnapshot is everything the game screen needs to render immediately.
type StateSnapshot struct {
	ServerTime time.Time   `json:"server_time"`
	Round      *RoundView  `json:"round"`
	Multiplier float64     `json:"multiplier"`
	MyBet      *BetView    `json:"my_bet,omitempty"`
	LiveBets   []BetView   `json:"live_bets"`
	Recent     []RoundView `json:"recent"`
	Previous   *RoundView  `json:"previous_round,omitempty"`
	Config     ConfigView  `json:"config"`
}

// ConfigView exposes the tunables the client needs for validation and display.
type ConfigView struct {
	GrowthRate       float64 `json:"growth_rate"`
	HouseEdgePercent float64 `json:"house_edge_percent"`
	MinBet           float64 `json:"min_bet"`
	MaxBet           float64 `json:"max_bet"`
	BettingSeconds   float64 `json:"betting_seconds"`
	MaxMultiplier    float64 `json:"max_multiplier"`
	Currency         string  `json:"currency"`
	// P2P fields
	ServiceFeePercent float64 `json:"service_fee_percent"`
	MinParticipants   int     `json:"min_participants"`
	MaxParticipants   int     `json:"max_participants"`
	P2PMode           bool    `json:"p2p_mode"`
}

// RoundView is the public projection of a round.
type RoundView struct {
	ID                    string     `json:"id"`
	RoundNumber           int64      `json:"round_number"`
	State                 string     `json:"state"`
	BettingStartsAt       time.Time  `json:"betting_starts_at"`
	BettingEndsAt         time.Time  `json:"betting_ends_at"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	CrashedAt             *time.Time `json:"crashed_at,omitempty"`
	SettledAt             *time.Time `json:"settled_at,omitempty"`
	CrashMultiplier       float64    `json:"crash_multiplier"`
	ServerSeedHash        string     `json:"server_seed_hash"`
	CommitmentPublishedAt time.Time  `json:"commitment_published_at"`
	ServerSeed            string     `json:"server_seed,omitempty"`
	Nonce                 int64      `json:"nonce"`
	TotalStake            float64    `json:"total_stake"`
	TotalPayout           float64    `json:"total_payout"`
	BetCount              int        `json:"bet_count"`
	CashoutCount          int        `json:"cashout_count"`
	// P2P fields
	P2PMode           bool    `json:"p2p_mode"`
	PrizePool         float64 `json:"prize_pool"`
	ServiceFeePercent float64 `json:"service_fee_percent"`
	MinParticipants   int     `json:"min_participants"`
	TotalFee          float64 `json:"total_fee"`
}

// BetView is the public projection of a bet.
type BetView struct {
	ID                    string     `json:"id"`
	RoundID               string     `json:"round_id"`
	UserID                string     `json:"user_id"`
	DisplayName           string     `json:"display_name"`
	Stake                 float64    `json:"stake"`
	Status                string     `json:"status"`
	AutoCashoutMultiplier float64    `json:"auto_cashout_multiplier,omitempty"`
	CashoutMultiplier     float64    `json:"cashout_multiplier,omitempty"`
	Payout                float64    `json:"payout"`
	CashedOutAt           *time.Time `json:"cashed_out_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	// P2P settlement fields (populated after settlement).
	GrossReturn   float64 `json:"gross_return,omitempty"`
	GrossProfit   float64 `json:"gross_profit,omitempty"`
	ServiceFee    float64 `json:"service_fee,omitempty"`
	ServiceFeePct float64 `json:"service_fee_pct,omitempty"`
	NetPayout     float64 `json:"net_payout,omitempty"`
}

// VerificationView is the public commit-reveal proof for a finished round.
type VerificationView struct {
	RoundID           string  `json:"round_id"`
	RoundNumber       int64   `json:"round_number"`
	ServerSeed        string  `json:"server_seed"`
	ServerSeedHash    string  `json:"server_seed_hash"`
	Nonce             int64   `json:"nonce"`
	CrashMultiplier   float64 `json:"crash_multiplier"`
	Recomputed        float64 `json:"recomputed_multiplier"`
	SeedMatchesCommit bool    `json:"seed_matches_commitment"`
	Verified          bool    `json:"verified"`
	Algorithm         string  `json:"algorithm"`
	Note              string  `json:"note"`
}

// GetState returns the current round, the caller's bet, the live table and
// recent history in one round trip.
func (s *CrashService) GetState(ctx context.Context, userID string) (*StateSnapshot, error) {
	now := s.clock.Now()
	snapshot := &StateSnapshot{
		ServerTime: now,
		Multiplier: domain.MultiplierValue(domain.MinMultiplierBP),
		LiveBets:   []BetView{},
		Recent:     []RoundView{},
		Config: ConfigView{
			GrowthRate:        s.cfg.GrowthRate,
			HouseEdgePercent:  float64(s.cfg.HouseEdgeBP) / 100,
			MinBet:            s.cfg.MinBetCents.Float(),
			MaxBet:            s.cfg.MaxBetCents.Float(),
			BettingSeconds:    s.cfg.BettingSeconds,
			MaxMultiplier:     domain.MultiplierValue(s.cfg.MaxMultiplierBP),
			Currency:          s.currencyFor(ctx, userID),
			ServiceFeePercent: float64(s.cfg.ServiceFeeBP) / 100,
			MinParticipants:   s.cfg.MinParticipants,
			MaxParticipants:   s.cfg.MaxParticipants,
			P2PMode:           true,
		},
	}

	round, err := s.rounds.FindLive(ctx, s.cfg.Room)
	if err != nil && err != domain.ErrNoLiveRound {
		return nil, apperrors.WithField("DB_ERROR", "failed to load crash round", 500, err)
	}

	if round != nil {
		view := s.roundView(round)
		snapshot.Round = &view

		if round.State == domain.StateRunning && round.StartedAt != nil {
			crashBP := s.source.CrashPoint(round.ServerSeed, round.Nonce)
			elapsed := now.Sub(*round.StartedAt).Seconds()
			current := domain.MultiplierAt(elapsed, s.cfg.GrowthRate)
			if current >= crashBP {
				current = crashBP
			}
			snapshot.Multiplier = domain.MultiplierValue(current)
		}

		bets, err := s.bets.FindByRound(ctx, round.ID)
		if err != nil {
			return nil, apperrors.WithField("DB_ERROR", "failed to load round bets", 500, err)
		}
		names := s.displayNames(ctx, betUserIDs(bets))
		for _, bet := range bets {
			displayName := names[bet.UserID]
			snapshot.LiveBets = append(snapshot.LiveBets, s.betView(ctx, bet, displayName))
			if userID != "" && bet.UserID == userID {
				myBet := s.betView(ctx, bet, displayName)
				snapshot.MyBet = &myBet
			}
		}
	}

	recent, err := s.rounds.Recent(ctx, s.cfg.Room, 20)
	if err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to load crash history", 500, err)
	}
	for _, r := range recent {
		view := s.roundView(r)
		snapshot.Recent = append(snapshot.Recent, view)
		if snapshot.Previous == nil && r.FairnessRevealed() {
			previous := view
			snapshot.Previous = &previous
		}
	}

	return snapshot, nil
}

// History returns recent settled rounds for the history strip.
func (s *CrashService) History(ctx context.Context, limit int) ([]RoundView, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rounds, err := s.rounds.Recent(ctx, s.cfg.Room, limit)
	if err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to load crash history", 500, err)
	}
	out := make([]RoundView, 0, len(rounds))
	for _, r := range rounds {
		out = append(out, s.roundView(r))
	}
	return out, nil
}

// MyBets returns the caller's bet history.
func (s *CrashService) MyBets(ctx context.Context, userID string, limit, offset int) ([]BetView, int64, error) {
	if userID == "" {
		return nil, 0, apperrors.ErrUnauthorized
	}
	bets, err := s.bets.FindByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.WithField("DB_ERROR", "failed to load bets", 500, err)
	}
	total, err := s.bets.CountByUser(ctx, userID)
	if err != nil {
		return nil, 0, apperrors.WithField("DB_ERROR", "failed to count bets", 500, err)
	}
	displayName := s.displayName(ctx, userID)
	out := make([]BetView, 0, len(bets))
	for _, bet := range bets {
		out = append(out, s.betView(ctx, bet, displayName))
	}
	return out, total, nil
}

// RoundBets returns the public player list for a round.
func (s *CrashService) RoundBets(ctx context.Context, roundID string) ([]BetView, error) {
	bets, err := s.bets.FindByRound(ctx, roundID)
	if err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to load round bets", 500, err)
	}
	names := s.displayNames(ctx, betUserIDs(bets))
	out := make([]BetView, 0, len(bets))
	for _, bet := range bets {
		out = append(out, s.betView(ctx, bet, names[bet.UserID]))
	}
	return out, nil
}

func betUserIDs(bets []*domain.Bet) []string {
	ids := make([]string, 0, len(bets))
	for _, bet := range bets {
		ids = append(ids, bet.UserID)
	}
	return ids
}

// VerifyRound publishes the commit-reveal proof for a finished round.
func (s *CrashService) VerifyRound(ctx context.Context, roundID string) (*VerificationView, error) {
	round, err := s.rounds.FindByID(ctx, roundID)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !round.FairnessRevealed() {
		return nil, apperrors.WithField("ROUND_LIVE", "the round has not finished yet; its seed is still sealed", 409, nil)
	}

	recomputed := s.source.CrashPoint(round.ServerSeed, round.Nonce)
	seedOK := s.source.Verify(round.ServerSeed, round.ServerSeedHash)

	return &VerificationView{
		RoundID:           round.ID,
		RoundNumber:       round.RoundNumber,
		ServerSeed:        round.ServerSeed,
		ServerSeedHash:    round.ServerSeedHash,
		Nonce:             round.Nonce,
		CrashMultiplier:   domain.MultiplierValue(round.CrashMultiplierBP),
		Recomputed:        domain.MultiplierValue(recomputed),
		SeedMatchesCommit: seedOK,
		Verified:          seedOK && recomputed == round.CrashMultiplierBP,
		Algorithm:         "sha256(server_seed:nonce) -> uniform bucket -> crash = max(1.00, (1-edge)/(1-u))",
		Note: "The crash point was fixed when the seed hash was published, before betting opened. " +
			"Verification proves the revealed seed matches that commitment and reproduces the recorded crash multiplier.",
	}, nil
}

// ---------------------------------------------------------------------------
// Admin settings
// ---------------------------------------------------------------------------

// GetSettings returns all admin-configurable crash game settings.
func (s *CrashService) GetSettings(ctx context.Context) (map[string]string, error) {
	if s.settings == nil {
		return map[string]string{}, nil
	}
	return s.settings.GetAll(ctx)
}

// UpdateSetting writes a single setting value (admin only).
func (s *CrashService) UpdateSetting(ctx context.Context, key, value, adminUserID string) error {
	if s.settings == nil {
		return apperrors.WithField("SETTINGS_UNAVAILABLE", "settings repository not configured", 500, nil)
	}
	// Validate numeric settings.
	allowedKeys := map[string]bool{
		"service_fee_bp": true, "min_participants": true, "max_participants": true,
		"min_bet_cents": true, "max_bet_cents": true, "betting_seconds": true,
		"waiting_seconds": true, "crashed_seconds": true,
	}
	if !allowedKeys[key] {
		return apperrors.WithField("INVALID_KEY", "unknown setting key", 400, nil)
	}
	return s.settings.Set(ctx, key, value, adminUserID)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func (s *CrashService) roundView(round *domain.Round) RoundView {
	view := RoundView{
		ID:                    round.ID,
		RoundNumber:           round.RoundNumber,
		State:                 round.State,
		BettingStartsAt:       round.BettingStartsAt,
		BettingEndsAt:         round.BettingEndsAt,
		StartedAt:             round.StartedAt,
		CrashedAt:             round.CrashedAt,
		SettledAt:             round.SettledAt,
		CrashMultiplier:       domain.MultiplierValue(round.CrashMultiplierBP),
		ServerSeedHash:        round.ServerSeedHash,
		CommitmentPublishedAt: round.CommitmentPublishedAt,
		Nonce:                 round.Nonce,
		TotalStake:            round.TotalStakeCents.Float(),
		TotalPayout:           round.TotalPayoutCents.Float(),
		BetCount:              round.BetCount,
		CashoutCount:          round.CashoutCount,
		P2PMode:               round.P2PMode,
		PrizePool:             round.PrizePoolCents.Float(),
		ServiceFeePercent:     float64(round.ServiceFeeBP) / 100,
		MinParticipants:       round.MinParticipants,
		TotalFee:              round.TotalFeeCents.Float(),
	}
	if round.FairnessRevealed() {
		view.ServerSeed = round.ServerSeed
	}
	return view
}

func (s *CrashService) betView(_ context.Context, bet *domain.Bet, displayName string) BetView {
	view := BetView{
		ID:          bet.ID,
		RoundID:     bet.RoundID,
		UserID:      bet.UserID,
		DisplayName: displayName,
		Stake:       bet.StakeAmount,
		Status:      bet.Status,
		Payout:      bet.PayoutAmount,
		CashedOutAt: bet.CashedOutAt,
		CreatedAt:   bet.CreatedAt,
	}
	if bet.AutoCashoutBP > 0 {
		view.AutoCashoutMultiplier = domain.MultiplierValue(bet.AutoCashoutBP)
	}
	if bet.CashoutMultiplierBP > 0 {
		view.CashoutMultiplier = domain.MultiplierValue(bet.CashoutMultiplierBP)
	}
	if bet.GrossPayoutCents > 0 {
		view.GrossReturn = bet.GrossPayoutCents.Float()
		view.GrossProfit = bet.GrossProfitCents.Float()
		view.ServiceFee = bet.ServiceFeeCents.Float()
		view.NetPayout = bet.NetPayoutCents.Float()
	}
	return view
}

// displayName resolves the masked public name for a single player.
func (s *CrashService) displayName(ctx context.Context, userID string) string {
	return s.displayNames(ctx, []string{userID})[userID]
}

// displayNames resolves the masked public name of every listed player in one query.
func (s *CrashService) displayNames(ctx context.Context, userIDs []string) map[string]string {
	names := make(map[string]string, len(userIDs))
	if len(userIDs) == 0 {
		return names
	}

	unique := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id == "" {
			names[id] = "Player"
			continue
		}
		if _, seen := names[id]; seen {
			continue
		}
		names[id] = "Player"
		unique = append(unique, id)
	}

	if s.players == nil || len(unique) == 0 {
		return names
	}
	infos, err := s.players.FindByIDs(ctx, unique)
	if err != nil {
		return names
	}
	for id, info := range infos {
		if info == nil {
			continue
		}
		if name := domain.DisplayName(info.FullName, info.Email); name != "" {
			names[id] = name
		}
	}
	return names
}

// currencyFor reports the currency the player's balance is held in.
func (s *CrashService) currencyFor(ctx context.Context, userID string) string {
	if s.wallets == nil || userID == "" {
		return defaultCurrency
	}
	currency, err := s.wallets.CurrencyFor(ctx, userID)
	if err != nil || currency == "" {
		return defaultCurrency
	}
	return currency
}

func (s *CrashService) recordBetAudit(ctx context.Context, userID, action string, bet *domain.Bet, newValue string) {
	if s.audit == nil {
		return
	}
	actorID := userID
	actorType := "user"
	if role := appcontext.GetUserRole(ctx); role != "" {
		actorType = role
	}
	_ = s.audit.Record(ctx, &actorID, actorType, action, "game_bet", &bet.ID, "", nil, &newValue)
}

func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}

// newID returns a canonical UUID string.
func newID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
