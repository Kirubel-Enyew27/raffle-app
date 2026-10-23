-- Migration 20240101000016_add_ethiopian_dama.up.sql
-- Adds Ethiopian Dama (ዳማ) as the 4th P2P multiplayer game.
--
-- The ruleset is documented in docs/ETHIOPIAN_DAMA.md. rules_json is the
-- server-authoritative configuration surface for regional variations; changing
-- a default only requires updating this row and the engine's DefaultRuleset.
--
-- Dama is a single-round game (max_rounds is chosen in code as the default of
-- 1 for every game except Tic-Tac-Toe and Rock-Paper-Scissors).

INSERT INTO p2p_games (
    slug, title, description, min_players, max_players, is_active,
    min_stake_cents, max_stake_cents, default_fee_bp, default_turn_seconds, rules_json
) VALUES (
    'ethiopian-dama',
    'Ethiopian Dama (ዳማ)',
    'Traditional Ethiopian draughts on an 8x8 board with 12 men per side. Captures are mandatory and chain, and kings fly across the board. Free play while paid matches remain compliance-gated.',
    2, 2, TRUE, 0, 500000, 500, 30,
    '{"variant":"ethiopian-dama","board_size":8,"pieces_per_side":12,"men_move_backwards":false,"men_capture_backwards":false,"flying_kings":true,"mandatory_capture":true,"majority_capture":false,"promotion_during_capture":false,"halfmove_draw_plies":100,"repetition_draw":3,"max_rounds":1}'::jsonb
) ON CONFLICT (slug) DO UPDATE SET is_active = TRUE;
