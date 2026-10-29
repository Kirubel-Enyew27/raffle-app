package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/raffle-app/backend/internal/p2p/domain"
	"github.com/raffle-app/backend/pkg/money"
)

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ---------------------------------------------------------------------------
// Game Repository
// ---------------------------------------------------------------------------

type GameRepo struct{ db *sql.DB }

func NewGameRepo(db *sql.DB) *GameRepo { return &GameRepo{db: db} }

func (r *GameRepo) ListActive(ctx context.Context) ([]*domain.Game, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT slug, title, description, min_players, max_players, is_active,
		min_stake_cents, max_stake_cents, default_fee_bp, default_turn_seconds, rules_json, created_at, updated_at
		FROM p2p_games WHERE is_active = TRUE ORDER BY title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*domain.Game
	for rows.Next() {
		g := &domain.Game{}
		var rulesRaw []byte
		var minStake, maxStake int64
		if err := rows.Scan(&g.Slug, &g.Title, &g.Description, &g.MinPlayers, &g.MaxPlayers, &g.IsActive,
			&minStake, &maxStake, &g.DefaultFeeBP, &g.DefaultTurnSeconds, &rulesRaw, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		g.MinStakeCents = money.Cents(minStake)
		g.MaxStakeCents = money.Cents(maxStake)
		if len(rulesRaw) > 0 {
			_ = json.Unmarshal(rulesRaw, &g.RulesJSON)
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func (r *GameRepo) FindBySlug(ctx context.Context, slug string) (*domain.Game, error) {
	g := &domain.Game{}
	var rulesRaw []byte
	var minStake, maxStake int64
	err := r.db.QueryRowContext(ctx, `SELECT slug, title, description, min_players, max_players, is_active,
		min_stake_cents, max_stake_cents, default_fee_bp, default_turn_seconds, rules_json, created_at, updated_at
		FROM p2p_games WHERE slug = $1`, slug).Scan(&g.Slug, &g.Title, &g.Description, &g.MinPlayers, &g.MaxPlayers,
		&g.IsActive, &minStake, &maxStake, &g.DefaultFeeBP, &g.DefaultTurnSeconds, &rulesRaw, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrGameNotFound
		}
		return nil, err
	}
	g.MinStakeCents = money.Cents(minStake)
	g.MaxStakeCents = money.Cents(maxStake)
	if len(rulesRaw) > 0 {
		_ = json.Unmarshal(rulesRaw, &g.RulesJSON)
	}
	return g, nil
}

// ---------------------------------------------------------------------------
// Match Repository
// ---------------------------------------------------------------------------

type MatchRepo struct{ db *sql.DB }

func NewMatchRepo(db *sql.DB) *MatchRepo { return &MatchRepo{db: db} }

func (r *MatchRepo) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

const matchColumns = `id, game_slug, status, match_type, invite_code, stake_cents, stake_amount,
	prize_pool_cents, service_fee_bp, total_fee_cents, winner_user_id, current_turn_user_id,
	turn_deadline, current_round, max_rounds, scores, board_state, created_by,
	started_at, completed_at, created_at, updated_at, rematch_requested_by, rematch_match_id,
	disconnect_user_id, disconnect_deadline`

// matchColumnsM is the same list qualified with the `m` alias. Queries that join
// p2p_match_players must use it: both tables expose id/status/created_at, and an
// unqualified list fails with "column reference is ambiguous".
const matchColumnsM = `m.id, m.game_slug, m.status, m.match_type, m.invite_code, m.stake_cents, m.stake_amount,
	m.prize_pool_cents, m.service_fee_bp, m.total_fee_cents, m.winner_user_id, m.current_turn_user_id,
	m.turn_deadline, m.current_round, m.max_rounds, m.scores, m.board_state, m.created_by,
	m.started_at, m.completed_at, m.created_at, m.updated_at, m.rematch_requested_by, m.rematch_match_id,
	m.disconnect_user_id, m.disconnect_deadline`

func scanMatch(row interface {
	Scan(dest ...interface{}) error
}) (*domain.Match, error) {
	m := &domain.Match{}
	var inviteCode sql.NullString
	var winnerID, turnUserID sql.NullString
	var turnDeadline, startedAt, completedAt, disconnectDeadline sql.NullTime
	var rematchRequestedBy, rematchMatchID, disconnectUserID sql.NullString
	var stakeCents, poolCents, feeCents int64
	var scoresRaw, boardRaw []byte

	err := row.Scan(&m.ID, &m.GameSlug, &m.Status, &m.MatchType, &inviteCode, &stakeCents, &m.StakeAmount,
		&poolCents, &m.ServiceFeeBP, &feeCents, &winnerID, &turnUserID,
		&turnDeadline, &m.CurrentRound, &m.MaxRounds, &scoresRaw, &boardRaw, &m.CreatedBy,
		&startedAt, &completedAt, &m.CreatedAt, &m.UpdatedAt, &rematchRequestedBy, &rematchMatchID,
		&disconnectUserID, &disconnectDeadline)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrMatchNotFound
		}
		return nil, err
	}

	m.StakeCents = money.Cents(stakeCents)
	m.PrizePoolCents = money.Cents(poolCents)
	m.TotalFeeCents = money.Cents(feeCents)
	if inviteCode.Valid {
		m.InviteCode = inviteCode.String
	}
	if winnerID.Valid {
		w := winnerID.String
		m.WinnerUserID = &w
	}
	if turnUserID.Valid {
		t := turnUserID.String
		m.CurrentTurnUserID = &t
	}
	if turnDeadline.Valid {
		m.TurnDeadline = &turnDeadline.Time
	}
	if startedAt.Valid {
		m.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		m.CompletedAt = &completedAt.Time
	}
	if rematchRequestedBy.Valid {
		v := rematchRequestedBy.String
		m.RematchRequestedBy = &v
	}
	if rematchMatchID.Valid {
		v := rematchMatchID.String
		m.RematchMatchID = &v
	}
	if disconnectUserID.Valid {
		v := disconnectUserID.String
		m.DisconnectUserID = &v
	}
	if disconnectDeadline.Valid {
		m.DisconnectDeadline = &disconnectDeadline.Time
	}
	if len(scoresRaw) > 0 {
		_ = json.Unmarshal(scoresRaw, &m.Scores)
	}
	if len(boardRaw) > 0 {
		_ = json.Unmarshal(boardRaw, &m.BoardState)
	}
	return m, nil
}

func (r *MatchRepo) Create(ctx context.Context, match *domain.Match) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := r.CreateTx(ctx, tx, match); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *MatchRepo) CreateTx(ctx context.Context, tx *sql.Tx, match *domain.Match) error {
	if match.ID == "" {
		match.ID = newID()
	}
	scoresJSON, _ := json.Marshal(match.Scores)
	boardJSON, _ := json.Marshal(match.BoardState)

	query := `INSERT INTO p2p_matches (
		id, game_slug, status, match_type, invite_code, stake_cents, stake_amount,
		prize_pool_cents, service_fee_bp, total_fee_cents, winner_user_id, current_turn_user_id,
		turn_deadline, current_round, max_rounds, scores, board_state, created_by,
		rematch_requested_by, rematch_match_id, started_at, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,NOW(),NOW())
	RETURNING created_at, updated_at`

	return tx.QueryRowContext(ctx, query,
		match.ID, match.GameSlug, match.Status, match.MatchType, nullIfEmpty(match.InviteCode),
		int64(match.StakeCents), match.StakeAmount, int64(match.PrizePoolCents),
		match.ServiceFeeBP, int64(match.TotalFeeCents), nullIfEmpty(ptrStr(match.WinnerUserID)),
		nullIfEmpty(ptrStr(match.CurrentTurnUserID)), match.TurnDeadline, match.CurrentRound,
		match.MaxRounds, string(scoresJSON), string(boardJSON), match.CreatedBy,
		nullIfEmpty(ptrStr(match.RematchRequestedBy)), nullIfEmpty(ptrStr(match.RematchMatchID)),
		match.StartedAt).Scan(&match.CreatedAt, &match.UpdatedAt)
}

