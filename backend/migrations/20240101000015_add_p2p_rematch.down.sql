-- Migration 20240101000015_add_p2p_rematch.down.sql

ALTER TABLE p2p_matches
    DROP COLUMN IF EXISTS rematch_match_id,
    DROP COLUMN IF EXISTS rematch_requested_by;
