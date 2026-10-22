package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/raffle-app/backend/internal/crash/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/money"
)

// roundColumns selects every field that scanRound expects, including the P2P
// columns added in migration 20240101000009_crash_p2p.
const roundColumns = `id, room, round_number, state, server_seed, server_seed_hash, nonce,
	commitment_published_at, betting_starts_at, betting_ends_at, started_at, crashed_at, settled_at,
	COALESCE(crash_multiplier_bp, 0),
	total_stake_cents, total_payout_cents, bet_count, cashout_count,
	COALESCE(p2p_mode, true),
	COALESCE(prize_pool_cents, 0),
	COALESCE(service_fee_bp, 500),
	COALESCE(min_participants, 2),
	COALESCE(total_fee_cents, 0),
	created_at, updated_at`

// betColumns selects every field that scanBet expects, including P2P settlement
// columns added in migration 20240101000009_crash_p2p.
const betColumns = `id, round_id, user_id, wallet_id, stake_cents, stake_amount, status,
	COALESCE(auto_cashout_bp, 0), COALESCE(cashout_multiplier_bp, 0), payout_cents, payout_amount,
	COALESCE(debit_tx_id::text, ''), COALESCE(credit_tx_id::text, ''),
	COALESCE(gross_payout_cents, 0), COALESCE(gross_profit_cents, 0),
	COALESCE(service_fee_cents, 0), COALESCE(net_payout_cents, 0),
	COALESCE(fee_tx_id::text, ''),
	cashed_out_at, settled_at, created_at, updated_at`

// ---------------------------------------------------------------------------
// Rounds
// ---------------------------------------------------------------------------

type RoundRepo struct{ db *sql.DB }

func NewRoundRepo(db *sql.DB) *RoundRepo { return &RoundRepo{db: db} }

func (r *RoundRepo) BeginTx(ctx context.Context) (*sql.Tx, error) { return r.db.BeginTx(ctx, nil) }

func (r *RoundRepo) Create(ctx context.Context, round *domain.Round) error {
	query := `INSERT INTO game_rounds (
		room, round_number, state,
		server_seed, server_seed_hash, nonce, commitment_published_at,
		betting_starts_at, betting_ends_at,
		p2p_mode, service_fee_bp, min_participants,
		created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NOW()) RETURNING id`
	err := r.db.QueryRowContext(ctx, query,
		round.Room, round.RoundNumber, round.State,
		round.ServerSeed, round.ServerSeedHash, round.Nonce, round.CommitmentPublishedAt,
		round.BettingStartsAt, round.BettingEndsAt,
		round.P2PMode, round.ServiceFeeBP, round.MinParticipants,
	).Scan(&round.ID)
	if err != nil {
		return fmt.Errorf("failed to create crash round: %w", err)
	}
	return nil
}

func (r *RoundRepo) FindByID(ctx context.Context, id string) (*domain.Round, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM game_rounds WHERE id = $1`, id)
	return scanRound(row, apperrors.ErrNotFound)
}

func (r *RoundRepo) FindLive(ctx context.Context, room string) (*domain.Round, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM game_rounds
		WHERE room = $1 AND state = ANY($2) ORDER BY round_number DESC LIMIT 1`,
		room, pq.Array([]string{domain.StateWaiting, domain.StateBetting, domain.StateRunning}))
	return scanRound(row, domain.ErrNoLiveRound)
}

// FindCurrent is what the engine drives. It includes the post-crash states so
// that a crashed round is still settled and closed out before the next one is
// created.
func (r *RoundRepo) FindCurrent(ctx context.Context, room string) (*domain.Round, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM game_rounds
		WHERE room = $1 AND state = ANY($2) ORDER BY round_number DESC LIMIT 1`,
		room, pq.Array([]string{domain.StateWaiting, domain.StateBetting, domain.StateRunning, domain.StateCrashed, domain.StateSettlement}))
	return scanRound(row, domain.ErrNoLiveRound)
}

func (r *RoundRepo) FindLiveForUpdate(ctx context.Context, tx *sql.Tx, room string) (*domain.Round, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM game_rounds
		WHERE room = $1 AND state = ANY($2) ORDER BY round_number DESC LIMIT 1 FOR UPDATE`,
		room, pq.Array([]string{domain.StateWaiting, domain.StateBetting, domain.StateRunning}))
	return scanRound(row, domain.ErrNoLiveRound)
}

