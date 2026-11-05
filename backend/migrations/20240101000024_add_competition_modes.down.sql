DROP INDEX IF EXISTS idx_p2p_matches_mode_lobby;

ALTER TABLE p2p_matches DROP CONSTRAINT IF EXISTS chk_p2p_matches_competition_mode;
ALTER TABLE p2p_matches DROP COLUMN IF EXISTS competition_mode;

DELETE FROM p2p_settings
WHERE key IN ('min_stake_cents', 'max_stake_cents', 'stake_increment_cents');
