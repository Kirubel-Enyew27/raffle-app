package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/raffle-app/backend/pkg/money"
)

var (
	ErrGameNotFound        = errors.New("game not found")
	ErrMatchNotFound       = errors.New("match not found")
	ErrMatchFull           = errors.New("match is already full")
	ErrMatchNotJoinable    = errors.New("match is not open for joining")
	ErrNotYourTurn         = errors.New("not your turn")
	ErrTurnTimeout         = errors.New("turn deadline has passed")
	ErrInvalidMove         = errors.New("invalid move")
	ErrStaleMove           = errors.New("stale or duplicate move request")
	ErrMatchNotInProgress  = errors.New("match is not in progress")
	ErrMatchNotFinished    = errors.New("match is not finished")
	ErrAlreadyJoined       = errors.New("already joined this match")
	ErrCannotJoinOwnMatch  = errors.New("cannot join your own match")
	ErrInvitationExpired   = errors.New("invitation has expired")
	ErrNotParticipant      = errors.New("you are not a participant in this match")
	ErrUnsupportedGame     = errors.New("this game is not supported by the match engine")
	ErrInsufficientStake   = errors.New("insufficient wallet balance for match stake")
	ErrInvalidStake        = errors.New("stake amount is outside allowed limits")
	ErrPaidMatchesDisabled = errors.New("compete to earn is currently unavailable; compete for fun is always available")
	ErrMatchAlreadyEnded   = errors.New("match has already ended")
	ErrInvalidMode         = errors.New("invalid competition mode: choose free play or compete to earn")
	ErrPaidNotAuthorized   = errors.New("compete to earn is currently unavailable")
	ErrInvalidSetting      = errors.New("invalid or unsupported P2P setting")
	ErrSettlementFinalized = errors.New("match has already been settled")
)

const (
	StatusWaiting    = "waiting"
	StatusReady      = "ready"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"
	StatusTied       = "tied"
	StatusForfeited  = "forfeited"

	TypePublicQueue = "public_queue"
	TypePrivateRoom = "private_room"

	// CompetitionMode is a match-level property. Rules engines never change
	// based on it: free and staked play share the same authoritative game code.
	ModeFun  = "fun"
	ModeEarn = "earn"

	// Settings keys read by the staked-play configuration and admin screens.
	SettingPaidEnabled      = "paid_matches_enabled"
	SettingDefaultFeeBP     = "default_service_fee_bp"
	SettingMinStakeCents    = "min_stake_cents"
	SettingMaxStakeCents    = "max_stake_cents"
	SettingStakeIncrement   = "stake_increment_cents"
	SettingReconnectSeconds = "reconnect_grace_seconds"
	SettingIdleTimeout      = "idle_room_timeout_seconds"
	SettingTurnSeconds      = "turn_timer_default_seconds"

	PlayerStatusReserved     = "reserved"
	PlayerStatusReady        = "ready"
	PlayerStatusActive       = "active"
	PlayerStatusForfeited    = "forfeited"
	PlayerStatusDisconnected = "disconnected"
)

