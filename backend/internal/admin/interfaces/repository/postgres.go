package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/raffle-app/backend/internal/admin/domain"
)

type AdminRepo struct {
	db *sql.DB
}

func NewAdminRepo(db *sql.DB) *AdminRepo {
	return &AdminRepo{db: db}
}

func percentChange(current, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	value := (current - previous) / previous * 100
	return &value
}

func (r *AdminRepo) Overview(ctx context.Context, from, to time.Time, period string) (*domain.Overview, error) {
	previousFrom := from.Add(-to.Sub(from))
	overview := &domain.Overview{
		From: from.Format(time.DateOnly),
		To:   to.AddDate(0, 0, -1).Format(time.DateOnly),
	}

	var newUsersPrev, activeUsersPrev, matchesPrev int64
	var depositsPrev, withdrawalsPrev float64
	var p2pFeesPrev, crashFeesPrev float64
	var ticketRevenue, ticketRevenuePrev, prizePaid, prizePaidPrev float64

	err := r.db.QueryRowContext(ctx, `
SELECT
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL),
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2),
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= $3 AND created_at < $1),
  (SELECT COUNT(DISTINCT user_id) FROM wallet_transactions WHERE created_at >= $1 AND created_at < $2),
  (SELECT COUNT(DISTINCT user_id) FROM wallet_transactions WHERE created_at >= $3 AND created_at < $1),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND created_at >= $1 AND created_at < $2),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND created_at >= $3 AND created_at < $1),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND created_at >= CURRENT_DATE AND created_at < CURRENT_DATE + INTERVAL '1 day'),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND status = 'completed' AND completed_at >= $1 AND completed_at < $2),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND status = 'tied' AND completed_at >= $1 AND completed_at < $2),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND status = 'forfeited' AND completed_at >= $1 AND completed_at < $2),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND status = 'cancelled' AND completed_at >= $1 AND completed_at < $2),
  (SELECT COUNT(*) FROM p2p_matches WHERE game_slug NOT LIKE '%test%' AND status IN ('waiting','ready','in_progress')),
  (SELECT COUNT(DISTINCT user_id) FROM (
     SELECT p.user_id
       FROM p2p_match_players p
       JOIN p2p_matches m ON m.id = p.match_id
      WHERE m.game_slug NOT LIKE '%test%' AND m.status IN ('waiting','ready','in_progress')
     UNION ALL
     SELECT b.user_id
       FROM game_bets b
       JOIN game_rounds r ON r.id = b.round_id
      WHERE r.state IN ('WAITING','BETTING','RUNNING') AND b.status = 'active'
   ) live_users),
  (SELECT COALESCE(SUM(amount), 0) FROM wallet_transactions
    WHERE type IN ('deposit','sms_deposit') AND status = 'completed'
      AND created_at >= $1 AND created_at < $2),
  (SELECT COALESCE(SUM(amount), 0) FROM wallet_transactions
    WHERE type IN ('deposit','sms_deposit') AND status = 'completed'
      AND created_at >= $3 AND created_at < $1),
  (SELECT COALESCE(SUM(amount), 0) FROM wallet_transactions
    WHERE type = 'withdrawal' AND status = 'completed'
      AND created_at >= $1 AND created_at < $2),
  (SELECT COALESCE(SUM(amount), 0) FROM wallet_transactions
    WHERE type = 'withdrawal' AND status = 'completed'
      AND created_at >= $3 AND created_at < $1),
  (SELECT COUNT(*) FROM deposits WHERE status IN ('pending','verifying','manual_review')),
  (SELECT COUNT(*) FROM wallet_transactions WHERE type = 'withdrawal' AND status = 'pending'),
  (SELECT COALESCE(SUM(amount), 0) FROM wallet_transactions
    WHERE type = 'p2p_platform_fee' AND status = 'completed'
      AND created_at >= $1 AND created_at < $2),
  (SELECT COALESCE(SUM(amount), 0) FROM wallet_transactions
    WHERE type = 'p2p_platform_fee' AND status = 'completed'
      AND created_at >= $3 AND created_at < $1),
  (SELECT COALESCE(SUM(total_fee_cents), 0) / 100.0 FROM game_rounds
    WHERE state IN ('SETTLEMENT','NEXT_ROUND') AND settled_at >= $1 AND settled_at < $2),
  (SELECT COALESCE(SUM(total_fee_cents), 0) / 100.0 FROM game_rounds
    WHERE state IN ('SETTLEMENT','NEXT_ROUND') AND settled_at >= $3 AND settled_at < $1),
  (SELECT COALESCE(SUM(r.ticket_price * t.ticket_count), 0)
     FROM raffles r
     JOIN (SELECT raffle_id, COUNT(*) AS ticket_count FROM tickets GROUP BY raffle_id) t ON t.raffle_id = r.id
     JOIN (SELECT raffle_id, SUM(prize_amount) AS prize_total, MAX(payment_date) AS paid_at, BOOL_AND(prize_paid) AS all_paid
             FROM winners GROUP BY raffle_id) w ON w.raffle_id = r.id
    WHERE w.all_paid = TRUE AND w.paid_at >= $1 AND w.paid_at < $2
      AND EXISTS (SELECT 1 FROM raffle_draws d WHERE d.raffle_id = r.id AND d.status = 'completed')),
  (SELECT COALESCE(SUM(w.prize_total), 0)
     FROM raffles r
     JOIN (SELECT raffle_id, SUM(prize_amount) AS prize_total, MAX(payment_date) AS paid_at, BOOL_AND(prize_paid) AS all_paid
             FROM winners GROUP BY raffle_id) w ON w.raffle_id = r.id
    WHERE w.all_paid = TRUE AND w.paid_at >= $1 AND w.paid_at < $2
      AND EXISTS (SELECT 1 FROM raffle_draws d WHERE d.raffle_id = r.id AND d.status = 'completed')),
  (SELECT COALESCE(SUM(r.ticket_price * t.ticket_count), 0)
     FROM raffles r
     JOIN (SELECT raffle_id, COUNT(*) AS ticket_count FROM tickets GROUP BY raffle_id) t ON t.raffle_id = r.id
     JOIN (SELECT raffle_id, SUM(prize_amount) AS prize_total, MAX(payment_date) AS paid_at, BOOL_AND(prize_paid) AS all_paid
             FROM winners GROUP BY raffle_id) w ON w.raffle_id = r.id
    WHERE w.all_paid = TRUE AND w.paid_at >= $3 AND w.paid_at < $1
      AND EXISTS (SELECT 1 FROM raffle_draws d WHERE d.raffle_id = r.id AND d.status = 'completed')),
  (SELECT COALESCE(SUM(w.prize_total), 0)
     FROM raffles r
     JOIN (SELECT raffle_id, SUM(prize_amount) AS prize_total, MAX(payment_date) AS paid_at, BOOL_AND(prize_paid) AS all_paid
             FROM winners GROUP BY raffle_id) w ON w.raffle_id = r.id
    WHERE w.all_paid = TRUE AND w.paid_at >= $3 AND w.paid_at < $1
      AND EXISTS (SELECT 1 FROM raffle_draws d WHERE d.raffle_id = r.id AND d.status = 'completed'))
`, from, to, previousFrom).Scan(
		&overview.TotalUsers,
		&overview.NewUsers,
		&newUsersPrev,
		&overview.ActiveUsers,
		&activeUsersPrev,
		&overview.TotalMatches,
		&matchesPrev,
		&overview.MatchesToday,
		&overview.CompletedMatches,
		&overview.TiedMatches,
		&overview.ForfeitedMatches,
		&overview.CancelledMatches,
		&overview.LiveMatches,
		&overview.LivePlayers,
		&overview.TotalDeposits,
		&depositsPrev,
		&overview.TotalWithdrawals,
		&withdrawalsPrev,
		&overview.PendingDeposits,
		&overview.PendingWithdrawals,
		&overview.P2PServiceFees,
		&p2pFeesPrev,
		&overview.CrashServiceFees,
		&crashFeesPrev,
		&ticketRevenue,
		&prizePaid,
		&ticketRevenuePrev,
		&prizePaidPrev,
	)
	if err != nil {
		return nil, err
	}

	overview.RaffleProfit = ticketRevenue - prizePaid
	raffleProfitPrev := ticketRevenuePrev - prizePaidPrev
	revenuePrev := p2pFeesPrev + crashFeesPrev + raffleProfitPrev
	overview.PlatformRevenue = overview.P2PServiceFees + overview.CrashServiceFees + overview.RaffleProfit

	overview.NewUsersChange = percentChange(float64(overview.NewUsers), float64(newUsersPrev))
	overview.ActiveUsersChange = percentChange(float64(overview.ActiveUsers), float64(activeUsersPrev))
	overview.TotalMatchesChange = percentChange(float64(overview.TotalMatches), float64(matchesPrev))
	overview.DepositsChange = percentChange(overview.TotalDeposits, depositsPrev)
	overview.WithdrawalsChange = percentChange(overview.TotalWithdrawals, withdrawalsPrev)
	overview.PlatformRevenueChange = percentChange(overview.PlatformRevenue, revenuePrev)

	if overview.RegistrationTrend, err = r.registrationTrend(ctx, from, to, period); err != nil {
		return nil, err
	}
	if overview.MatchTrend, err = r.matchTrend(ctx, from, to, period); err != nil {
		return nil, err
	}
	if overview.FinanceTrend, err = r.financeTrend(ctx, from, to, period); err != nil {
		return nil, err
	}
	return overview, nil
}

