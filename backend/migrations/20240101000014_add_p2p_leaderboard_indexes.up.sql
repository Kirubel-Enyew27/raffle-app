-- Migration 20240101000014_add_p2p_leaderboard_indexes.up.sql
-- Optimizes player leaderboard aggregations, match history, and streak queries

CREATE INDEX IF NOT EXISTS idx_p2p_matches_leaderboard 
ON p2p_matches (game_slug, status, completed_at)
WHERE status IN ('completed', 'forfeited', 'tied');

CREATE INDEX IF NOT EXISTS idx_p2p_matches_completed_at
ON p2p_matches (completed_at)
WHERE status IN ('completed', 'forfeited', 'tied');

CREATE INDEX IF NOT EXISTS idx_p2p_match_players_match_user
ON p2p_match_players (match_id, user_id);
