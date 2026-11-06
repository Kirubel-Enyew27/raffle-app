package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/raffle-app/backend/internal/admin/domain"
)

const userSelect = `
SELECT
  u.id,
  u.email,
  COALESCE(u.phone, ''),
  COALESCE(u.full_name, ''),
  COALESCE(u.avatar_url, ''),
  u.role,
  u.is_banned,
  u.created_at,
  GREATEST(
    u.updated_at,
    COALESCE(w.updated_at, u.updated_at),
    COALESCE(ms.last_match, u.updated_at),
    COALESCE(tx.last_tx, u.updated_at)
  ) AS last_active_at,
  COALESCE(w.balance, 0),
  COALESCE(w.reserved_balance, 0),
  COALESCE(ms.matches_played, 0),
  COALESCE(ms.wins, 0),
  COALESCE(ms.losses, 0),
  COALESCE(ms.draws, 0),
  EXISTS(
    SELECT 1
      FROM p2p_match_players ap
      JOIN p2p_matches am ON am.id = ap.match_id
     WHERE ap.user_id = u.id
       AND am.game_slug NOT LIKE '%test%'
       AND am.status IN ('waiting','ready','in_progress')
  ) AS is_playing,
  COUNT(*) OVER () AS total_count
FROM users u
LEFT JOIN LATERAL (
  SELECT SUM(balance) AS balance, SUM(reserved_balance) AS reserved_balance, MAX(updated_at) AS updated_at
  FROM wallets
  WHERE user_id = u.id
) w ON TRUE
LEFT JOIN LATERAL (
  SELECT
    COUNT(*) FILTER (WHERE m.status IN ('completed','tied','forfeited')) AS matches_played,
    COUNT(*) FILTER (WHERE m.status = 'completed' AND m.winner_user_id = u.id) AS wins,
    COUNT(*) FILTER (WHERE m.status IN ('completed','forfeited') AND m.winner_user_id IS NOT NULL AND m.winner_user_id <> u.id) AS losses,
    COUNT(*) FILTER (WHERE m.status = 'tied') AS draws,
    MAX(COALESCE(m.completed_at, m.updated_at)) AS last_match
  FROM p2p_match_players p
  JOIN p2p_matches m ON m.id = p.match_id
  WHERE p.user_id = u.id AND m.game_slug NOT LIKE '%test%'
) ms ON TRUE
LEFT JOIN LATERAL (
  SELECT MAX(created_at) AS last_tx
  FROM wallet_transactions
  WHERE user_id = u.id
) tx ON TRUE
WHERE u.deleted_at IS NULL`

func scanUser(row interface {
	Scan(dest ...interface{}) error
}) (*domain.UserRow, int, error) {
	var user domain.UserRow
	var total int
	var winRateBase int64
	var lastActiveAt time.Time

	err := row.Scan(
		&user.ID, &user.Email, &user.Phone, &user.FullName, &user.AvatarURL,
		&user.Role, &user.IsBanned, &user.CreatedAt, &lastActiveAt,
		&user.WalletBalance, &user.ReservedBalance,
		&user.MatchesPlayed, &user.Wins, &user.Losses, &user.Draws, &user.IsPlaying,
		&total,
	)
	if err != nil {
		return nil, 0, err
	}
	user.LastActiveAt = &lastActiveAt
	winRateBase = user.MatchesPlayed
	if winRateBase > 0 {
		user.WinRate = math.Round((float64(user.Wins)/float64(winRateBase))*1000) / 10
	}
	return &user, total, nil
}

func userFilterClause(filter domain.UserFilter) (string, []interface{}) {
	where := ""
	args := make([]interface{}, 0)
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		where += fmt.Sprintf(` AND (
			u.id::text ILIKE $%d OR u.email ILIKE $%d
			OR COALESCE(u.full_name, '') ILIKE $%d OR COALESCE(u.phone, '') ILIKE $%d
		)`, len(args), len(args), len(args), len(args))
	}
	switch strings.ToLower(filter.Status) {
	case "active":
		where += " AND u.is_banned = FALSE"
	case "banned":
		where += " AND u.is_banned = TRUE"
	case "admin":
		where += " AND u.role = 'admin'"
	case "playing":
		where += ` AND EXISTS (
			SELECT 1 FROM p2p_match_players ap
			JOIN p2p_matches am ON am.id = ap.match_id
			WHERE ap.user_id = u.id AND am.game_slug NOT LIKE '%test%'
			  AND am.status IN ('waiting','ready','in_progress')
		)`
	}
	return where, args
}

