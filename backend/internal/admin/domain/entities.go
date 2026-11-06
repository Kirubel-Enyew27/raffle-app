package domain

import "time"

type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Overview is the single cross-platform snapshot used by the admin landing
// screen. Every value is aggregated from persisted platform records; no
// client-side estimate or fabricated growth figure is returned.
type Overview struct {
	From string `json:"from"`
	To   string `json:"to"`

	TotalUsers         int64   `json:"total_users"`
	NewUsers           int64   `json:"new_users"`
	ActiveUsers        int64   `json:"active_users"`
	TotalMatches       int64   `json:"total_matches"`
	MatchesToday       int64   `json:"matches_today"`
	CompletedMatches   int64   `json:"completed_matches"`
	TiedMatches        int64   `json:"tied_matches"`
	ForfeitedMatches   int64   `json:"forfeited_matches"`
	CancelledMatches   int64   `json:"cancelled_matches"`
	LiveMatches        int64   `json:"live_matches"`
	LivePlayers        int64   `json:"live_players"`
	TotalDeposits      float64 `json:"total_deposits"`
	TotalWithdrawals   float64 `json:"total_withdrawals"`
	PendingDeposits    int64   `json:"pending_deposits"`
	PendingWithdrawals int64   `json:"pending_withdrawals"`
	PlatformRevenue    float64 `json:"platform_revenue"`
	P2PServiceFees     float64 `json:"p2p_service_fees"`
	CrashServiceFees   float64 `json:"crash_service_fees"`
	RaffleProfit       float64 `json:"raffle_profit"`

	// Comparisons against the immediately preceding window of equal length.
	// They are nil when the previous window had no base to compare against.
	NewUsersChange        *float64 `json:"new_users_change"`
	ActiveUsersChange     *float64 `json:"active_users_change"`
	TotalMatchesChange    *float64 `json:"total_matches_change"`
	DepositsChange        *float64 `json:"deposits_change"`
	WithdrawalsChange     *float64 `json:"withdrawals_change"`
	PlatformRevenueChange *float64 `json:"platform_revenue_change"`

	RegistrationTrend []RegistrationPoint `json:"registration_trend"`
	MatchTrend        []MatchPoint        `json:"match_trend"`
	FinanceTrend      []FinancePoint      `json:"finance_trend"`
	RevenueTrend      []RevenuePoint      `json:"revenue_trend"`
	Games             []GameSummary       `json:"games"`
}

type RegistrationPoint struct {
	Period string `json:"period"`
	Count  int64  `json:"count"`
}

type MatchPoint struct {
	Period    string `json:"period"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Free      int64  `json:"free"`
	Paid      int64  `json:"paid"`
}

type FinancePoint struct {
	Period      string  `json:"period"`
	Deposits    float64 `json:"deposits"`
	Withdrawals float64 `json:"withdrawals"`
}

type RevenuePoint struct {
	Period       string  `json:"period"`
	P2PFees      float64 `json:"p2p_fees"`
	CrashFees    float64 `json:"crash_fees"`
	RaffleProfit float64 `json:"raffle_profit"`
	Total        float64 `json:"total"`
}

type GameSummary struct {
	Slug             string           `json:"slug"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	TotalMatches     int64            `json:"total_matches"`
	ActiveMatches    int64            `json:"active_matches"`
	CompletedMatches int64            `json:"completed_matches"`
	UniquePlayers    int64            `json:"unique_players"`
	FreeMatches      int64            `json:"free_matches"`
	PaidMatches      int64            `json:"paid_matches"`
	Trend            []GameTrendPoint `json:"trend"`
}

type GameTrendPoint struct {
	Period  string `json:"period"`
	Matches int64  `json:"matches"`
}

type UserFilter struct {
	Search string
	Status string
	Sort   string
	Limit  int
	Offset int
}

