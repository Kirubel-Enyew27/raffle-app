-- Migration 20240101000015_add_p2p_rematch.up.sql
-- Rematch handshake for finished matches: `rematch_requested_by` records who
-- asked for a return game, and `rematch_match_id` points at the fresh match that
-- was created once both players agreed.

ALTER TABLE p2p_matches
    ADD COLUMN IF NOT EXISTS rematch_requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS rematch_match_id     UUID REFERENCES p2p_matches(id) ON DELETE SET NULL;