func userOrderClause(sort string) string {
	switch strings.ToLower(sort) {
	case "oldest":
		return "u.created_at ASC"
	case "matches":
		return "COALESCE(ms.matches_played, 0) DESC, u.created_at DESC"
	case "wins":
		return "COALESCE(ms.wins, 0) DESC, u.created_at DESC"
	case "balance":
		return "COALESCE(w.balance, 0) DESC, u.created_at DESC"
	default:
		return "u.created_at DESC"
	}
}

func (r *AdminRepo) Users(ctx context.Context, filter domain.UserFilter) ([]domain.UserRow, int, error) {
	where, args := userFilterClause(filter)
	args = append(args, filter.Limit, filter.Offset)
	query := userSelect + where +
		" ORDER BY " + userOrderClause(filter.Sort) +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]domain.UserRow, 0)
	total := 0
	for rows.Next() {
		user, rowTotal, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		total = rowTotal
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *AdminRepo) UserDetail(ctx context.Context, id string) (*domain.UserDetail, error) {
	user, _, err := scanUser(r.db.QueryRowContext(ctx, userSelect+" AND u.id = $1 LIMIT 1", id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	detail := &domain.UserDetail{UserRow: *user}

	matchRows, err := r.db.QueryContext(ctx, `
SELECT
  m.id,
  m.game_slug,
  m.status,
  m.competition_mode,
  CASE
    WHEN m.status = 'tied' THEN 'draw'
    WHEN m.status = 'cancelled' THEN 'cancelled'
    WHEN m.status NOT IN ('completed','forfeited','tied','cancelled') THEN 'pending'
    WHEN m.winner_user_id = $1 THEN 'win'
    WHEN m.winner_user_id IS NULL THEN 'draw'
    ELSE 'loss'
  END AS result,
  m.stake_amount,
  m.prize_pool_cents / 100.0,
  m.created_at,
  m.completed_at
FROM p2p_matches m
JOIN p2p_match_players p ON p.match_id = m.id
WHERE p.user_id = $1 AND m.game_slug NOT LIKE '%test%'
ORDER BY m.created_at DESC
LIMIT 10`, id)
	if err != nil {
		return nil, err
	}
	defer matchRows.Close()
	detail.RecentMatches = make([]domain.UserMatch, 0)
	for matchRows.Next() {
		var match domain.UserMatch
		if err := matchRows.Scan(
			&match.ID, &match.GameSlug, &match.Status, &match.CompetitionMode,
			&match.Result, &match.StakeAmount, &match.PrizePool,
			&match.CreatedAt, &match.CompletedAt,
		); err != nil {
			return nil, err
		}
		detail.RecentMatches = append(detail.RecentMatches, match)
	}
	if err := matchRows.Err(); err != nil {
		return nil, err
	}

	txRows, err := r.db.QueryContext(ctx, `
SELECT id, type, status, amount, COALESCE(reference, ''), created_at
FROM wallet_transactions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 10`, id)
	if err != nil {
		return nil, err
	}
	defer txRows.Close()
	detail.RecentTransactions = make([]domain.UserTransaction, 0)
	for txRows.Next() {
		var transaction domain.UserTransaction
		if err := txRows.Scan(
			&transaction.ID, &transaction.Type, &transaction.Status,
			&transaction.Amount, &transaction.Reference, &transaction.CreatedAt,
		); err != nil {
			return nil, err
		}
		detail.RecentTransactions = append(detail.RecentTransactions, transaction)
	}
	if err := txRows.Err(); err != nil {
		return nil, err
	}
	return detail, nil
}