func (r *MatchRepo) FindByID(ctx context.Context, id string) (*domain.Match, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches WHERE id = $1`, id)
	return scanMatch(row)
}

func (r *MatchRepo) FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*domain.Match, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches WHERE id = $1 FOR UPDATE`, id)
	return scanMatch(row)
}

func (r *MatchRepo) FindByInviteCode(ctx context.Context, code string) (*domain.Match, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches WHERE UPPER(invite_code) = UPPER($1)`, strings.TrimSpace(code))
	return scanMatch(row)
}

func (r *MatchRepo) FindByInviteCodeForUpdate(ctx context.Context, tx *sql.Tx, code string) (*domain.Match, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches WHERE UPPER(invite_code) = UPPER($1) FOR UPDATE`, strings.TrimSpace(code))
	return scanMatch(row)
}

func (r *MatchRepo) ListWaiting(ctx context.Context, gameSlug string, limit int) ([]*domain.Match, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows *sql.Rows
	var err error
	if gameSlug != "" {
		rows, err = r.db.QueryContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches
			WHERE status = 'waiting' AND match_type = 'public_queue' AND game_slug = $1
			ORDER BY created_at DESC LIMIT $2`, gameSlug, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches
			WHERE status = 'waiting' AND match_type = 'public_queue'
			ORDER BY created_at DESC LIMIT $1`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Match
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MatchRepo) ListUserMatches(ctx context.Context, userID string, limit, offset int) ([]*domain.Match, int, error) {
	if limit <= 0 {
		limit = 20
	}
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT m.id) FROM p2p_matches m
		JOIN p2p_match_players p ON m.id = p.match_id WHERE p.user_id = $1`, userID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `SELECT `+matchColumnsM+` FROM p2p_matches m
		JOIN p2p_match_players p ON m.id = p.match_id WHERE p.user_id = $1
		ORDER BY m.created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*domain.Match
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, m)
	}
	return list, count, rows.Err()
}

