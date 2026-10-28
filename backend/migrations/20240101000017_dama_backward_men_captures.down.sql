-- Restore the previous documented metadata. The rules engine default is not
-- changed by a migration; only the p2p_games catalogue description is reverted.

UPDATE p2p_games
SET rules_json = jsonb_set(rules_json, '{men_capture_backwards}', 'false'::jsonb, true),
    updated_at = NOW()
WHERE slug = 'ethiopian-dama';
