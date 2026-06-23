package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/raffle-app/backend/internal/reporting/domain"
)

type ReportRepo struct {
	db *sql.DB
}

func NewReportRepo(db *sql.DB) *ReportRepo {
	return &ReportRepo{db: db}
}

// truncExpr maps a Period to a Postgres DATE_TRUNC expression.
func truncExpr(period domain.Period) string {
	switch period {
	case domain.PeriodWeekly:
		return "week"
	case domain.PeriodMonthly:
		return "month"
	default:
		return "day"
	}
}

// Revenue uses a single query with conditional aggregation over wallet_transactions
// joined with ticket revenue from tickets × raffles.
//
// Bucketing: DATE_TRUNC so the DB does one sequential scan per table.
// Count: window function avoids a second round-trip.
func (r *ReportRepo) Revenue(ctx context.Context, period domain.Period, f domain.Filter) ([]domain.RevenueRow, int, error) {
	trunc := truncExpr(period)
	// One CTE per source; final join aligns buckets.
	query := fmt.Sprintf(`
WITH wallet_buckets AS (
  SELECT
    DATE_TRUNC('%s', created_at)            AS bucket,
    SUM(CASE WHEN type = 'deposit'    THEN amount ELSE 0 END) AS deposits,
    SUM(CASE WHEN type = 'withdrawal' THEN amount ELSE 0 END) AS withdrawals
  FROM wallet_transactions
  WHERE created_at BETWEEN $1 AND $2
  GROUP BY bucket
),
ticket_buckets AS (
  SELECT
    DATE_TRUNC('%s', t.created_at) AS bucket,
    SUM(r.ticket_price)            AS ticket_revenue
  FROM tickets t
  JOIN raffles r ON r.id = t.raffle_id
  WHERE t.created_at BETWEEN $1 AND $2
  GROUP BY bucket
),
prize_buckets AS (
  SELECT
    DATE_TRUNC('%s', payment_date) AS bucket,
    SUM(prize_amount)              AS prize_paid
  FROM winners
  WHERE prize_paid = TRUE AND payment_date BETWEEN $1 AND $2
  GROUP BY bucket
),
all_buckets AS (
  SELECT bucket FROM wallet_buckets
  UNION
  SELECT bucket FROM ticket_buckets
  UNION
  SELECT bucket FROM prize_buckets
),
result AS (
  SELECT
    ab.bucket,
    COALESCE(tb.ticket_revenue, 0) AS ticket_revenue,
    COALESCE(wb.deposits,       0) AS deposit_volume,
    COALESCE(wb.withdrawals,    0) AS withdraw_volume,
    COALESCE(pb.prize_paid,     0) AS prize_paid,
    COUNT(*) OVER ()               AS total_count
  FROM all_buckets ab
  LEFT JOIN wallet_buckets wb ON wb.bucket = ab.bucket
  LEFT JOIN ticket_buckets  tb ON tb.bucket = ab.bucket
  LEFT JOIN prize_buckets   pb ON pb.bucket = ab.bucket
)
SELECT
  TO_CHAR(bucket, 'YYYY-MM-DD') AS period,
  ticket_revenue,
  deposit_volume,
  withdraw_volume,
  prize_paid,
  ticket_revenue - prize_paid    AS profit,
  total_count
FROM result
ORDER BY bucket DESC
LIMIT $3 OFFSET $4
`, trunc, trunc, trunc)

	rows, err := r.db.QueryContext(ctx, query, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []domain.RevenueRow
	total := 0
	for rows.Next() {
		var row domain.RevenueRow
		if err := rows.Scan(
			&row.Period, &row.TicketRevenue, &row.DepositVolume,
			&row.WithdrawVolume, &row.PrizePaid, &row.Profit, &total,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	return result, total, nil
}

func (r *ReportRepo) TicketSales(ctx context.Context, period domain.Period, f domain.Filter) ([]domain.TicketSalesRow, int, error) {
	trunc := truncExpr(period)
	query := fmt.Sprintf(`
WITH buckets AS (
  SELECT
    DATE_TRUNC('%s', t.created_at) AS bucket,
    COUNT(t.id)                    AS tickets_sold,
    COUNT(DISTINCT t.raffle_id)    AS raffles_held,
    COUNT(*) OVER ()               AS total_count
  FROM tickets t
  WHERE t.created_at BETWEEN $1 AND $2
  GROUP BY bucket
)
SELECT
  TO_CHAR(bucket, 'YYYY-MM-DD') AS period,
  tickets_sold,
  raffles_held,
  total_count
FROM buckets
ORDER BY bucket DESC
LIMIT $3 OFFSET $4
`, trunc)

	rows, err := r.db.QueryContext(ctx, query, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []domain.TicketSalesRow
	total := 0
	for rows.Next() {
		var row domain.TicketSalesRow
		if err := rows.Scan(&row.Period, &row.TicketsSold, &row.RafflesHeld, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	return result, total, nil
}

func (r *ReportRepo) ActiveUsers(ctx context.Context, period domain.Period, f domain.Filter) ([]domain.ActiveUsersRow, int, error) {
	trunc := truncExpr(period)
	// A user is "active" if they had any wallet transaction in the bucket.
	query := fmt.Sprintf(`
WITH buckets AS (
  SELECT
    DATE_TRUNC('%s', created_at) AS bucket,
    COUNT(DISTINCT user_id)       AS active_users,
    COUNT(*) OVER ()              AS total_count
  FROM wallet_transactions
  WHERE created_at BETWEEN $1 AND $2
  GROUP BY bucket
)
SELECT
  TO_CHAR(bucket, 'YYYY-MM-DD') AS period,
  active_users,
  total_count
FROM buckets
ORDER BY bucket DESC
LIMIT $3 OFFSET $4
`, trunc)

	rows, err := r.db.QueryContext(ctx, query, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []domain.ActiveUsersRow
	total := 0
	for rows.Next() {
		var row domain.ActiveUsersRow
		if err := rows.Scan(&row.Period, &row.ActiveUsers, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	return result, total, nil
}

func (r *ReportRepo) WinnerSummary(ctx context.Context, f domain.Filter) ([]domain.WinnerSummaryRow, int, error) {
	query := `
SELECT
  w.id,
  u.email,
  rf.title,
  w.prize_amount,
  w.prize_paid,
  w.payment_date,
  w.payment_reference,
  w.created_at,
  COUNT(*) OVER () AS total_count
FROM winners w
JOIN users   u  ON u.id  = w.user_id
JOIN raffles rf ON rf.id = w.raffle_id
WHERE w.created_at BETWEEN $1 AND $2
ORDER BY w.created_at DESC
LIMIT $3 OFFSET $4`

	rows, err := r.db.QueryContext(ctx, query, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []domain.WinnerSummaryRow
	total := 0
	for rows.Next() {
		var row domain.WinnerSummaryRow
		if err := rows.Scan(
			&row.WinnerID, &row.UserEmail, &row.RaffleTitle,
			&row.PrizeAmount, &row.PrizePaid, &row.PaymentDate,
			&row.PaymentReference, &row.CreatedAt, &total,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	return result, total, nil
}

func (r *ReportRepo) ProfitSummary(ctx context.Context, f domain.Filter) (*domain.ProfitSummary, error) {
	query := `
SELECT
  COALESCE(SUM(CASE WHEN wt.type = 'deposit'    THEN wt.amount ELSE 0 END), 0) AS deposit_volume,
  COALESCE(SUM(CASE WHEN wt.type = 'withdrawal' THEN wt.amount ELSE 0 END), 0) AS withdraw_volume,
  (SELECT COALESCE(SUM(r.ticket_price), 0)
     FROM tickets t JOIN raffles r ON r.id = t.raffle_id
    WHERE t.created_at BETWEEN $1 AND $2)                                        AS ticket_revenue,
  (SELECT COALESCE(SUM(prize_amount), 0)
     FROM winners WHERE prize_paid = TRUE AND payment_date BETWEEN $1 AND $2)    AS prize_paid,
  (SELECT COUNT(*) FROM tickets WHERE created_at BETWEEN $1 AND $2)              AS tickets_sold,
  (SELECT COUNT(*) FROM winners WHERE created_at BETWEEN $1 AND $2)              AS total_winners,
  (SELECT COUNT(DISTINCT user_id)
     FROM wallet_transactions WHERE created_at BETWEEN $1 AND $2)                AS active_users
FROM wallet_transactions wt
WHERE wt.created_at BETWEEN $1 AND $2`

	s := &domain.ProfitSummary{}
	err := r.db.QueryRowContext(ctx, query, f.From, f.To).Scan(
		&s.TotalDepositVolume,
		&s.TotalWithdrawVolume,
		&s.TotalTicketRevenue,
		&s.TotalPrizePaid,
		&s.TotalTicketsSold,
		&s.TotalWinners,
		&s.TotalActiveUsers,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	s.NetProfit = s.TotalTicketRevenue - s.TotalPrizePaid
	return s, nil
}