func (r *MatchRepo) FindExpiredTurns(ctx context.Context, now time.Time) ([]*domain.Match, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches
		WHERE status = 'in_progress' AND turn_deadline IS NOT NULL AND turn_deadline < $1`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Match
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MatchRepo) FindIdleWaitingRooms(ctx context.Context, olderThan time.Time) ([]*domain.Match, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches
		WHERE status = 'waiting' AND created_at < $1`, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Match
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MatchRepo) FindExpiredDisconnects(ctx context.Context, now time.Time) ([]*domain.Match, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+matchColumns+` FROM p2p_matches
		WHERE status = 'in_progress' AND disconnect_deadline IS NOT NULL AND disconnect_deadline < $1`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Match
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MatchRepo) UpdateStateTx(ctx context.Context, tx *sql.Tx, match *domain.Match) error {
	scoresJSON, _ := json.Marshal(match.Scores)
	boardJSON, _ := json.Marshal(match.BoardState)

	_, err := tx.ExecContext(ctx, `UPDATE p2p_matches SET
		status = $2,
		current_turn_user_id = $3,
		turn_deadline = $4,
		current_round = $5,
		scores = $6,
		board_state = $7,
		prize_pool_cents = $8,
		started_at = COALESCE(started_at, $9),
		rematch_requested_by = $10,
		rematch_match_id = $11,
		disconnect_user_id = $12,
		disconnect_deadline = $13,
		updated_at = NOW()
		WHERE id = $1`,
		match.ID, match.Status, nullIfEmpty(ptrStr(match.CurrentTurnUserID)), match.TurnDeadline,
		match.CurrentRound, string(scoresJSON), string(boardJSON), int64(match.PrizePoolCents), match.StartedAt,
		nullIfEmpty(ptrStr(match.RematchRequestedBy)), nullIfEmpty(ptrStr(match.RematchMatchID)),
		nullIfEmpty(ptrStr(match.DisconnectUserID)), match.DisconnectDeadline)
	return err
}

func (r *MatchRepo) MarkCompletedTx(ctx context.Context, tx *sql.Tx, matchID string, winnerUserID *string, status string, totalFeeCents money.Cents, completedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE p2p_matches SET
		status = $2,
		winner_user_id = $3,
		total_fee_cents = $4,
		completed_at = $5,
		current_turn_user_id = NULL,
		turn_deadline = NULL,
		disconnect_user_id = NULL,
		disconnect_deadline = NULL,
		updated_at = NOW()
		WHERE id = $1`,
		matchID, status, nullIfEmpty(ptrStr(winnerUserID)), int64(totalFeeCents), completedAt)
	return err
}

func (r *MatchRepo) MarkCancelledTx(ctx context.Context, tx *sql.Tx, matchID string, cancelledAt time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE p2p_matches SET
		status = 'cancelled',
		completed_at = $2,
		current_turn_user_id = NULL,
		turn_deadline = NULL,
		updated_at = NOW()
		WHERE id = $1 AND status = 'waiting'`,
		matchID, cancelledAt)
	return err
}

