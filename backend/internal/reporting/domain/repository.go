package domain

import "context"

type ReportRepository interface {
	// Revenue returns bucketed revenue/volume data for the given period granularity.
	Revenue(ctx context.Context, period Period, f Filter) ([]RevenueRow, int, error)

	// TicketSales returns bucketed ticket-sales data.
	TicketSales(ctx context.Context, period Period, f Filter) ([]TicketSalesRow, int, error)

	// ActiveUsers returns bucketed distinct-user counts.
	ActiveUsers(ctx context.Context, period Period, f Filter) ([]ActiveUsersRow, int, error)

	// WinnerSummary returns a paginated list of winners enriched with user/raffle info.
	WinnerSummary(ctx context.Context, f Filter) ([]WinnerSummaryRow, int, error)

	// ProfitSummary returns a single aggregate for the filter window.
	ProfitSummary(ctx context.Context, f Filter) (*ProfitSummary, error)
}
