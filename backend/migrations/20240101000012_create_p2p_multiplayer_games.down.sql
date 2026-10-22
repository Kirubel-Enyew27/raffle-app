-- Migration 20240101000012_create_p2p_multiplayer_games.down.sql
DROP TABLE IF EXISTS p2p_settings;
DROP TABLE IF EXISTS p2p_match_moves;
DROP TABLE IF EXISTS p2p_match_players;
DROP TABLE IF EXISTS p2p_matches;
DROP TABLE IF EXISTS p2p_games;
