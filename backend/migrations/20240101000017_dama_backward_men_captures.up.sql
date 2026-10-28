-- Migration 20240101000017_dama_backward_men_captures.up.sql
-- Ethiopian Dama now lets ordinary pieces capture diagonally forwards *and*
-- backwards. Ordinary (non-capturing) movement is unchanged: men still move
-- forwards only, because men_move_backwards stays false.
--
-- The engine's DefaultRuleset already enforces this; rules_json only mirrors
-- the enforced rules for display, so keep it in sync for existing rows.

UPDATE p2p_games
SET rules_json = jsonb_set(rules_json, '{men_capture_backwards}', 'true'::jsonb, true),
    updated_at = NOW()
WHERE slug = 'ethiopian-dama';
