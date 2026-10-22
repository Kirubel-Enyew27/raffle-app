-- Migration 20240101000014_add_p2p_leaderboard_indexes.down.sql
DROP INDEX IF EXISTS idx_p2p_matches_leaderboard;
DROP INDEX IF EXISTS idx_p2p_matches_completed_at;
DROP INDEX IF EXISTS idx_p2p_match_players_match_user;