type UserRow struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Phone           string     `json:"phone,omitempty"`
	FullName        string     `json:"full_name"`
	AvatarURL       string     `json:"avatar_url"`
	Role            string     `json:"role"`
	IsBanned        bool       `json:"is_banned"`
	CreatedAt       time.Time  `json:"created_at"`
	LastActiveAt    *time.Time `json:"last_active_at,omitempty"`
	IsPlaying       bool       `json:"is_playing"`
	WalletBalance   float64    `json:"wallet_balance"`
	ReservedBalance float64    `json:"reserved_balance"`
	MatchesPlayed   int64      `json:"matches_played"`
	Wins            int64      `json:"wins"`
	Losses          int64      `json:"losses"`
	Draws           int64      `json:"draws"`
	WinRate         float64    `json:"win_rate"`
}

type UserDetail struct {
	UserRow
	RecentMatches      []UserMatch       `json:"recent_matches"`
	RecentTransactions []UserTransaction `json:"recent_transactions"`
}

type UserMatch struct {
	ID              string     `json:"id"`
	GameSlug        string     `json:"game_slug"`
	Status          string     `json:"status"`
	CompetitionMode string     `json:"competition_mode"`
	Result          string     `json:"result"`
	StakeAmount     float64    `json:"stake_amount"`
	PrizePool       float64    `json:"prize_pool"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

type UserTransaction struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Amount    float64   `json:"amount"`
	Reference string    `json:"reference"`
	CreatedAt time.Time `json:"created_at"`
}

type MatchFilter struct {
	Search string
	Game   string
	Status string
	Mode   string
	Sort   string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

type MatchRow struct {
	ID              string        `json:"id"`
	GameSlug        string        `json:"game_slug"`
	GameTitle       string        `json:"game_title"`
	Status          string        `json:"status"`
	MatchType       string        `json:"match_type"`
	CompetitionMode string        `json:"competition_mode"`
	StakeAmount     float64       `json:"stake_amount"`
	PrizePool       float64       `json:"prize_pool"`
	ServiceFee      float64       `json:"service_fee"`
	WinnerUserID    string        `json:"winner_user_id,omitempty"`
	WinnerName      string        `json:"winner_name,omitempty"`
	DurationSeconds int64         `json:"duration_seconds"`
	FinancialStatus string        `json:"financial_status"`
	Players         []MatchPlayer `json:"players"`
	CreatedAt       time.Time     `json:"created_at"`
	StartedAt       *time.Time    `json:"started_at,omitempty"`
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
}

type MatchPlayer struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	PlayerIndex int    `json:"player_index"`
	Status      string `json:"status"`
	Score       int    `json:"score"`
}

type FinanceFilter struct {
	Search string
	Status string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

type DepositRow struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	UserEmail         string     `json:"user_email"`
	UserName          string     `json:"user_name"`
	Amount            float64    `json:"amount"`
	Currency          string     `json:"currency"`
	PaymentMethod     string     `json:"payment_method"`
	Status            string     `json:"status"`
	ExternalReference string     `json:"external_reference,omitempty"`
	ReceiptURL        string     `json:"receipt_url,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	CreditedAt        *time.Time `json:"credited_at,omitempty"`
}

type WithdrawalRow struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	UserName    string    `json:"user_name"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	Reference   string    `json:"reference"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type LiveSnapshot struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	LivePlayers     int64            `json:"live_players"`
	ActiveMatches   int64            `json:"active_matches"`
	ActiveCrashBets int64            `json:"active_crash_bets"`
	Matches         []MatchRow       `json:"matches"`
	CrashRounds     []LiveCrashRound `json:"crash_rounds"`
}

type LiveCrashRound struct {
	ID              string         `json:"id"`
	RoundNumber     int64          `json:"round_number"`
	State           string         `json:"state"`
	Players         int64          `json:"players"`
	TotalStake      float64        `json:"total_stake"`
	CrashMultiplier *float64       `json:"crash_multiplier,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	Bets            []LiveCrashBet `json:"bets"`
}

type LiveCrashBet struct {
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	AvatarURL   string  `json:"avatar_url"`
	StakeAmount float64 `json:"stake_amount"`
}
