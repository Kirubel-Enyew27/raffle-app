-- P2P crash game: extends the existing game_rounds and game_bets tables with
-- peer-to-peer prize pool, winner-paid service fee, and admin settings.
--
-- All monetary additions use the same dual-storage pattern already established:
--   *_cents  BIGINT  -> authoritative integer minor units for game math
--   *_amount DECIMAL(15,2) -> mirrors wallet/ledger precision for reporting

-- ---------------------------------------------------------------------------
-- 1. Admin-configurable game settings
-- ---------------------------------------------------------------------------
-- Stores the live and snapshot configuration for the P2P game. One row per
-- setting key. The engine snapshots relevant values onto game_rounds at round
-- creation time so an in-flight round is never affected by a setting change.

CREATE TABLE IF NOT EXISTS crash_settings (
    key         VARCHAR(64) PRIMARY KEY,
    value       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    updated_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the default values so the engine always has a row to read.
INSERT INTO crash_settings (key, value, description) VALUES
    ('service_fee_bp',    '500',   'Service fee charged on winner profit, in basis points (500 = 5.00%)'),
    ('min_participants',  '2',     'Minimum players required to start a round'),
    ('max_participants',  '100',   'Maximum players allowed in a round'),
    ('min_bet_cents',     '1000',  'Minimum stake in minor units (1000 = 10.00)'),
    ('max_bet_cents',     '1000000', 'Maximum stake in minor units (1000000 = 10000.00)'),
    ('betting_seconds',   '30',    'Duration of the betting window in seconds'),
    ('waiting_seconds',   '5',     'Pre-betting display window in seconds'),
    ('crashed_seconds',   '5',     'Post-crash display window in seconds')
ON CONFLICT (key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. Extend game_rounds with P2P columns
-- ---------------------------------------------------------------------------

-- Prize pool (sum of all accepted stakes for the round).
ALTER TABLE game_rounds
    ADD COLUMN IF NOT EXISTS prize_pool_cents      BIGINT      NOT NULL DEFAULT 0;

-- Snapshot of the service fee basis points in effect when this round was created.
-- Snapshotting means a settings change never retroactively alters a live round.
ALTER TABLE game_rounds
    ADD COLUMN IF NOT EXISTS service_fee_bp        INTEGER     NOT NULL DEFAULT 500;

-- Minimum participants required to start; also snapshotted at round creation.
ALTER TABLE game_rounds
    ADD COLUMN IF NOT EXISTS min_participants       INTEGER     NOT NULL DEFAULT 2;

-- P2P mode flag (true = peer-to-peer; false = legacy house model).
-- All new rounds created by this code set this to true.
ALTER TABLE game_rounds
    ADD COLUMN IF NOT EXISTS p2p_mode              BOOLEAN     NOT NULL DEFAULT TRUE;

-- Total service fee collected for the round (sum of all winner fees).
ALTER TABLE game_rounds
    ADD COLUMN IF NOT EXISTS total_fee_cents        BIGINT      NOT NULL DEFAULT 0;

-- ---------------------------------------------------------------------------
-- 3. Extend game_bets with P2P settlement columns
-- ---------------------------------------------------------------------------

-- gross_payout_cents: the winner's raw share of the prize pool before the fee.
-- For losers this stays 0. For cashed-out bets this is set at P2P settlement.
ALTER TABLE game_bets
    ADD COLUMN IF NOT EXISTS gross_payout_cents    BIGINT      NOT NULL DEFAULT 0;

-- gross_profit_cents: max(0, gross_payout_cents - stake_cents). Zero for losers.
ALTER TABLE game_bets
    ADD COLUMN IF NOT EXISTS gross_profit_cents    BIGINT      NOT NULL DEFAULT 0;

-- service_fee_cents: gross_profit_cents * service_fee_bp / 10000. Zero for
-- losers and winners whose gross return does not exceed their original stake.
ALTER TABLE game_bets
    ADD COLUMN IF NOT EXISTS service_fee_cents     BIGINT      NOT NULL DEFAULT 0;

-- net_payout_cents: gross_payout_cents - service_fee_cents. The amount actually
-- credited to the player's wallet.
ALTER TABLE game_bets
    ADD COLUMN IF NOT EXISTS net_payout_cents      BIGINT      NOT NULL DEFAULT 0;

-- fee_tx_id: ledger link for the service-fee debit transaction.
ALTER TABLE game_bets
    ADD COLUMN IF NOT EXISTS fee_tx_id             UUID        REFERENCES wallet_transactions(id) ON DELETE SET NULL;

-- ---------------------------------------------------------------------------
-- 4. Platform revenue ledger link
-- ---------------------------------------------------------------------------
-- When a winner pays a service fee, a "crash_platform_fee" wallet_transaction
-- row is created (type = 'crash_platform_fee'). The fee_tx_id column above
-- links the bet to that transaction. No additional table is required because
-- the platform's wallet_transactions table already handles this: filtering by
-- type = 'crash_platform_fee' gives the full fee revenue ledger.

-- ---------------------------------------------------------------------------
-- 5. Index additions
-- ---------------------------------------------------------------------------

CREATE INDEX IF NOT EXISTS idx_crash_settings_key
    ON crash_settings (key);

CREATE INDEX IF NOT EXISTS idx_game_rounds_p2p
    ON game_rounds (p2p_mode, state, room);
