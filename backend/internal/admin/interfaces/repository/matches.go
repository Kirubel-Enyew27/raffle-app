package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/raffle-app/backend/internal/admin/domain"
)

const matchSelect = `
SELECT
  m.id,
  m.game_slug,
  COALESCE(g.title, m.game_slug),
  m.status,
  m.match_type,
  m.competition_mode,
  m.stake_amount,
  m.prize_pool_cents / 100.0,
  m.total_fee_cents / 100.0,
  COALESCE(m.winner_user_id::text, ''),
  m.created_at,
  m.started_at,
  m.completed_at,
  GREATEST(0, EXTRACT(EPOCH FROM (
    COALESCE(m.completed_at, NOW()) - COALESCE(m.started_at, m.created_at)
  )))::bigint,
  COALESCE(json_agg(json_build_object(
    'user_id', p.user_id,
    'display_name', COALESCE(NULLIF(u.full_name, ''), u.email, 'Player'),
    'avatar_url', COALESCE(u.avatar_url, ''),
    'player_index', p.player_index,
    'status', p.status,
    'score', p.score
  ) ORDER BY p.player_index) FILTER (WHERE p.id IS NOT NULL), '[]'::json),
  EXISTS(SELECT 1 FROM wallet_transactions wt WHERE wt.reference = 'p2p_payout:' || m.id AND wt.status = 'completed') AS has_payout,
  EXISTS(SELECT 1 FROM wallet_transactions wt WHERE wt.reference = 'p2p_refund:' || m.id AND wt.status IN ('completed', 'refunded')) AS has_refund,
  EXISTS(SELECT 1 FROM wallet_transactions wt WHERE wt.reference = 'p2p_stake:' || m.id AND wt.status = 'reserved') AS has_reserved
FROM p2p_matches m
JOIN p2p_games g ON g.slug = m.game_slug
LEFT JOIN p2p_match_players p ON p.match_id = m.id
LEFT JOIN users u ON u.id = p.user_id
`

func financialStatus(row *domain.MatchRow, hasPayout, hasRefund, hasReserved bool) string {
	if row.CompetitionMode != "earn" {
		return "not_applicable"
	}
	if hasPayout {
		return "settled"
	}
	if hasRefund && !hasReserved {
		return "refunded"
	}
	if hasReserved {
		return "reserved"
	}
	switch row.Status {
	case "completed", "forfeited":
		return "needs_attention"
	case "tied", "cancelled":
		return "needs_attention"
	case "waiting", "ready", "in_progress":
		return "pending"
	default:
		return "pending"
	}
}

func queryMatchRows(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]domain.MatchRow, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]domain.MatchRow, 0)
	for rows.Next() {
		var match domain.MatchRow
		var rawPlayers []byte
		var hasPayout, hasRefund, hasReserved bool
		if err := rows.Scan(
			&match.ID, &match.GameSlug, &match.GameTitle, &match.Status,
			&match.MatchType, &match.CompetitionMode, &match.StakeAmount,
			&match.PrizePool, &match.ServiceFee, &match.WinnerUserID,
			&match.CreatedAt, &match.StartedAt, &match.CompletedAt,
			&match.DurationSeconds, &rawPlayers,
			&hasPayout, &hasRefund, &hasReserved,
		); err != nil {
			return nil, err
		}
		if len(rawPlayers) > 0 {
			_ = json.Unmarshal(rawPlayers, &match.Players)
		}
		if match.Players == nil {
			match.Players = []domain.MatchPlayer{}
		}
		for _, player := range match.Players {
			if player.UserID == match.WinnerUserID {
				match.WinnerName = player.DisplayName
				break
			}
		}
		match.FinancialStatus = financialStatus(&match, hasPayout, hasRefund, hasReserved)
		matches = append(matches, match)
	}
	return matches, rows.Err()
}

func matchFilterClause(filter domain.MatchFilter) (string, []interface{}) {
	// leaderboard-test-* games are internal fixtures, never real player activity.
	where := " AND m.game_slug NOT LIKE '%test%'"
	args := make([]interface{}, 0)
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		index := len(args)
		where += fmt.Sprintf(` AND (
			m.id::text ILIKE $%d
			OR EXISTS (
				SELECT 1 FROM p2p_match_players sp
				JOIN users su ON su.id = sp.user_id
				WHERE sp.match_id = m.id
				  AND (su.email ILIKE $%d OR COALESCE(su.full_name, '') ILIKE $%d)
			)
		)`, index, index, index)
	}
	if game := strings.TrimSpace(filter.Game); game != "" {
		args = append(args, game)
		where += fmt.Sprintf(" AND m.game_slug = $%d", len(args))
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND m.status = $%d", len(args))
	}
	if mode := strings.TrimSpace(filter.Mode); mode != "" {
		args = append(args, mode)
		where += fmt.Sprintf(" AND m.competition_mode = $%d", len(args))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		where += fmt.Sprintf(" AND m.created_at >= $%d", len(args))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		where += fmt.Sprintf(" AND m.created_at < $%d", len(args))
	}
	return where, args
}

func matchOrderClause(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "oldest":
		return "m.created_at ASC"
	case "stake":
		return "m.stake_amount DESC, m.created_at DESC"
	case "duration":
		return "EXTRACT(EPOCH FROM (COALESCE(m.completed_at, NOW()) - COALESCE(m.started_at, m.created_at))) DESC, m.created_at DESC"
	default:
		return "m.created_at DESC"
	}
}

func (r *AdminRepo) Matches(ctx context.Context, filter domain.MatchFilter) ([]domain.MatchRow, int, error) {
	where, args := matchFilterClause(filter)
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM p2p_matches m WHERE 1=1"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	queryArgs := append(append([]interface{}{}, args...), filter.Limit, filter.Offset)
	query := matchSelect + " WHERE 1=1" + where +
		" GROUP BY m.id, g.title ORDER BY m.created_at DESC" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	matches, err := queryMatchRows(ctx, r.db, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	return matches, total, nil
}
