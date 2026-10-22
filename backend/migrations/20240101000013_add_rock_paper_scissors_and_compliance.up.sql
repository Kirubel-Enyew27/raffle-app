-- Migration 20240101000013_add_rock_paper_scissors_and_compliance.up.sql
-- Adds Rock-Paper-Scissors (Bo3) as the 3rd P2P multiplayer game
-- Adds default compliance setting for paid matches (disabled by default)

INSERT INTO p2p_games (
    slug, title, description, min_players, max_players, is_active,
    min_stake_cents, max_stake_cents, default_fee_bp, default_turn_seconds, rules_json
) VALUES (
    'rock-paper-scissors',
    'Rock-Paper-Scissors (Bo3)',
    'Simultaneous mind game played as Best-of-3 rounds with hidden move commitments. Rock crushes Scissors, Scissors cuts Paper, Paper covers Rock.',
    2, 2, TRUE, 0, 500000, 500, 15,
    '{"target_wins": 2, "max_rounds": 5}'::jsonb
) ON CONFLICT (slug) DO NOTHING;

INSERT INTO p2p_settings (key, value, description) VALUES
    ('paid_matches_enabled', 'false', 'Compliance toggle: controls whether matches with real-money stakes can be created. Must remain false until legal and licensing clearance.')
ON CONFLICT (key) DO NOTHING;
