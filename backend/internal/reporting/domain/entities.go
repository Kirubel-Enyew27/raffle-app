package domain

import "time"

// Period granularity for revenue reports.
type Period string

const (
	PeriodDaily   Period = "daily"
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
)

// Filter is the shared date-range + pagination filter for all reports.
type Filter struct {
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
}

// RevenueRow is one bucket in a time-series revenue report.
type RevenueRow struct {
	Period        string  `json:"period"`
	TicketRevenue float64 `json:"ticket_revenue"`
	DepositVolume float64 `json:"deposit_volume"`
	WithdrawVolume float64 `json:"withdraw_volume"`
	PrizePaid     float64 `json:"prize_paid"`
	Profit        float64 `json:"profit"` // ticket_revenue - prize_paid
}

// TicketSalesRow is one bucket of ticket sales over time.
type TicketSalesRow struct {
	Period      string `json:"period"`
	TicketsSold int    `json:"tickets_sold"`
	RafflesHeld int    `json:"raffles_held"`
}

// ActiveUsersRow counts distinct users who transacted in a period.
type ActiveUsersRow struct {
	Period      string `json:"period"`
	ActiveUsers int    `json:"active_users"`
}

// WinnerSummaryRow is one winner entry for the winner summary report.
type WinnerSummaryRow struct {
	WinnerID         string     `json:"winner_id"`
	UserEmail        string     `json:"user_email"`
	RaffleTitle      string     `json:"raffle_title"`
	PrizeAmount      float64    `json:"prize_amount"`
	PrizePaid        bool       `json:"prize_paid"`
	PaymentDate      *time.Time `json:"payment_date,omitempty"`
	PaymentReference string     `json:"payment_reference,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ProfitSummary is a single aggregate covering the full filter window.
type ProfitSummary struct {
	TotalTicketRevenue  float64 `json:"total_ticket_revenue"`
	TotalDepositVolume  float64 `json:"total_deposit_volume"`
	TotalWithdrawVolume float64 `json:"total_withdraw_volume"`
	TotalPrizePaid      float64 `json:"total_prize_paid"`
	NetProfit           float64 `json:"net_profit"`
	TotalTicketsSold    int     `json:"total_tickets_sold"`
	TotalWinners        int     `json:"total_winners"`
	TotalActiveUsers    int     `json:"total_active_users"`
}

// Page wraps a paginated result set.
type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