func (r *AdminRepo) registrationTrend(ctx context.Context, from, to time.Time, period string) ([]domain.RegistrationPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT TO_CHAR(DATE_TRUNC($3::text, created_at), 'YYYY-MM-DD') AS bucket, COUNT(*)
  FROM users
 WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2
 GROUP BY bucket
 ORDER BY bucket ASC`, from, to, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]domain.RegistrationPoint, 0)
	for rows.Next() {
		var point domain.RegistrationPoint
		if err := rows.Scan(&point.Period, &point.Count); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (r *AdminRepo) matchTrend(ctx context.Context, from, to time.Time, period string) ([]domain.MatchPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  TO_CHAR(DATE_TRUNC($3::text, created_at), 'YYYY-MM-DD') AS bucket,
  COUNT(*),
  COUNT(*) FILTER (WHERE status IN ('completed','tied','forfeited')),
  COUNT(*) FILTER (WHERE competition_mode = 'fun'),
  COUNT(*) FILTER (WHERE competition_mode = 'earn')
FROM p2p_matches
WHERE game_slug NOT LIKE '%test%' AND created_at >= $1 AND created_at < $2
GROUP BY bucket
ORDER BY bucket ASC`, from, to, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]domain.MatchPoint, 0)
	for rows.Next() {
		var point domain.MatchPoint
		if err := rows.Scan(&point.Period, &point.Total, &point.Completed, &point.Free, &point.Paid); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (r *AdminRepo) financeTrend(ctx context.Context, from, to time.Time, period string) ([]domain.FinancePoint, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  TO_CHAR(DATE_TRUNC($3::text, created_at), 'YYYY-MM-DD') AS bucket,
  COALESCE(SUM(amount) FILTER (WHERE type IN ('deposit','sms_deposit') AND status = 'completed'), 0),
  COALESCE(SUM(amount) FILTER (WHERE type = 'withdrawal' AND status = 'completed'), 0)
FROM wallet_transactions
WHERE created_at >= $1 AND created_at < $2
GROUP BY bucket
ORDER BY bucket ASC`, from, to, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]domain.FinancePoint, 0)
	for rows.Next() {
		var point domain.FinancePoint
		if err := rows.Scan(&point.Period, &point.Deposits, &point.Withdrawals); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

type revenuePoint struct {
	Period      string
	P2PFees     float64
	CrashFees   float64
	RaffleProfit float64
	Total       float64
}

func (r *AdminRepo) revenueTrend(ctx context.Context, from, to time.Time, period string) ([]revenuePoint, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  TO_CHAR(DATE_TRUNC($3::text, bucket), 'YYYY-MM-DD') AS period,
  COALESCE(SUM(p2p_fee), 0),
  COALESCE(SUM(crash_fee), 0),
  COALESCE(SUM(raffle_profit), 0)
FROM (
  SELECT DATE_TRUNC($3::text, created_at) AS bucket,
         COALESCE(SUM(amount), 0) AS p2p_fee,
         0::numeric AS crash_fee,
         0::numeric AS raffle_profit
    FROM wallet_transactions
   WHERE type = 'p2p_platform_fee' AND status = 'completed'
     AND created_at >= $1 AND created_at < $2
   GROUP BY 1
  UNION ALL
  SELECT DATE_TRUNC($3::text, settled_at),
         0::numeric,
         COALESCE(SUM(total_fee_cents), 0) / 100.0,
         0::numeric
    FROM game_rounds
   WHERE state IN ('SETTLEMENT','NEXT_ROUND') AND settled_at >= $1 AND settled_at < $2
   GROUP BY 1
  UNION ALL
  SELECT DATE_TRUNC($3::text, w.paid_at),
         0::numeric,
         0::numeric,
         r.ticket_price * t.ticket_count - w.prize_total
    FROM (SELECT raffle_id, SUM(prize_amount) AS prize_total, MAX(payment_date) AS paid_at, BOOL_AND(prize_paid) AS all_paid
            FROM winners GROUP BY raffle_id) w
    JOIN raffles r ON r.id = w.raffle_id
    JOIN (SELECT raffle_id, COUNT(*) AS ticket_count FROM tickets GROUP BY raffle_id) t ON t.raffle_id = w.raffle_id
   WHERE w.all_paid = TRUE AND w.paid_at >= $1 AND w.paid_at < $2
     AND EXISTS (SELECT 1 FROM raffle_draws d WHERE d.raffle_id = w.raffle_id AND d.status = 'completed')
) revenue
GROUP BY bucket
ORDER BY bucket ASC`, from, to, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]revenuePoint, 0)
	for rows.Next() {
		var point revenuePoint
		if err := rows.Scan(&point.Period, &point.P2PFees, &point.CrashFees, &point.RaffleProfit); err != nil {
			return nil, err
		}
		point.Total = point.P2PFees + point.CrashFees + point.RaffleProfit
		points = append(points, point)
	}
	return points, rows.Err()
}

