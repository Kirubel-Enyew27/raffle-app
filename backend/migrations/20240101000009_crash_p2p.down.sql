-- Reverses 20240101000009_crash_p2p.up.sql

ALTER TABLE game_bets
    DROP COLUMN IF EXISTS fee_tx_id,
    DROP COLUMN IF EXISTS net_payout_cents,
    DROP COLUMN IF EXISTS service_fee_cents,
    DROP COLUMN IF EXISTS gross_profit_cents,
    DROP COLUMN IF EXISTS gross_payout_cents;

ALTER TABLE game_rounds
    DROP COLUMN IF EXISTS total_fee_cents,
    DROP COLUMN IF EXISTS p2p_mode,
    DROP COLUMN IF EXISTS min_participants,
    DROP COLUMN IF EXISTS service_fee_bp,
    DROP COLUMN IF EXISTS prize_pool_cents;

DROP TABLE IF EXISTS crash_settings;