type Game struct {
	Slug               string                 `json:"slug"`
	Title              string                 `json:"title"`
	Description        string                 `json:"description"`
	MinPlayers         int                    `json:"min_players"`
	MaxPlayers         int                    `json:"max_players"`
	IsActive           bool                   `json:"is_active"`
	MinStakeCents      money.Cents            `json:"min_stake_cents"`
	MaxStakeCents      money.Cents            `json:"max_stake_cents"`
	DefaultFeeBP       int                    `json:"default_fee_bp"`
	DefaultTurnSeconds int                    `json:"default_turn_seconds"`
	RulesJSON          map[string]interface{} `json:"rules_json"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

type Match struct {
	ID                string                 `json:"id"`
	GameSlug          string                 `json:"game_slug"`
	Status            string                 `json:"status"`
	MatchType         string                 `json:"match_type"`
	CompetitionMode   string                 `json:"competition_mode"`
	InviteCode        string                 `json:"invite_code,omitempty"`
	StakeCents        money.Cents            `json:"stake_cents"`
	StakeAmount       float64                `json:"stake_amount"`
	PrizePoolCents    money.Cents            `json:"prize_pool_cents"`
	ServiceFeeBP      int                    `json:"service_fee_bp"`
	TotalFeeCents     money.Cents            `json:"total_fee_cents"`
	WinnerUserID      *string                `json:"winner_user_id,omitempty"`
	CurrentTurnUserID *string                `json:"current_turn_user_id,omitempty"`
	TurnDeadline      *time.Time             `json:"turn_deadline,omitempty"`
	CurrentRound      int                    `json:"current_round"`
	MaxRounds         int                    `json:"max_rounds"`
	Scores            map[string]int         `json:"scores"`
	BoardState        map[string]interface{} `json:"board_state"`
	CreatedBy         string                 `json:"created_by"`
	// Rematch handshake: who asked for a return game, and the match that the
	// two players agreed to play next.
	RematchRequestedBy *string `json:"rematch_requested_by,omitempty"`
	RematchMatchID     *string `json:"rematch_match_id,omitempty"`
	// DisconnectUserID is the user_id of the player who has temporarily
	// disconnected. Cleared when they reconnect or the match is forfeited.
	DisconnectUserID *string `json:"disconnect_user_id,omitempty"`
	// DisconnectDeadline is the server-authoritative deadline for the grace
	// period. If the player has not reconnected by this time, the opponent wins.
	DisconnectDeadline *time.Time     `json:"disconnect_deadline,omitempty"`
	Players            []*MatchPlayer `json:"players,omitempty"`
	StartedAt          *time.Time     `json:"started_at,omitempty"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	StateVersion       int64          `json:"state_version"`
}

// Finished reports whether the match reached a terminal state. Only finished
// matches can be rematched — a live one would abandon a game in progress.
func (m *Match) Finished() bool {
	switch m.Status {
	case StatusCompleted, StatusTied, StatusForfeited, StatusCancelled:
		return true
	default:
		return false
	}
}

