-- Multiplayer crash game (Aviator-style).
--
-- Money is stored twice on purpose:
--   * *_cents  BIGINT  -> exact integer minor units used by all game math
--   * *_amount DECIMAL(15,2) -> mirrors the existing wallet/ledger precision so
--                              reporting can join against wallet_transactions
-- The integer columns are authoritative for the game itself.

-- Monotonic round numbering, race-free without an extra SELECT MAX().
CREATE SEQUENCE IF NOT EXISTS crash_round_number_seq START 1;

CREATE TABLE IF NOT EXISTS game_rounds (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    room                    VARCHAR(50) NOT NULL DEFAULT 'default',
    round_number            BIGINT NOT NULL,
    state                   VARCHAR(20) NOT NULL,

    -- Commit-reveal fairness metadata.
    -- server_seed is never exposed through the API while the round is live.
    server_seed             VARCHAR(128) NOT NULL,
    server_seed_hash        VARCHAR(128) NOT NULL,
    nonce                   BIGINT NOT NULL DEFAULT 1,
    commitment_published_at TIMESTAMPTZ NOT NULL,

    -- Authoritative timeline. All multiplier/cash-out math derives from these.
    betting_starts_at       TIMESTAMPTZ NOT NULL,
    betting_ends_at         TIMESTAMPTZ NOT NULL,
    started_at              TIMESTAMPTZ,
    crashed_at              TIMESTAMPTZ,
    settled_at              TIMESTAMPTZ,

    -- Derived from the seed at crash time (NULL until then).
    crash_multiplier_bp     INTEGER,

    total_stake_cents       BIGINT NOT NULL DEFAULT 0,
    total_payout_cents      BIGINT NOT NULL DEFAULT 0,
    bet_count               INTEGER NOT NULL DEFAULT 0,
    cashout_count           INTEGER NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- At most one live (money-moving) round per room. CRASHED / SETTLEMENT /
-- NEXT_ROUND are post-crash states and are excluded so the engine can create
-- the next round while displaying the previous result.
CREATE UNIQUE INDEX IF NOT EXISTS uq_game_rounds_live_room
    ON game_rounds (room)
    WHERE state IN ('WAITING', 'BETTING', 'RUNNING');

CREATE INDEX IF NOT EXISTS idx_game_rounds_room_number
    ON game_rounds (room, round_number DESC);

CREATE INDEX IF NOT EXISTS idx_game_rounds_settled
    ON game_rounds (settled_at DESC)
    WHERE state IN ('SETTLEMENT', 'NEXT_ROUND');

CREATE TABLE IF NOT EXISTS game_bets (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    round_id                UUID NOT NULL REFERENCES game_rounds(id) ON DELETE CASCADE,
    user_id                 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id               UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,

    stake_cents             BIGINT NOT NULL CHECK (stake_cents > 0),
    stake_amount            DECIMAL(15,2) NOT NULL CHECK (stake_amount > 0),

    -- active -> cashed_out | lost (terminal)
    status                  VARCHAR(20) NOT NULL DEFAULT 'active',

    -- Optional player-set automatic cash-out target.
    auto_cashout_bp         INTEGER,

    cashout_multiplier_bp   INTEGER,
    payout_cents            BIGINT NOT NULL DEFAULT 0,
    payout_amount           DECIMAL(15,2) NOT NULL DEFAULT 0,

    -- Ledger links (existing wallet_transactions table).
    debit_tx_id             UUID REFERENCES wallet_transactions(id) ON DELETE SET NULL,
    credit_tx_id            UUID REFERENCES wallet_transactions(id) ON DELETE SET NULL,

    cashed_out_at           TIMESTAMPTZ,
    settled_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One bet per user per round. This is the primary duplicate-bet guard and
    -- holds even if two API requests race or arrive from different instances.
    CONSTRAINT uq_game_bets_round_user UNIQUE (round_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_game_bets_round ON game_bets (round_id);
CREATE INDEX IF NOT EXISTS idx_game_bets_round_status ON game_bets (round_id, status);
CREATE INDEX IF NOT EXISTS idx_game_bets_user_created ON game_bets (user_id, created_at DESC);