func (r *RoundRepo) FindByRoomAndState(ctx context.Context, room string, states []string, limit int) ([]*domain.Round, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+roundColumns+` FROM game_rounds
		WHERE room = $1 AND state = ANY($2) ORDER BY round_number DESC LIMIT $3`,
		room, pq.Array(states), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRounds(rows)
}

func (r *RoundRepo) MarkBettingStartedTx(ctx context.Context, tx *sql.Tx, roundID string) (bool, error) {
	return r.transitionTx(ctx, tx, roundID, domain.StateWaiting, domain.StateBetting, "")
}

func (r *RoundRepo) MarkRunningTx(ctx context.Context, tx *sql.Tx, roundID string, startedAt time.Time) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE game_rounds SET state = $3, started_at = $4, updated_at = NOW()
		WHERE id = $1 AND state = $2`, roundID, domain.StateBetting, domain.StateRunning, startedAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (r *RoundRepo) MarkCrashedTx(ctx context.Context, tx *sql.Tx, roundID string, crashBP int, crashedAt time.Time) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE game_rounds
		SET state = $3, crash_multiplier_bp = $4, crashed_at = $5, updated_at = NOW()
		WHERE id = $1 AND state = $2`,
		roundID, domain.StateRunning, domain.StateCrashed, crashBP, crashedAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// MarkSettledTx is the legacy (non-P2P) settlement path; preserved for backward
// compatibility with non-P2P rounds.
func (r *RoundRepo) MarkSettledTx(ctx context.Context, tx *sql.Tx, roundID string, totalStakeCents, totalPayoutCents int64, settledAt time.Time) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE game_rounds
		SET state = $3, total_stake_cents = $4, total_payout_cents = $5, settled_at = $6, updated_at = NOW()
		WHERE id = $1 AND state = $2`,
		roundID, domain.StateCrashed, domain.StateSettlement, totalStakeCents, totalPayoutCents, settledAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// MarkSettledP2PTx is the P2P settlement path. It also stores the prize pool
// and total fee so the round row is self-contained for auditing.
func (r *RoundRepo) MarkSettledP2PTx(ctx context.Context, tx *sql.Tx, roundID string, totalStakeCents, totalPayoutCents, prizePoolCents, totalFeeCents int64, settledAt time.Time) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE game_rounds
		SET state = $3,
		    total_stake_cents  = $4,
		    total_payout_cents = $5,
		    prize_pool_cents   = $6,
		    total_fee_cents    = $7,
		    settled_at         = $8,
		    updated_at         = NOW()
		WHERE id = $1 AND state = $2`,
		roundID, domain.StateCrashed, domain.StateSettlement,
		totalStakeCents, totalPayoutCents, prizePoolCents, totalFeeCents, settledAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (r *RoundRepo) MarkNextRoundTx(ctx context.Context, tx *sql.Tx, roundID string) (bool, error) {
	return r.transitionTx(ctx, tx, roundID, domain.StateSettlement, domain.StateNextRound, "")
}