func (r *MatchRepo) GetLeaderboard(ctx context.Context, filter domain.LeaderboardFilter) (*domain.LeaderboardResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	var periodCondition string
	switch filter.Period {
	case domain.PeriodDaily:
		periodCondition = "completed_at >= DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC')"
	case domain.PeriodWeekly:
		periodCondition = "completed_at >= DATE_TRUNC('week', NOW() AT TIME ZONE 'UTC')"
	case domain.PeriodMonthly:
		periodCondition = "completed_at >= DATE_TRUNC('month', NOW() AT TIME ZONE 'UTC')"
	default:
		filter.Period = domain.PeriodAllTime
		periodCondition = "1=1"
	}

	var orderClause string
	switch filter.SortBy {
	case domain.SortByWinStreak:
		orderClause = "current_win_streak DESC, wins DESC, win_rate DESC, matches_played DESC, user_id ASC"
	case domain.SortByWinRate:
		orderClause = "min_matches_met ASC, win_rate DESC, wins DESC, matches_played DESC, user_id ASC"
	default:
		filter.SortBy = domain.SortByWins
		orderClause = "wins DESC, win_rate DESC, matches_played DESC, user_id ASC"
	}

	gameSlug := ""
	if filter.Game != nil {
		gameSlug = *filter.Game
	}

	offset := (filter.Page - 1) * filter.Limit

	baseCTE := `WITH finalized_matches AS (
    SELECT
        m.id AS match_id,
        m.game_slug,
        m.winner_user_id,
        m.status,
        COALESCE(m.completed_at, m.created_at) AS completed_at,
        p.user_id
    FROM p2p_matches m
    JOIN p2p_match_players p ON m.id = p.match_id
    WHERE m.status IN ('completed', 'forfeited', 'tied')
      AND ($1 = '' OR m.game_slug = $1)
),
period_stats AS (
    SELECT
        user_id,
        COUNT(*) AS matches_played,
        SUM(CASE WHEN winner_user_id = user_id THEN 1 ELSE 0 END) AS wins,
        SUM(CASE WHEN winner_user_id IS NOT NULL AND winner_user_id != user_id THEN 1 ELSE 0 END) AS losses,
        SUM(CASE WHEN status = 'tied' OR winner_user_id IS NULL THEN 1 ELSE 0 END) AS draws
    FROM finalized_matches
    WHERE ` + periodCondition + `
    GROUP BY user_id
    HAVING COUNT(*) > 0
),
player_outcomes AS (
    SELECT
        user_id,
        completed_at,
        match_id,
        CASE WHEN winner_user_id = user_id THEN 1 ELSE 0 END AS is_win
    FROM finalized_matches
),
ranked_outcomes AS (
    SELECT
        user_id,
        is_win,
        ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY completed_at DESC, match_id DESC) AS rn_desc,
        SUM(CASE WHEN is_win = 0 THEN 1 ELSE 0 END) OVER (PARTITION BY user_id ORDER BY completed_at ASC, match_id ASC) AS grp
    FROM player_outcomes
),
streak_groups AS (
    SELECT
        user_id,
        grp,
        COUNT(*) AS streak_len
    FROM ranked_outcomes
    WHERE is_win = 1
    GROUP BY user_id, grp
),
player_streaks AS (
    SELECT
        r.user_id,
        COALESCE(MIN(r.rn_desc) FILTER (WHERE r.is_win = 0) - 1, COUNT(*)) AS current_win_streak,
        COALESCE((SELECT MAX(sg.streak_len) FROM streak_groups sg WHERE sg.user_id = r.user_id), 0) AS longest_win_streak
    FROM ranked_outcomes r
    GROUP BY r.user_id
),
aggregated_leaderboard AS (
    SELECT
        ps.user_id,
        COALESCE(NULLIF(TRIM(u.full_name), ''), 'Player_' || SUBSTRING(u.id::text FROM 1 FOR 6)) AS display_name,
        u.avatar_url,
        ps.matches_played,
        ps.wins,
        ps.losses,
        ps.draws,
        ROUND(
            CASE WHEN ps.matches_played > 0
                 THEN (ps.wins::numeric / ps.matches_played::numeric) * 100.0
                 ELSE 0.0
            END,
            2
        )::float8 AS win_rate,
        COALESCE(st.current_win_streak, 0) AS current_win_streak,
        COALESCE(st.longest_win_streak, 0) AS longest_win_streak,
        CASE WHEN ps.matches_played >= 3 THEN 0 ELSE 1 END AS min_matches_met
    FROM period_stats ps
    JOIN users u ON ps.user_id = u.id
    LEFT JOIN player_streaks st ON ps.user_id = st.user_id
),
ranked_players AS (
    SELECT
        ROW_NUMBER() OVER (ORDER BY ` + orderClause + `) AS rank,
        user_id,
        display_name,
        avatar_url,
        matches_played,
        wins,
        losses,
        draws,
        win_rate,
        current_win_streak,
        longest_win_streak,
        COUNT(*) OVER () AS total_players
    FROM aggregated_leaderboard
)`

	query := baseCTE + `
SELECT
    rank,
    user_id,
    display_name,
    avatar_url,
    matches_played,
    wins,
    losses,
    draws,
    win_rate,
    current_win_streak,
    longest_win_streak,
    total_players
FROM ranked_players
ORDER BY rank ASC
LIMIT $2 OFFSET $3;`

	rows, err := r.db.QueryContext(ctx, query, gameSlug, filter.Limit, offset)
	if err != nil {
		return nil, fmt.Errorf("leaderboard query failed: %w", err)
	}
	defer rows.Close()

	var players []*domain.LeaderboardPlayer
	totalPlayers := 0

	for rows.Next() {
		p := &domain.LeaderboardPlayer{}
		var avatar sql.NullString
		if err := rows.Scan(
			&p.Rank,
			&p.PlayerID,
			&p.DisplayName,
			&avatar,
			&p.MatchesPlayed,
			&p.Wins,
			&p.Losses,
			&p.Draws,
			&p.WinRate,
			&p.CurrentWinStreak,
			&p.LongestWinStreak,
			&totalPlayers,
		); err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard row: %w", err)
		}
		if avatar.Valid && avatar.String != "" {
			s := avatar.String
			p.AvatarURL = &s
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if players == nil {
		players = []*domain.LeaderboardPlayer{}
	}

	totalPages := 0
	if totalPlayers > 0 {
		totalPages = int(math.Ceil(float64(totalPlayers) / float64(filter.Limit)))
	}

	result := &domain.LeaderboardResult{
		Period:  string(filter.Period),
		Game:    filter.Game,
		SortBy:  string(filter.SortBy),
		Players: players,
		Pagination: &domain.LeaderboardPagination{
			Page:         filter.Page,
			Limit:        filter.Limit,
			TotalPlayers: totalPlayers,
			TotalPages:   totalPages,
		},
	}

	// Check if current authenticated player should be identified
	if filter.UserID != nil && *filter.UserID != "" {
		userID := *filter.UserID
		for _, p := range players {
			if p.PlayerID == userID {
				result.CurrentPlayer = p
				break
			}
		}

		if result.CurrentPlayer == nil {
			userQuery := baseCTE + `
SELECT
    rank,
    user_id,
    display_name,
    avatar_url,
    matches_played,
    wins,
    losses,
    draws,
    win_rate,
    current_win_streak,
    longest_win_streak,
    total_players
FROM ranked_players
WHERE user_id = $2;`
			row := r.db.QueryRowContext(ctx, userQuery, gameSlug, userID)
			userPlayer := &domain.LeaderboardPlayer{}
			var avatar sql.NullString
			var total int
			if err := row.Scan(
				&userPlayer.Rank,
				&userPlayer.PlayerID,
				&userPlayer.DisplayName,
				&avatar,
				&userPlayer.MatchesPlayed,
				&userPlayer.Wins,
				&userPlayer.Losses,
				&userPlayer.Draws,
				&userPlayer.WinRate,
				&userPlayer.CurrentWinStreak,
				&userPlayer.LongestWinStreak,
				&total,
			); err == nil {
				if avatar.Valid && avatar.String != "" {
					s := avatar.String
					userPlayer.AvatarURL = &s
				}
				result.CurrentPlayer = userPlayer
			}
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Player Repository
// ---------------------------------------------------------------------------

type PlayerRepo struct{ db *sql.DB }

func NewPlayerRepo(db *sql.DB) *PlayerRepo { return &PlayerRepo{db: db} }

const playerColumns = `p.id, p.match_id, p.user_id, COALESCE(u.full_name, u.email, 'Player'),
	COALESCE(u.avatar_url, ''), p.player_index, p.symbol, p.status, p.score,
	p.debit_tx_id, p.credit_tx_id, p.fee_tx_id, p.joined_at, p.ready_at`

func scanPlayer(row interface {
	Scan(dest ...interface{}) error
}) (*domain.MatchPlayer, error) {
	p := &domain.MatchPlayer{}
	var debitID, creditID, feeID sql.NullString
	var readyAt sql.NullTime

	err := row.Scan(&p.ID, &p.MatchID, &p.UserID, &p.DisplayName, &p.AvatarURL,
		&p.PlayerIndex, &p.Symbol, &p.Status, &p.Score,
		&debitID, &creditID, &feeID, &p.JoinedAt, &readyAt)
	if err != nil {
		return nil, err
	}
	if debitID.Valid {
		d := debitID.String
		p.DebitTxID = &d
	}
	if creditID.Valid {
		c := creditID.String
		p.CreditTxID = &c
	}
	if feeID.Valid {
		f := feeID.String
		p.FeeTxID = &f
	}
	if readyAt.Valid {
		p.ReadyAt = &readyAt.Time
	}
	return p, nil
}

func (r *PlayerRepo) AddPlayerTx(ctx context.Context, tx *sql.Tx, p *domain.MatchPlayer) error {
	if p.ID == "" {
		p.ID = newID()
	}
	query := `INSERT INTO p2p_match_players (
		id, match_id, user_id, player_index, symbol, status, score,
		debit_tx_id, credit_tx_id, fee_tx_id, joined_at, ready_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),$11)`

	_, err := tx.ExecContext(ctx, query,
		p.ID, p.MatchID, p.UserID, p.PlayerIndex, p.Symbol, p.Status, p.Score,
		nullIfEmpty(ptrStr(p.DebitTxID)), nullIfEmpty(ptrStr(p.CreditTxID)),
		nullIfEmpty(ptrStr(p.FeeTxID)), p.ReadyAt)
	return err
}

func (r *PlayerRepo) FindByMatchAndUser(ctx context.Context, matchID, userID string) (*domain.MatchPlayer, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM p2p_match_players p
		JOIN users u ON p.user_id = u.id WHERE p.match_id = $1 AND p.user_id = $2`, matchID, userID)
	return scanPlayer(row)
}

func (r *PlayerRepo) FindByMatchAndUserForUpdate(ctx context.Context, tx *sql.Tx, matchID, userID string) (*domain.MatchPlayer, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM p2p_match_players p
		JOIN users u ON p.user_id = u.id WHERE p.match_id = $1 AND p.user_id = $2 FOR UPDATE`, matchID, userID)
	return scanPlayer(row)
}

func (r *PlayerRepo) ListByMatch(ctx context.Context, matchID string) ([]*domain.MatchPlayer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+playerColumns+` FROM p2p_match_players p
		JOIN users u ON p.user_id = u.id WHERE p.match_id = $1 ORDER BY p.player_index ASC`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.MatchPlayer
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *PlayerRepo) ListByMatchTx(ctx context.Context, tx *sql.Tx, matchID string) ([]*domain.MatchPlayer, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+playerColumns+` FROM p2p_match_players p
		JOIN users u ON p.user_id = u.id WHERE p.match_id = $1 ORDER BY p.player_index ASC`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.MatchPlayer
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *PlayerRepo) UpdateStatusTx(ctx context.Context, tx *sql.Tx, matchID, userID, status string, readyAt *time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE p2p_match_players
		SET status = $3, ready_at = COALESCE($4, ready_at)
		WHERE match_id = $1 AND user_id = $2`, matchID, userID, status, readyAt)
	return err
}

func (r *PlayerRepo) UpdateScoreTx(ctx context.Context, tx *sql.Tx, matchID, userID string, score int) error {
	_, err := tx.ExecContext(ctx, `UPDATE p2p_match_players SET score = $3 WHERE match_id = $1 AND user_id = $2`,
		matchID, userID, score)
	return err
}

func (r *PlayerRepo) SetLedgerTxIDs(ctx context.Context, tx *sql.Tx, playerID string, debitTxID, creditTxID, feeTxID *string) error {
	_, err := tx.ExecContext(ctx, `UPDATE p2p_match_players SET
		debit_tx_id = COALESCE($2, debit_tx_id),
		credit_tx_id = COALESCE($3, credit_tx_id),
		fee_tx_id = COALESCE($4, fee_tx_id)
		WHERE id = $1`, playerID, nullIfEmpty(ptrStr(debitTxID)), nullIfEmpty(ptrStr(creditTxID)), nullIfEmpty(ptrStr(feeTxID)))
	return err
}

// ---------------------------------------------------------------------------
// Move Repository
// ---------------------------------------------------------------------------

type MoveRepo struct{ db *sql.DB }

func NewMoveRepo(db *sql.DB) *MoveRepo { return &MoveRepo{db: db} }

func (r *MoveRepo) RecordMoveTx(ctx context.Context, tx *sql.Tx, move *domain.MatchMove) error {
	if move.ID == "" {
		move.ID = newID()
	}
	actionJSON, _ := json.Marshal(move.Action)
	boardJSON, _ := json.Marshal(move.BoardSnapshot)

	_, err := tx.ExecContext(ctx, `INSERT INTO p2p_match_moves (
		id, match_id, round_number, move_number, user_id, action, board_snapshot, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		move.ID, move.MatchID, move.RoundNumber, move.MoveNumber, move.UserID, string(actionJSON), string(boardJSON))
	return err
}

func (r *MoveRepo) ListByMatch(ctx context.Context, matchID string) ([]*domain.MatchMove, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, match_id, round_number, move_number, user_id, action, board_snapshot, created_at
		FROM p2p_match_moves WHERE match_id = $1 ORDER BY round_number ASC, move_number ASC`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moves []*domain.MatchMove
	for rows.Next() {
		m := &domain.MatchMove{}
		var actionRaw, boardRaw []byte
		if err := rows.Scan(&m.ID, &m.MatchID, &m.RoundNumber, &m.MoveNumber, &m.UserID, &actionRaw, &boardRaw, &m.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(actionRaw, &m.Action)
		_ = json.Unmarshal(boardRaw, &m.BoardSnapshot)
		moves = append(moves, m)
	}
	return moves, rows.Err()
}

// ---------------------------------------------------------------------------
// Settings Repository
// ---------------------------------------------------------------------------

type SettingsRepo struct{ db *sql.DB }

func NewSettingsRepo(db *sql.DB) *SettingsRepo { return &SettingsRepo{db: db} }

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var val string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM p2p_settings WHERE key = $1`, key).Scan(&val)
	return val, err
}

func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM p2p_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		res[k] = v
	}
	return res, rows.Err()
}

