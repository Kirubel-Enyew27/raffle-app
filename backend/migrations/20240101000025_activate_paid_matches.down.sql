-- Migration 20240101000025_activate_paid_matches.down.sql
UPDATE p2p_settings
SET value = 'false',
    description = 'Compliance toggle: controls whether matches with real-money stakes can be created.'
WHERE key = 'paid_matches_enabled';
