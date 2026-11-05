-- Migration 20240101000025_activate_paid_matches.up.sql
-- Compete to Earn is open. The earlier compliance seed left the persisted
-- setting at 'false', which made the API report paid play as disabled and the
-- lobby fall back to free play. Activate the setting here; administrators can
-- still switch it off later from the P2P settings screen.

INSERT INTO p2p_settings (key, value, description) VALUES (
    'paid_matches_enabled',
    'true',
    'Enables Compete to Earn. Wallet balance and stake validation still apply to every paid match.'
)
ON CONFLICT (key) DO UPDATE
SET value = 'true',
    description = EXCLUDED.description;