func (r *SettingsRepo) Set(ctx context.Context, key, value, description string, updatedBy *string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO p2p_settings (key, value, description, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, description = EXCLUDED.description,
		updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		key, value, description, nullIfEmpty(ptrStr(updatedBy)))
	return err
}

// ---------------------------------------------------------------------------
// Escrow Repository (Wallet Balance Lock & Settlement)
// ---------------------------------------------------------------------------

type EscrowRepo struct {
	db             *sql.DB
	platformUserID string
}

func NewEscrowRepo(db *sql.DB, platformUserID string) *EscrowRepo {
	return &EscrowRepo{db: db, platformUserID: platformUserID}
}

func (r *EscrowRepo) ReserveFundsTx(ctx context.Context, tx *sql.Tx, userID string, amount money.Cents, matchID string) (string, error) {
	if amount <= 0 {
		return "", nil // Free play match requires no fund reservation
	}

	// Ensure wallet exists and lock row FOR UPDATE
	var walletID string
	var balance, reserved float64
	row := tx.QueryRowContext(ctx, `SELECT id, balance, reserved_balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID)
	if err := row.Scan(&walletID, &balance, &reserved); err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrInsufficientStake
		}
		return "", err
	}

	amtFloat := amount.Float()
	avail := balance - reserved
	if avail < amtFloat {
		return "", domain.ErrInsufficientStake
	}

	// Increment reserved balance
	_, err := tx.ExecContext(ctx, `UPDATE wallets SET
		reserved_balance = reserved_balance + $1,
		reserved_cents = reserved_cents + $2,
		updated_at = NOW()
		WHERE id = $3`, amtFloat, int64(amount), walletID)
	if err != nil {
		return "", err
	}

	// Record reservation transaction
	txID := newID()
	metaJSON, _ := json.Marshal(map[string]interface{}{"match_id": matchID, "amount_cents": int64(amount)})
	_, err = tx.ExecContext(ctx, `INSERT INTO wallet_transactions (
		id, wallet_id, user_id, type, status, amount, balance_before, balance_after,
		reference, description, metadata, created_at, updated_at
	) VALUES ($1,$2,$3,'p2p_match_stake','reserved',$4,$5,$6,$7,$8,$9,NOW(),NOW())`,
		txID, walletID, userID, amtFloat, balance, balance,
		"p2p_stake:"+matchID, "Locked stake for P2P match", string(metaJSON))
	if err != nil {
		return "", err
	}

	return txID, nil
}

func (r *EscrowRepo) ReleaseReservedFundsTx(ctx context.Context, tx *sql.Tx, userID string, amount money.Cents, matchID, reason string) error {
	if amount <= 0 {
		return nil
	}
	amtFloat := amount.Float()

	var walletID string
	var balance float64
	row := tx.QueryRowContext(ctx, `SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID)
	if err := row.Scan(&walletID, &balance); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `UPDATE wallets SET
		reserved_balance = GREATEST(0, reserved_balance - $1),
		reserved_cents = GREATEST(0, reserved_cents - $2),
		updated_at = NOW()
		WHERE id = $3`, amtFloat, int64(amount), walletID)
	if err != nil {
		return err
	}

	// Close out the reservation itself so the ledger never keeps a stake row in
	// 'reserved' after the funds have been released.
	_, err = tx.ExecContext(ctx, `UPDATE wallet_transactions SET
		status = 'refunded',
		balance_after = $1,
		updated_at = NOW()
		WHERE reference = $2 AND type = 'p2p_match_stake' AND user_id = $3 AND status = 'reserved'`,
		balance, "p2p_stake:"+matchID, userID)
	if err != nil {
		return err
	}

	txID := newID()
	metaJSON, _ := json.Marshal(map[string]interface{}{"match_id": matchID, "reason": reason})
	_, err = tx.ExecContext(ctx, `INSERT INTO wallet_transactions (
		id, wallet_id, user_id, type, status, amount, balance_before, balance_after,
		reference, description, metadata, created_at, updated_at
	) VALUES ($1,$2,$3,'p2p_stake_refund','completed',$4,$5,$6,$7,$8,$9,NOW(),NOW())`,
		txID, walletID, userID, amtFloat, balance, balance,
		"p2p_refund:"+matchID, "Refunded P2P stake: "+reason, string(metaJSON))
	return err
}