func (r *AdminRepo) Games(ctx context.Context, from, to time.Time, period string) ([]domain.GameSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  g.slug,
  g.title,
  g.description,
  COUNT(DISTINCT m.id),
  COUNT(DISTINCT m.id) FILTER (WHERE m.status IN ('waiting','ready','in_progress')),
  COUNT(DISTINCT m.id) FILTER (WHERE m.status IN ('completed','tied','forfeited')),
  COUNT(DISTINCT p.user_id),
  COUNT(DISTINCT m.id) FILTER (WHERE m.competition_mode = 'fun'),
  COUNT(DISTINCT m.id) FILTER (WHERE m.competition_mode = 'earn')
FROM p2p_games g
LEFT JOIN p2p_matches m
  ON m.game_slug = g.slug AND m.created_at >= $1 AND m.created_at < $2
LEFT JOIN p2p_match_players p ON p.match_id = m.id
WHERE g.slug NOT LIKE '%test%'
GROUP BY g.slug, g.title, g.description
ORDER BY g.title ASC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	games := make([]domain.GameSummary, 0, 5)
	for rows.Next() {
		var game domain.GameSummary
		if err := rows.Scan(
			&game.Slug, &game.Title, &game.Description,
			&game.TotalMatches, &game.ActiveMatches, &game.CompletedMatches,
			&game.UniquePlayers, &game.FreeMatches, &game.PaidMatches,
		); err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	trendRows, err := r.db.QueryContext(ctx, `
SELECT m.game_slug, TO_CHAR(DATE_TRUNC($3::text, m.created_at), 'YYYY-MM-DD'), COUNT(*)
FROM p2p_matches m
WHERE m.game_slug NOT LIKE '%test%' AND m.created_at >= $1 AND m.created_at < $2
GROUP BY m.game_slug, 2
ORDER BY 2 ASC`, from, to, period)
	if err != nil {
		return nil, err
	}
	defer trendRows.Close()

	trends := make(map[string][]domain.GameTrendPoint)
	for trendRows.Next() {
		var slug, bucket string
		var count int64
		if err := trendRows.Scan(&slug, &bucket, &count); err != nil {
			return nil, err
		}
		trends[slug] = append(trends[slug], domain.GameTrendPoint{Period: bucket, Matches: count})
	}
	if err := trendRows.Err(); err != nil {
		return nil, err
	}
	for i := range games {
		if trend, ok := trends[games[i].Slug]; ok {
			games[i].Trend = trend
		} else {
			games[i].Trend = []domain.GameTrendPoint{}
		}
	}

	var crash domain.GameSummary
	crash.Slug = "crash"
	crash.Title = "Crash"
	crash.Description = "P2P multiplier rounds with server-authoritative crash points."
	crash.Trend = []domain.GameTrendPoint{}
	err = r.db.QueryRowContext(ctx, `
SELECT
  COUNT(r.id),
  COUNT(r.id) FILTER (WHERE r.state IN ('WAITING','BETTING','RUNNING')),
  COUNT(r.id) FILTER (WHERE r.state IN ('SETTLEMENT','NEXT_ROUND')),
  COUNT(DISTINCT b.user_id)
FROM game_rounds r
LEFT JOIN game_bets b ON b.round_id = r.id
WHERE r.created_at >= $1 AND r.created_at < $2`, from, to).Scan(
		&crash.TotalMatches, &crash.ActiveMatches, &crash.CompletedMatches, &crash.UniquePlayers,
	)
	if err != nil {
		return nil, err
	}
	crash.PaidMatches = crash.TotalMatches

	crashTrendRows, err := r.db.QueryContext(ctx, `
SELECT TO_CHAR(DATE_TRUNC($3::text, created_at), 'YYYY-MM-DD'), COUNT(*)
FROM game_rounds
WHERE created_at >= $1 AND created_at < $2
GROUP BY 1
ORDER BY 1 ASC`, from, to, period)
	if err != nil {
		return nil, err
	}
	defer crashTrendRows.Close()
	for crashTrendRows.Next() {
		var point domain.GameTrendPoint
		if err := crashTrendRows.Scan(&point.Period, &point.Matches); err != nil {
			return nil, err
		}
		crash.Trend = append(crash.Trend, point)
	}
	if err := crashTrendRows.Err(); err != nil {
		return nil, err
	}
	games = append(games, crash)

	sort.SliceStable(games, func(i, j int) bool {
		return games[i].TotalMatches > games[j].TotalMatches
	})
	return games, nil
}

func (r *AdminRepo) Live(ctx context.Context) (*domain.LiveSnapshot, error) {
	matches, err := queryMatchRows(ctx, r.db, `
`+matchSelect+`
WHERE m.game_slug NOT LIKE '%test%'
  AND m.status IN ('waiting','ready','in_progress')
GROUP BY m.id, g.title
ORDER BY m.created_at DESC
LIMIT 50`)
	if err != nil {
		return nil, err
	}

	snapshot := &domain.LiveSnapshot{
		GeneratedAt: time.Now().UTC(),
		Matches:     matches,
		CrashRounds: []domain.LiveCrashRound{},
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT
  r.id,
  r.round_number,
  r.state,
  COUNT(*) FILTER (WHERE b.status = 'active'),
  COALESCE(SUM(b.stake_amount) FILTER (WHERE b.status = 'active'), 0),
  r.crash_multiplier_bp / 10000.0,
  r.created_at,
  COALESCE(json_agg(json_build_object(
    'user_id', b.user_id,
    'display_name', COALESCE(NULLIF(u.full_name, ''), u.email, 'Player'),
    'avatar_url', COALESCE(u.avatar_url, ''),
    'stake_amount', b.stake_amount
  ) ORDER BY b.created_at) FILTER (WHERE b.id IS NOT NULL AND b.status = 'active'), '[]'::json)
FROM game_rounds r
LEFT JOIN game_bets b ON b.round_id = r.id
LEFT JOIN users u ON u.id = b.user_id
WHERE r.state IN ('WAITING','BETTING','RUNNING')
GROUP BY r.id
ORDER BY r.created_at DESC
LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var round domain.LiveCrashRound
		var raw []byte
		var multiplier sql.NullFloat64
		if err := rows.Scan(
			&round.ID, &round.RoundNumber, &round.State, &round.Players,
			&round.TotalStake, &multiplier, &round.CreatedAt, &raw,
		); err != nil {
			return nil, err
		}
		if multiplier.Valid {
			value := multiplier.Float64
			round.CrashMultiplier = &value
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &round.Bets)
		}
		if round.Bets == nil {
			round.Bets = []domain.LiveCrashBet{}
		}
		snapshot.CrashRounds = append(snapshot.CrashRounds, round)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// The match and round arrays are capped for display. Aggregate the live
	// totals over every active session so the headline counts are not silently
	// truncated to the first page of cards.
	if err := r.db.QueryRowContext(ctx, `
SELECT
  (SELECT COUNT(*) FROM p2p_matches
    WHERE game_slug NOT LIKE '%test%' AND status IN ('waiting','ready','in_progress')),
  (SELECT COUNT(*) FROM game_bets b
     JOIN game_rounds cr ON cr.id = b.round_id
    WHERE cr.state IN ('WAITING','BETTING','RUNNING') AND b.status = 'active'),
  (SELECT COUNT(DISTINCT user_id) FROM (
     SELECT p.user_id
       FROM p2p_match_players p
       JOIN p2p_matches m ON m.id = p.match_id
      WHERE m.game_slug NOT LIKE '%test%' AND m.status IN ('waiting','ready','in_progress')
     UNION ALL
     SELECT b.user_id
       FROM game_bets b
       JOIN game_rounds cr ON cr.id = b.round_id
      WHERE cr.state IN ('WAITING','BETTING','RUNNING') AND b.status = 'active'
   ) live_users)`).Scan(
		&snapshot.ActiveMatches, &snapshot.ActiveCrashBets, &snapshot.LivePlayers,
	); err != nil {
		return nil, err
	}
	return snapshot, nil
}
