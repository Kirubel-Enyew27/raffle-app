-- Migration 20240101000024_add_competition_modes.up.sql
-- Makes free ("compete for fun") and staked ("compete to earn") matches an
-- explicit match-level property instead of inferring the mode from stake_cents.
-- Free matches must never carry a stake and paid matches must always carry one.

ALTER TABLE p2p_matches
    ADD COLUMN IF NOT EXISTS competition_mode VARCHAR(16) NOT NULL DEFAULT 'fun';

-- Existing rows predate the column: any match that reserved a stake was a paid
-- match, everything else is free play.
UPDATE p2p_matches
SET competition_mode = CASE WHEN stake_cents > 0 THEN 'earn' ELSE 'fun' END
WHERE competition_mode = 'fun' AND stake_cents > 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_p2p_matches_competition_mode'
    ) THEN
        ALTER TABLE p2p_matches
            ADD CONSTRAINT chk_p2p_matches_competition_mode
            CHECK (
                (competition_mode = 'fun' AND stake_cents = 0)
                OR (competition_mode = 'earn' AND stake_cents > 0)
            );
    END IF;
END $$;

-- Matchmaking only ever pairs rooms from the same mode, game and stake.
CREATE INDEX IF NOT EXISTS idx_p2p_matches_mode_lobby
    ON p2p_matches (competition_mode, game_slug, stake_cents)
    WHERE status = 'waiting' AND match_type = 'public_queue';

-- Admin-configurable staked-play limits. Amounts are in minor units (cents).
INSERT INTO p2p_settings (key, value, description) VALUES
    ('min_stake_cents',       '1000',  'Minimum Compete to Earn stake in minor units (1000 = 10.00 ETB)'),
    ('max_stake_cents',       '500000','Maximum Compete to Earn stake in minor units (500000 = 5000.00 ETB)'),
    ('stake_increment_cents', '1000',  'Permitted Compete to Earn stake increment in minor units (1000 = 10.00 ETB)')
ON CONFLICT (key) DO NOTHING;
