-- Migration 20240101000012_create_p2p_multiplayer_games.up.sql
-- Creates schema for peer-to-peer multiplayer games, match sessions, players,
-- moves, and configurable game rules.

CREATE TABLE IF NOT EXISTS p2p_games (
    slug                VARCHAR(64) PRIMARY KEY,
    title               VARCHAR(128) NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    min_players         INT NOT NULL DEFAULT 2,
    max_players         INT NOT NULL DEFAULT 2,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    min_stake_cents     BIGINT NOT NULL DEFAULT 1000,
    max_stake_cents     BIGINT NOT NULL DEFAULT 500000,
    default_fee_bp      INT NOT NULL DEFAULT 500,
    default_turn_seconds INT NOT NULL DEFAULT 20,
    rules_json          JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS p2p_matches (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    game_slug           VARCHAR(64) NOT NULL REFERENCES p2p_games(slug),
    status              VARCHAR(32) NOT NULL DEFAULT 'waiting',
    match_type          VARCHAR(32) NOT NULL DEFAULT 'public_queue',
    invite_code         VARCHAR(16) UNIQUE,
    stake_cents         BIGINT NOT NULL DEFAULT 0,
    stake_amount        DECIMAL(15,2) NOT NULL DEFAULT 0.00,
    prize_pool_cents    BIGINT NOT NULL DEFAULT 0,
    service_fee_bp      INT NOT NULL DEFAULT 500,
    total_fee_cents     BIGINT NOT NULL DEFAULT 0,
    winner_user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    current_turn_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    turn_deadline       TIMESTAMPTZ,
    current_round       INT NOT NULL DEFAULT 1,
    max_rounds          INT NOT NULL DEFAULT 3,
    scores              JSONB NOT NULL DEFAULT '{}'::jsonb,
    board_state         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by          UUID NOT NULL REFERENCES users(id),
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS p2p_match_players (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    match_id            UUID NOT NULL REFERENCES p2p_matches(id) ON DELETE CASCADE,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_index        INT NOT NULL,
    symbol              VARCHAR(16) NOT NULL DEFAULT '',
    status              VARCHAR(32) NOT NULL DEFAULT 'reserved',
    score               INT NOT NULL DEFAULT 0,
    debit_tx_id         UUID REFERENCES wallet_transactions(id) ON DELETE SET NULL,
    credit_tx_id        UUID REFERENCES wallet_transactions(id) ON DELETE SET NULL,
    fee_tx_id           UUID REFERENCES wallet_transactions(id) ON DELETE SET NULL,
    joined_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ready_at            TIMESTAMPTZ,
    CONSTRAINT uq_p2p_match_player_match_user UNIQUE (match_id, user_id),
    CONSTRAINT uq_p2p_match_player_match_index UNIQUE (match_id, player_index)
);

CREATE TABLE IF NOT EXISTS p2p_match_moves (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    match_id            UUID NOT NULL REFERENCES p2p_matches(id) ON DELETE CASCADE,
    round_number        INT NOT NULL DEFAULT 1,
    move_number         INT NOT NULL,
    user_id             UUID NOT NULL REFERENCES users(id),
    action              JSONB NOT NULL,
    board_snapshot      JSONB NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_p2p_match_moves_order UNIQUE (match_id, round_number, move_number)
);

CREATE TABLE IF NOT EXISTS p2p_settings (
    key                 VARCHAR(64) PRIMARY KEY,
    value               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    updated_by          UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for matchmaking and query efficiency
CREATE INDEX IF NOT EXISTS idx_p2p_matches_lobby ON p2p_matches(game_slug, status, match_type) WHERE status = 'waiting';
CREATE INDEX IF NOT EXISTS idx_p2p_matches_created_by ON p2p_matches(created_by);
CREATE INDEX IF NOT EXISTS idx_p2p_match_players_user ON p2p_match_players(user_id);
CREATE INDEX IF NOT EXISTS idx_p2p_match_moves_match ON p2p_match_moves(match_id, round_number);

-- Seed initial games
INSERT INTO p2p_games (slug, title, description, min_players, max_players, is_active, min_stake_cents, max_stake_cents, default_fee_bp, default_turn_seconds, rules_json)
VALUES
    ('tic-tac-toe', 'Tic-Tac-Toe Blitz (Bo3)', 'Classic 3x3 strategic match played as Best-of-3 rounds with a 15-second blitz move clock.', 2, 2, TRUE, 0, 500000, 500, 15, '{"grid_size": 3, "target_wins": 2, "max_rounds": 3}'::jsonb),
    ('connect-four', 'Connect Four', 'Drop your chips into a 7x6 grid. First to align four chips in a line horizontally, vertically, or diagonally wins.', 2, 2, TRUE, 0, 500000, 500, 20, '{"columns": 7, "rows": 6, "target_wins": 1, "max_rounds": 1}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- Seed default P2P settings
INSERT INTO p2p_settings (key, value, description) VALUES
    ('default_service_fee_bp',     '500',  'Platform service fee percentage on winner prize pool (500 = 5.00%)'),
    ('reconnect_grace_seconds',    '45',   'Seconds allowed for disconnected player to reconnect before forfeiture'),
    ('idle_room_timeout_seconds',  '300',  'Seconds before an unfilled private match room auto-expires and refunds creator'),
    ('turn_timer_default_seconds', '20',   'Default turn timer duration when not overridden by game rule')
ON CONFLICT (key) DO NOTHING;