func (r *RoundRepo) transitionTx(ctx context.Context, tx *sql.Tx, roundID, from, to, _ string) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE game_rounds SET state = $3, updated_at = NOW() WHERE id = $1 AND state = $2`, roundID, from, to)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (r *RoundRepo) Recent(ctx context.Context, room string, limit int) ([]*domain.Round, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+roundColumns+` FROM game_rounds
		WHERE room = $1 AND crash_multiplier_bp IS NOT NULL
		ORDER BY round_number DESC LIMIT $2`, room, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRounds(rows)
}

func (r *RoundRepo) AdjustTotalsTx(ctx context.Context, tx *sql.Tx, roundID string, stakeDelta, payoutDelta int64, betDelta, cashoutDelta int) error {
	_, err := tx.ExecContext(ctx, `UPDATE game_rounds
		SET total_stake_cents  = total_stake_cents  + $2,
		    total_payout_cents = total_payout_cents + $3,
		    prize_pool_cents   = prize_pool_cents   + $2,
		    bet_count          = bet_count          + $4,
		    cashout_count      = cashout_count      + $5,
		    updated_at         = NOW()
		WHERE id = $1`, roundID, stakeDelta, payoutDelta, betDelta, cashoutDelta)
	return err
}

func (r *RoundRepo) NextRoundNumber(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `SELECT nextval('crash_round_number_seq')`).Scan(&n)
	return n, err
}

func scanRound(row *sql.Row, notFound error) (*domain.Round, error) {
	round := &domain.Round{}
	var startedAt, crashedAt, settledAt sql.NullTime
	var totalStake, totalPayout, prizePool, totalFee int64
	err := row.Scan(
		&round.ID, &round.Room, &round.RoundNumber, &round.State,
		&round.ServerSeed, &round.ServerSeedHash, &round.Nonce, &round.CommitmentPublishedAt,
		&round.BettingStartsAt, &round.BettingEndsAt,
		&startedAt, &crashedAt, &settledAt, &round.CrashMultiplierBP,
		&totalStake, &totalPayout, &round.BetCount, &round.CashoutCount,
		&round.P2PMode, &prizePool, &round.ServiceFeeBP, &round.MinParticipants, &totalFee,
		&round.CreatedAt, &round.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, notFound
		}
		return nil, err
	}
	round.TotalStakeCents = money.Cents(totalStake)
	round.TotalPayoutCents = money.Cents(totalPayout)
	round.PrizePoolCents = money.Cents(prizePool)
	round.TotalFeeCents = money.Cents(totalFee)
	if startedAt.Valid {
		t := startedAt.Time
		round.StartedAt = &t
	}
	if crashedAt.Valid {
		t := crashedAt.Time
		round.CrashedAt = &t
	}
	if settledAt.Valid {
		t := settledAt.Time
		round.SettledAt = &t
	}
	return round, nil
}

func scanRounds(rows *sql.Rows) ([]*domain.Round, error) {
	out := []*domain.Round{}
	for rows.Next() {
		round := &domain.Round{}
		var startedAt, crashedAt, settledAt sql.NullTime
		var totalStake, totalPayout, prizePool, totalFee int64
		err := rows.Scan(
			&round.ID, &round.Room, &round.RoundNumber, &round.State,
			&round.ServerSeed, &round.ServerSeedHash, &round.Nonce, &round.CommitmentPublishedAt,
			&round.BettingStartsAt, &round.BettingEndsAt,
			&startedAt, &crashedAt, &settledAt, &round.CrashMultiplierBP,
			&totalStake, &totalPayout, &round.BetCount, &round.CashoutCount,
			&round.P2PMode, &prizePool, &round.ServiceFeeBP, &round.MinParticipants, &totalFee,
			&round.CreatedAt, &round.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		round.TotalStakeCents = money.Cents(totalStake)
		round.TotalPayoutCents = money.Cents(totalPayout)
		round.PrizePoolCents = money.Cents(prizePool)
		round.TotalFeeCents = money.Cents(totalFee)
		if startedAt.Valid {
			t := startedAt.Time
			round.StartedAt = &t
		}
		if crashedAt.Valid {
			t := crashedAt.Time
			round.CrashedAt = &t
		}
		if settledAt.Valid {
			t := settledAt.Time
			round.SettledAt = &t
		}
		out = append(out, round)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Bets
// ---------------------------------------------------------------------------

type BetRepo struct{ db *sql.DB }

func NewBetRepo(db *sql.DB) *BetRepo { return &BetRepo{db: db} }

func (r *BetRepo) BeginTx(ctx context.Context) (*sql.Tx, error) { return r.db.BeginTx(ctx, nil) }

func (r *BetRepo) CreateTx(ctx context.Context, tx *sql.Tx, bet *domain.Bet) error {
	query := `INSERT INTO game_bets (
		id, round_id, user_id, wallet_id, stake_cents, stake_amount, status,
		auto_cashout_bp, payout_cents, payout_amount, debit_tx_id, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,0,0,$9,NOW(),NOW())`
	var auto sql.NullInt64
	if bet.AutoCashoutBP > 0 {
		auto = sql.NullInt64{Int64: int64(bet.AutoCashoutBP), Valid: true}
	}
	_, err := tx.ExecContext(ctx, query,
		bet.ID, bet.RoundID, bet.UserID, bet.WalletID, int64(bet.StakeCents), bet.StakeAmount,
		bet.Status, auto, nullIfEmpty(bet.DebitTxID),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateBet
		}
		return err
	}
	return nil
}

func (r *BetRepo) FindByRoundAndUser(ctx context.Context, roundID, userID string) (*domain.Bet, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+betColumns+` FROM game_bets WHERE round_id = $1 AND user_id = $2`, roundID, userID)
	return scanBet(row)
}

func (r *BetRepo) FindByRoundAndUserTx(ctx context.Context, tx *sql.Tx, roundID, userID string) (*domain.Bet, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+betColumns+` FROM game_bets WHERE round_id = $1 AND user_id = $2 FOR UPDATE`, roundID, userID)
	return scanBet(row)
}

func (r *BetRepo) FindByRound(ctx context.Context, roundID string) ([]*domain.Bet, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+betColumns+` FROM game_bets WHERE round_id = $1 ORDER BY created_at ASC`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBets(rows)
}

func (r *BetRepo) FindActiveByRound(ctx context.Context, roundID string) ([]*domain.Bet, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+betColumns+` FROM game_bets
		WHERE round_id = $1 AND status = 'active' ORDER BY created_at ASC`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBets(rows)
}

// FindCashedOutByRound returns bets in cashed_out status for a round.
// Used at P2P settlement to identify the winners.
func (r *BetRepo) FindCashedOutByRound(ctx context.Context, roundID string) ([]*domain.Bet, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+betColumns+` FROM game_bets
		WHERE round_id = $1 AND status = 'cashed_out' ORDER BY cashout_multiplier_bp ASC`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBets(rows)
}

func (r *BetRepo) FindByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Bet, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+betColumns+` FROM game_bets
		WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBets(rows)
}

