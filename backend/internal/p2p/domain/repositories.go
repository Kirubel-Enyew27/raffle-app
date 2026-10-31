package domain

import (
	"context"
	"database/sql"
	"time"

	"github.com/raffle-app/backend/pkg/money"
)

type GameRepository interface {
	ListActive(ctx context.Context) ([]*Game, error)
	FindBySlug(ctx context.Context, slug string) (*Game, error)
}

type MatchRepository interface {
	BeginTx(ctx context.Context) (*sql.Tx, error)
	Create(ctx context.Context, match *Match) error
	CreateTx(ctx context.Context, tx *sql.Tx, match *Match) error
	FindByID(ctx context.Context, id string) (*Match, error)
	FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*Match, error)
	FindByInviteCode(ctx context.Context, code string) (*Match, error)
	FindByInviteCodeForUpdate(ctx context.Context, tx *sql.Tx, code string) (*Match, error)
	ListWaiting(ctx context.Context, gameSlug string, limit int) ([]*Match, error)
	ListUserMatches(ctx context.Context, userID string, limit, offset int) ([]*Match, int, error)
	FindExpiredTurns(ctx context.Context, now time.Time) ([]*Match, error)
	FindIdleWaitingRooms(ctx context.Context, olderThan time.Time) ([]*Match, error)
	FindExpiredDisconnects(ctx context.Context, now time.Time) ([]*Match, error)
	UpdateStateTx(ctx context.Context, tx *sql.Tx, match *Match) error
	MarkCompletedTx(ctx context.Context, tx *sql.Tx, match *Match, winnerUserID *string, status string, totalFeeCents money.Cents, completedAt time.Time, incrementVersion bool) error
	MarkCancelledTx(ctx context.Context, tx *sql.Tx, match *Match, cancelledAt time.Time) error
	GetLeaderboard(ctx context.Context, filter LeaderboardFilter) (*LeaderboardResult, error)
}

type LeaderboardRepository interface {
	GetLeaderboard(ctx context.Context, filter LeaderboardFilter) (*LeaderboardResult, error)
}

type PlayerRepository interface {
	AddPlayerTx(ctx context.Context, tx *sql.Tx, player *MatchPlayer) error
	FindByMatchAndUser(ctx context.Context, matchID, userID string) (*MatchPlayer, error)
	FindByMatchAndUserForUpdate(ctx context.Context, tx *sql.Tx, matchID, userID string) (*MatchPlayer, error)
	ListByMatch(ctx context.Context, matchID string) ([]*MatchPlayer, error)
	ListByMatchTx(ctx context.Context, tx *sql.Tx, matchID string) ([]*MatchPlayer, error)
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, matchID, userID, status string, readyAt *time.Time) error
	UpdateScoreTx(ctx context.Context, tx *sql.Tx, matchID, userID string, score int) error
	SetLedgerTxIDs(ctx context.Context, tx *sql.Tx, playerID string, debitTxID, creditTxID, feeTxID *string) error
}

type MoveRepository interface {
	RecordMoveTx(ctx context.Context, tx *sql.Tx, move *MatchMove) error
	ListByMatch(ctx context.Context, matchID string) ([]*MatchMove, error)
}

type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, error)
	GetAll(ctx context.Context) (map[string]string, error)
	Set(ctx context.Context, key, value, description string, updatedBy *string) error
}

type EscrowRepository interface {
	ReserveFundsTx(ctx context.Context, tx *sql.Tx, userID string, amount money.Cents, matchID string) (debitTxID string, err error)
	ReleaseReservedFundsTx(ctx context.Context, tx *sql.Tx, userID string, amount money.Cents, matchID, reason string) error
	SettleWinnerTx(ctx context.Context, tx *sql.Tx, winnerUserID string, grossPayout, fee, netPayout money.Cents, matchID string) (creditTxID, feeTxID string, err error)
	FinalizeLoserTx(ctx context.Context, tx *sql.Tx, loserUserID string, stake money.Cents, matchID string) error
	// PlayersWithoutReservedStake reports the players that have no 'reserved'
	// stake ledger row for the match. Settlement may only move money that a
	// reservation recorded, so callers use this to detect a match that can
	// never be settled instead of retrying the failing settlement for ever.
	PlayersWithoutReservedStake(ctx context.Context, tx *sql.Tx, matchID string, userIDs []string) ([]string, error)
}
