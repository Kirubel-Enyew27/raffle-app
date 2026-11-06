package domain

import (
	"context"
	"time"
)

type AdminRepository interface {
	Overview(ctx context.Context, from, to time.Time, period string) (*Overview, error)
	Games(ctx context.Context, from, to time.Time, period string) ([]GameSummary, error)
	Users(ctx context.Context, filter UserFilter) ([]UserRow, int, error)
	UserDetail(ctx context.Context, id string) (*UserDetail, error)
	Matches(ctx context.Context, filter MatchFilter) ([]MatchRow, int, error)
	Deposits(ctx context.Context, filter FinanceFilter) ([]DepositRow, int, error)
	Withdrawals(ctx context.Context, filter FinanceFilter) ([]WithdrawalRow, int, error)
	Live(ctx context.Context) (*LiveSnapshot, error)
}