func (r *EscrowRepo) FinalizeLoserTx(ctx context.Context, tx *sql.Tx, loserUserID string, stake money.Cents, matchID string) error {
	if stake <= 0 {
		return nil
	}
	stakeFloat := stake.Float()

	var walletID string
	var balance float64
	row := tx.QueryRowContext(ctx, `SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, loserUserID)
	if err := row.Scan(&walletID, &balance); err != nil {
		return err
	}

	newBalance := balance - stakeFloat
	_, err := tx.ExecContext(ctx, `UPDATE wallets SET
		balance = balance - $1,
		reserved_balance = GREATEST(0, reserved_balance - $1),
		reserved_cents = GREATEST(0, reserved_cents - $2),
		updated_at = NOW()
		WHERE id = $3`, stakeFloat, int64(stake), walletID)
	if err != nil {
		return err
	}

	// Update this player's stake tx status to completed. The user filter is
	// essential: both players of a match share the same reference, so an
	// unfiltered update would mark the opponent's (possibly winning) stake as
	// settled and leave the loser's row stuck in 'reserved'.
	res, err := tx.ExecContext(ctx, `UPDATE wallet_transactions SET
		status = 'completed',
		balance_after = $1,
		updated_at = NOW()
		WHERE reference = $2 AND type = 'p2p_match_stake' AND user_id = $3 AND status = 'reserved'`,
		newBalance, "p2p_stake:"+matchID, loserUserID)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Fail closed: never move money without the matching ledger reservation.
		return fmt.Errorf("p2p escrow: no reserved stake row for match %s user %s", matchID, loserUserID)
	}

	return nil
}

func (r *EscrowRepo) SettleWinnerTx(ctx context.Context, tx *sql.Tx, winnerUserID string, grossPayout, fee, netPayout money.Cents, matchID string) (string, string, error) {
	if grossPayout <= 0 {
		return "", "", nil
	}

	if netPayout < 0 {
		netPayout = 0
	}
	if fee < 0 {
		fee = 0
	}

	var walletID string
	var balance float64
	row := tx.QueryRowContext(ctx, `SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, winnerUserID)
	if err := row.Scan(&walletID, &balance); err != nil {
		return "", "", err
	}

	// The winner's own stake is held in reserved_balance. Settling means:
	//   balance += netPayout - stake,  reserved_balance -= stake
	// so the final effect is "stake returned + profit", which equals netPayout.
	// For the fixed 2-player pool the stake is exactly half the gross pool.
	stake := grossPayout / 2
	stakeFloat := stake.Float()
	netPayoutFloat := netPayout.Float()
	feeFloat := fee.Float()

	newBalance := balance - stakeFloat + netPayoutFloat

	_, err := tx.ExecContext(ctx, `UPDATE wallets SET
		balance = balance - $1 + $2,
		reserved_balance = GREATEST(0, reserved_balance - $1),
		reserved_cents = GREATEST(0, reserved_cents - $3),
		updated_at = NOW()
		WHERE id = $4`, stakeFloat, netPayoutFloat, int64(stake), walletID)
	if err != nil {
		return "", "", err
	}

	// Close the winner's own reservation row, scoped to the winner only.
	res, err := tx.ExecContext(ctx, `UPDATE wallet_transactions SET
		status = 'completed',
		balance_after = $1,
		updated_at = NOW()
		WHERE reference = $2 AND type = 'p2p_match_stake' AND user_id = $3 AND status = 'reserved'`,
		newBalance, "p2p_stake:"+matchID, winnerUserID)
	if err != nil {
		return "", "", err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return "", "", err
	}
	if affected == 0 {
		return "", "", fmt.Errorf("p2p escrow: no reserved stake row for match %s user %s", matchID, winnerUserID)
	}

	// Credit transaction for winner (only when there is something to credit).
	creditTxID := ""
	if netPayout > 0 {
		creditTxID = newID()
		metaJSON, _ := json.Marshal(map[string]interface{}{
			"match_id":           matchID,
			"gross_payout_cents": int64(grossPayout),
			"fee_cents":          int64(fee),
			"net_payout_cents":   int64(netPayout),
		})

		_, err = tx.ExecContext(ctx, `INSERT INTO wallet_transactions (
			id, wallet_id, user_id, type, status, amount, balance_before, balance_after,
			reference, description, metadata, created_at, updated_at
		) VALUES ($1,$2,$3,'p2p_payout','completed',$4,$5,$6,$7,$8,$9,NOW(),NOW())`,
			creditTxID, walletID, winnerUserID, netPayoutFloat, balance, newBalance,
			"p2p_payout:"+matchID, "Won P2P match prize pool", string(metaJSON))
		if err != nil {
			return "", "", err
		}
	}

	// Platform fee transaction (only when a fee was actually charged).
	feeTxID := ""
	if fee > 0 {
		feeTxID = newID()
		feeMeta, _ := json.Marshal(map[string]interface{}{
			"match_id":    matchID,
			"winner_id":   winnerUserID,
			"service_fee": feeFloat,
			"pool_cents":  int64(grossPayout),
		})

		// Platform fee has NULL wallet_id (per migration 10) or platform user ID
		platformUser := r.platformUserID
		if platformUser == "" {
			platformUser = winnerUserID // attribution fallback
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO wallet_transactions (
			id, wallet_id, user_id, type, status, amount, balance_before, balance_after,
			reference, description, metadata, created_at, updated_at
		) VALUES ($1,NULL,$2,'p2p_platform_fee','completed',$3,0,0,$4,$5,$6,NOW(),NOW())`,
			feeTxID, platformUser, feeFloat,
			"p2p_fee:"+matchID, "Platform service fee from P2P match", string(feeMeta))
		if err != nil {
			return "", "", err
		}
	}

	return creditTxID, feeTxID, nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