type MatchPlayer struct {
	ID          string     `json:"id"`
	MatchID     string     `json:"match_id"`
	UserID      string     `json:"user_id"`
	DisplayName string     `json:"display_name,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	PlayerIndex int        `json:"player_index"`
	Symbol      string     `json:"symbol"`
	Status      string     `json:"status"`
	Score       int        `json:"score"`
	DebitTxID   *string    `json:"debit_tx_id,omitempty"`
	CreditTxID  *string    `json:"credit_tx_id,omitempty"`
	FeeTxID     *string    `json:"fee_tx_id,omitempty"`
	JoinedAt    time.Time  `json:"joined_at"`
	ReadyAt     *time.Time `json:"ready_at,omitempty"`
}

type MatchMove struct {
	ID            string                 `json:"id"`
	MatchID       string                 `json:"match_id"`
	RoundNumber   int                    `json:"round_number"`
	MoveNumber    int                    `json:"move_number"`
	UserID        string                 `json:"user_id"`
	Action        map[string]interface{} `json:"action"`
	BoardSnapshot map[string]interface{} `json:"board_snapshot"`
	CreatedAt     time.Time              `json:"created_at"`
}

type SettlementResult struct {
	MatchID        string      `json:"match_id"`
	WinnerUserID   string      `json:"winner_user_id"`
	PrizePoolCents money.Cents `json:"prize_pool_cents"`
	ServiceFeeBP   int         `json:"service_fee_bp"`
	FeeCents       money.Cents `json:"fee_cents"`
	NetPayoutCents money.Cents `json:"net_payout_cents"`
	IsTie          bool        `json:"is_tie"`
}

// NormalizeCompetitionMode resolves the requested mode, defaulting to free play
// and inferring the mode from the stake for older clients that only send a stake.
func NormalizeCompetitionMode(mode string, stakeCents money.Cents) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "":
		if stakeCents > 0 {
			return ModeEarn, nil
		}
		return ModeFun, nil
	case ModeFun:
		if stakeCents != 0 {
			return "", ErrInvalidMode
		}
		return ModeFun, nil
	case ModeEarn:
		if stakeCents <= 0 {
			return "", ErrInvalidStake
		}
		return ModeEarn, nil
	default:
		return "", ErrInvalidMode
	}
}

// ValidateStake enforces the configured limits and increment for a paid stake.
// Returns nil when the stake is playable.
func ValidateStake(stakeCents, minCents, maxCents, incrementCents money.Cents) error {
	if stakeCents <= 0 {
		return ErrInvalidStake
	}
	if stakeCents < minCents || stakeCents > maxCents {
		return ErrInvalidStake
	}
	if incrementCents > 0 && stakeCents%incrementCents != 0 {
		return ErrInvalidStake
	}
	return nil
}

// LedgerEntry is an immutable wallet ledger row attached to a match. Admin
// views use it to audit reservations, refunds, payouts and platform fees.
type LedgerEntry struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Amount      money.Cents            `json:"amount_cents"`
	Reference   string                 `json:"reference"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// MatchAdminFilter scopes the paid-match oversight list.
type MatchAdminFilter struct {
	// Status optionally limits the match status. "needs_attention" selects
	// terminal paid matches whose ledger never completed or refunded.
	Status string
	Game   string
	Limit  int
	Offset int
}

// CalculateSettlement computes exact fee and net payout using integer minor units.
// Platform fee is deducted from the gross prize pool:
// fee = floor(prize_pool * service_fee_bp / 10000)
// net_payout = prize_pool - fee
func CalculateSettlement(prizePoolCents money.Cents, serviceFeeBP int, isTie bool) SettlementResult {
	if isTie || prizePoolCents <= 0 {
		return SettlementResult{
			PrizePoolCents: prizePoolCents,
			ServiceFeeBP:   0,
			FeeCents:       0,
			NetPayoutCents: 0,
			IsTie:          true,
		}
	}

	// Clamp the configured fee so a misconfigured setting can never charge more
	// than the pool or produce a negative payout.
	if serviceFeeBP < 0 {
		serviceFeeBP = 0
	}
	if serviceFeeBP > 10000 {
		serviceFeeBP = 10000
	}

	feeCents := money.Cents(int64(prizePoolCents) * int64(serviceFeeBP) / 10000)
	netPayout := prizePoolCents - feeCents

	return SettlementResult{
		PrizePoolCents: prizePoolCents,
		ServiceFeeBP:   serviceFeeBP,
		FeeCents:       feeCents,
		NetPayoutCents: netPayout,
		IsTie:          false,
	}
}

func MarshalJSONMap(m map[string]interface{}) (string, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type LeaderboardPeriod string

const (
	PeriodAllTime LeaderboardPeriod = "all_time"
	PeriodDaily   LeaderboardPeriod = "daily"
	PeriodWeekly  LeaderboardPeriod = "weekly"
	PeriodMonthly LeaderboardPeriod = "monthly"
)

type LeaderboardSortBy string

const (
	SortByWins      LeaderboardSortBy = "wins"
	SortByWinStreak LeaderboardSortBy = "win_streak"
	SortByWinRate   LeaderboardSortBy = "win_rate"
)

type LeaderboardFilter struct {
	Period LeaderboardPeriod
	Game   *string
	SortBy LeaderboardSortBy
	Page   int
	Limit  int
	UserID *string
}

type LeaderboardPlayer struct {
	Rank             int     `json:"rank"`
	PlayerID         string  `json:"player_id"`
	DisplayName      string  `json:"display_name"`
	AvatarURL        *string `json:"avatar_url"`
	MatchesPlayed    int     `json:"matches_played"`
	Wins             int     `json:"wins"`
	Losses           int     `json:"losses"`
	Draws            int     `json:"draws"`
	WinRate          float64 `json:"win_rate"`
	CurrentWinStreak int     `json:"current_win_streak"`
	LongestWinStreak int     `json:"longest_win_streak"`
}

type LeaderboardPagination struct {
	Page         int `json:"page"`
	Limit        int `json:"limit"`
	TotalPlayers int `json:"total_players"`
	TotalPages   int `json:"total_pages"`
}

type LeaderboardResult struct {
	Period        string                 `json:"period"`
	Game          *string                `json:"game"`
	SortBy        string                 `json:"sort_by"`
	Players       []*LeaderboardPlayer   `json:"players"`
	CurrentPlayer *LeaderboardPlayer     `json:"current_player,omitempty"`
	Pagination    *LeaderboardPagination `json:"pagination"`
}

// StreakResult holds current and longest streak
type StreakResult struct {
	CurrentStreak int
	LongestStreak int
}

// CalculateStreaks calculates current and longest consecutive win streak given outcomes in chronological order (oldest to newest).
// true represents a win. false represents a loss, draw, or forfeiture against the player.
func CalculateStreaks(outcomes []bool) StreakResult {
	current := 0
	longest := 0

	for _, won := range outcomes {
		if won {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}

	return StreakResult{
		CurrentStreak: current,
		LongestStreak: longest,
	}
}
