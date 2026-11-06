package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/raffle-app/backend/internal/admin/domain"
)

func financeFilterClause(filter domain.FinanceFilter, dateColumn, referenceColumn string) (string, []interface{}) {
	where := ""
	args := make([]interface{}, 0)
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		index := len(args)
		where += fmt.Sprintf(` AND (
			t.id::text ILIKE $%d OR t.user_id::text ILIKE $%d
			OR u.email ILIKE $%d OR COALESCE(u.full_name, '') ILIKE $%d
			OR COALESCE(%s, '') ILIKE $%d
		)`, index, index, index, index, referenceColumn, index)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND t.status = $%d", len(args))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		where += fmt.Sprintf(" AND %s >= $%d", dateColumn, len(args))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		where += fmt.Sprintf(" AND %s < $%d", dateColumn, len(args))
	}
	return where, args
}

func (r *AdminRepo) Deposits(ctx context.Context, filter domain.FinanceFilter) ([]domain.DepositRow, int, error) {
	where, args := financeFilterClause(filter, "t.created_at", "t.external_reference")
	queryArgs := append(append([]interface{}{}, args...), filter.Limit, filter.Offset)
	query := `
SELECT
  t.id, t.user_id, u.email, COALESCE(u.full_name, ''),
  t.amount, t.currency, t.payment_method, t.status,
  COALESCE(t.external_reference, ''), COALESCE(t.receipt_url, ''),
  t.created_at, t.credited_at, COUNT(*) OVER ()
FROM deposits t
JOIN users u ON u.id = t.user_id
WHERE 1=1` + where + `
ORDER BY t.created_at DESC
LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	deposits := make([]domain.DepositRow, 0)
	total := 0
	for rows.Next() {
		var deposit domain.DepositRow
		if err := rows.Scan(
			&deposit.ID, &deposit.UserID, &deposit.UserEmail, &deposit.UserName,
			&deposit.Amount, &deposit.Currency, &deposit.PaymentMethod, &deposit.Status,
			&deposit.ExternalReference, &deposit.ReceiptURL,
			&deposit.CreatedAt, &deposit.CreditedAt, &total,
		); err != nil {
			return nil, 0, err
		}
		deposits = append(deposits, deposit)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return deposits, total, nil
}

func (r *AdminRepo) Withdrawals(ctx context.Context, filter domain.FinanceFilter) ([]domain.WithdrawalRow, int, error) {
	where, args := financeFilterClause(filter, "t.created_at", "t.reference")
	queryArgs := append(append([]interface{}{}, args...), filter.Limit, filter.Offset)
	query := `
SELECT
  t.id, t.user_id, u.email, COALESCE(u.full_name, ''),
  t.amount, COALESCE(w.currency, 'ETB'), t.status,
  COALESCE(t.reference, ''), COALESCE(t.description, ''),
  t.created_at, t.updated_at, COUNT(*) OVER ()
FROM wallet_transactions t
JOIN users u ON u.id = t.user_id
LEFT JOIN wallets w ON w.id = t.wallet_id
WHERE t.type = 'withdrawal'` + where + `
ORDER BY t.created_at DESC
LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	withdrawals := make([]domain.WithdrawalRow, 0)
	total := 0
	for rows.Next() {
		var withdrawal domain.WithdrawalRow
		if err := rows.Scan(
			&withdrawal.ID, &withdrawal.UserID, &withdrawal.UserEmail, &withdrawal.UserName,
			&withdrawal.Amount, &withdrawal.Currency, &withdrawal.Status,
			&withdrawal.Reference, &withdrawal.Description,
			&withdrawal.CreatedAt, &withdrawal.UpdatedAt, &total,
		); err != nil {
			return nil, 0, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return withdrawals, total, nil
}