func (r *BetRepo) CountByUser(ctx context.Context, userID string) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_bets WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}

func (r *BetRepo) TotalsForRoundTx(ctx context.Context, tx *sql.Tx, roundID string) (*domain.RoundTotals, error) {
	totals := &domain.RoundTotals{}
	err := tx.QueryRowContext(ctx, `SELECT
			COALESCE(SUM(stake_cents), 0),
			COALESCE(SUM(payout_cents), 0),
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'cashed_out')
		FROM game_bets WHERE round_id = $1`, roundID).Scan(
		&totals.StakeCents, &totals.PayoutCents, &totals.BetCount, &totals.CashoutCount)
	if err != nil {
		return nil, err
	}
	return totals, nil
}

func (r *BetRepo) MarkCashedOutTx(ctx context.Context, tx *sql.Tx, betID string, multiplierBP int, payoutCents int64, creditTxID string, at time.Time) (bool, error) {
	// In P2P mode payoutCents is 0 here; the real payout is computed at
	// settlement. We still record multiplierBP so the proportional distribution
	// has a weight for this bet.
	res, err := tx.ExecContext(ctx, `UPDATE game_bets SET
			status = 'cashed_out',
			cashout_multiplier_bp = $2,
			payout_cents = $3::bigint,
			payout_amount = $3::bigint / 100.0,
			credit_tx_id = $4,
			cashed_out_at = $5,
			settled_at = $5,
			updated_at = NOW()
		WHERE id = $1 AND status = 'active'`,
		betID, multiplierBP, payoutCents, nullIfEmpty(creditTxID), at)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (r *BetRepo) MarkAllLostTx(ctx context.Context, tx *sql.Tx, roundID string, at time.Time) (int64, error) {
	res, err := tx.ExecContext(ctx, `UPDATE game_bets SET status = 'lost', settled_at = $2, updated_at = NOW()
		WHERE round_id = $1 AND status = 'active'`, roundID, at)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SettleP2PWinnerTx writes the P2P settlement figures onto a cashed-out bet.
// Idempotent: a re-run on an already-settled bet (net_payout_cents != 0) matches
// zero rows.
func (r *BetRepo) SettleP2PWinnerTx(ctx context.Context, tx *sql.Tx, betID string, grossPayout, grossProfit, serviceFee, netPayout int64, creditTxID, feeTxID string, settledAt time.Time) (bool, error) {
	payoutAmount := float64(netPayout) / 100.0
	res, err := tx.ExecContext(ctx, `UPDATE game_bets SET
			gross_payout_cents = $2,
			gross_profit_cents = $3,
			service_fee_cents  = $4,
			net_payout_cents   = $5,
			payout_cents       = $6,
			payout_amount      = $7,
			credit_tx_id       = $8,
			fee_tx_id          = $9,
			settled_at         = $10,
			updated_at         = NOW()
		WHERE id = $1 AND status = 'cashed_out' AND net_payout_cents = 0`,
		betID, grossPayout, grossProfit, serviceFee, netPayout,
		netPayout, payoutAmount,
		nullIfEmpty(creditTxID), nullIfEmpty(feeTxID), settledAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func scanBet(row *sql.Row) (*domain.Bet, error) {
	bet := &domain.Bet{}
	var cashedOutAt, settledAt sql.NullTime
	var stakeCents, payoutCents, grossPayout, grossProfit, serviceFee, netPayout int64
	err := row.Scan(
		&bet.ID, &bet.RoundID, &bet.UserID, &bet.WalletID,
		&stakeCents, &bet.StakeAmount, &bet.Status,
		&bet.AutoCashoutBP, &bet.CashoutMultiplierBP,
		&payoutCents, &bet.PayoutAmount,
		&bet.DebitTxID, &bet.CreditTxID,
		&grossPayout, &grossProfit, &serviceFee, &netPayout,
		&bet.FeeTxID,
		&cashedOutAt, &settledAt, &bet.CreatedAt, &bet.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	bet.StakeCents = money.Cents(stakeCents)
	bet.PayoutCents = money.Cents(payoutCents)
	bet.GrossPayoutCents = money.Cents(grossPayout)
	bet.GrossProfitCents = money.Cents(grossProfit)
	bet.ServiceFeeCents = money.Cents(serviceFee)
	bet.NetPayoutCents = money.Cents(netPayout)
	if cashedOutAt.Valid {
		t := cashedOutAt.Time
		bet.CashedOutAt = &t
	}
	if settledAt.Valid {
		t := settledAt.Time
		bet.SettledAt = &t
	}
	return bet, nil
}

func scanBets(rows *sql.Rows) ([]*domain.Bet, error) {
	out := []*domain.Bet{}
	for rows.Next() {
		bet := &domain.Bet{}
		var cashedOutAt, settledAt sql.NullTime
		var stakeCents, payoutCents, grossPayout, grossProfit, serviceFee, netPayout int64
		err := rows.Scan(
			&bet.ID, &bet.RoundID, &bet.UserID, &bet.WalletID,
			&stakeCents, &bet.StakeAmount, &bet.Status,
			&bet.AutoCashoutBP, &bet.CashoutMultiplierBP,
			&payoutCents, &bet.PayoutAmount,
			&bet.DebitTxID, &bet.CreditTxID,
			&grossPayout, &grossProfit, &serviceFee, &netPayout,
			&bet.FeeTxID,
			&cashedOutAt, &settledAt, &bet.CreatedAt, &bet.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		bet.StakeCents = money.Cents(stakeCents)
		bet.PayoutCents = money.Cents(payoutCents)
		bet.GrossPayoutCents = money.Cents(grossPayout)
		bet.GrossProfitCents = money.Cents(grossProfit)
		bet.ServiceFeeCents = money.Cents(serviceFee)
		bet.NetPayoutCents = money.Cents(netPayout)
		if cashedOutAt.Valid {
			t := cashedOutAt.Time
			bet.CashedOutAt = &t
		}
		if settledAt.Valid {
			t := settledAt.Time
			bet.SettledAt = &t
		}
		out = append(out, bet)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Wallet adapter (reuses the existing wallets / wallet_transactions tables)
// ---------------------------------------------------------------------------

type WalletRepo struct{ db *sql.DB }

func NewWalletRepo(db *sql.DB) *WalletRepo { return &WalletRepo{db: db} }

// EnsureWalletTx returns the player's wallet locked FOR UPDATE, lazily creating
// it when the player has never had one.
func (r *WalletRepo) EnsureWalletTx(ctx context.Context, tx *sql.Tx, userID string) (*domain.Wallet, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO wallets (id, user_id, balance, currency, created_at, updated_at)
		SELECT $1, $2, 0, 'ETB', NOW(), NOW()
		WHERE NOT EXISTS (SELECT 1 FROM wallets WHERE user_id = $2)`, newID(), userID)
	if err != nil {
		return nil, err
	}

	row := tx.QueryRowContext(ctx, `SELECT id, user_id, balance, currency FROM wallets WHERE user_id = $1 FOR UPDATE`, userID)
	w := &domain.Wallet{}
	if err := row.Scan(&w.ID, &w.UserID, &w.Balance, &w.Currency); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return w, nil
}

// CurrencyFor reports the currency of the player's wallet, or an empty string
// when the player has no wallet yet.
func (r *WalletRepo) CurrencyFor(ctx context.Context, userID string) (string, error) {
	var currency string
	if err := r.db.QueryRowContext(ctx, `SELECT currency FROM wallets WHERE user_id = $1`, userID).Scan(&currency); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return currency, nil
}

func (r *WalletRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	_, err := tx.ExecContext(ctx, `UPDATE wallets SET balance = $1, updated_at = $2 WHERE id = $3`, newBalance, time.Now(), walletID)
	return err
}

func (r *WalletRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *domain.WalletTransaction) error {
	metaJSON, err := marshalMetadata(walletTx.Metadata)
	if err != nil {
		return err
	}
	// wallet_id is nullable (migration 20240101000010): platform ledger entries
	// (type = crash_platform_fee, crash_pool_revenue) are not tied to a specific
	// wallet. Pass NULL when WalletID is empty rather than an invalid empty UUID.
	var walletID sql.NullString
	if walletTx.WalletID != "" {
		walletID = sql.NullString{String: walletTx.WalletID, Valid: true}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO wallet_transactions
		(id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		walletTx.ID, walletID, walletTx.UserID, walletTx.Type, walletTx.Status,
		walletTx.Amount, walletTx.BalanceBefore, walletTx.BalanceAfter,
		walletTx.Reference, walletTx.Description, metaJSON, time.Now(), time.Now())
	return err
}

func marshalMetadata(m map[string]interface{}) (string, error) {
	if m == nil {
		return "null", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return string(b), nil
}

// ---------------------------------------------------------------------------
// Settings repository (crash_settings table)
// ---------------------------------------------------------------------------

type SettingsRepo struct{ db *sql.DB }

func NewSettingsRepo(db *sql.DB) *SettingsRepo { return &SettingsRepo{db: db} }

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM crash_settings WHERE key = $1`, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", apperrors.ErrNotFound
		}
		return "", err
	}
	return value, nil
}

func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM crash_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

func (r *SettingsRepo) Set(ctx context.Context, key, value, updatedByUserID string) error {
	var updatedBy sql.NullString
	if updatedByUserID != "" {
		updatedBy = sql.NullString{String: updatedByUserID, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO crash_settings (key, value, updated_by, updated_at, created_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_by = $3, updated_at = NOW()`,
		key, value, updatedBy)
	return err
}

// ---------------------------------------------------------------------------
// Players
// ---------------------------------------------------------------------------

type PlayerRepo struct{ db *sql.DB }

func NewPlayerRepo(db *sql.DB) *PlayerRepo { return &PlayerRepo{db: db} }

func (r *PlayerRepo) FindByID(ctx context.Context, userID string) (*domain.PlayerInfo, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, COALESCE(full_name, ''), COALESCE(email, '') FROM users WHERE id = $1`, userID)
	p := &domain.PlayerInfo{}
	if err := row.Scan(&p.UserID, &p.FullName, &p.Email); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// FindByIDs resolves the given players in a single query.
func (r *PlayerRepo) FindByIDs(ctx context.Context, userIDs []string) (map[string]*domain.PlayerInfo, error) {
	out := make(map[string]*domain.PlayerInfo, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, COALESCE(full_name, ''), COALESCE(email, '') FROM users WHERE id = ANY($1::uuid[])`,
		pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p := &domain.PlayerInfo{}
		if err := rows.Scan(&p.UserID, &p.FullName, &p.Email); err != nil {
			return nil, err
		}
		out[p.UserID] = p
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Engine lock
// ---------------------------------------------------------------------------

const engineLockKey int64 = 0x6372_6173_685F_656E // "crash_en"

type AdvisoryLocker struct{ db *sql.DB }

func NewAdvisoryLocker(db *sql.DB) *AdvisoryLocker { return &AdvisoryLocker{db: db} }

func (l *AdvisoryLocker) Acquire(ctx context.Context, key string) (bool, func(), error) {
	conn, err := l.db.Conn(ctx)
	if err != nil {
		return false, nil, err
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, engineLockKey).Scan(&acquired); err != nil {
		_ = conn.Close()
		return false, nil, err
	}
	if !acquired {
		_ = conn.Close()
		return false, func() {}, nil
	}

	release := func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, engineLockKey)
		_ = conn.Close()
	}
	return true, release, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func isUniqueViolation(err error) bool {
	pqErr, ok := err.(*pq.Error)
	return ok && pqErr.Code == "23505"
}

func newID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
