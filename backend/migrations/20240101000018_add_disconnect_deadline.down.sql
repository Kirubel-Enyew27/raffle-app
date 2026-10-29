DROP INDEX IF EXISTS idx_p2p_matches_disconnect_deadline;
ALTER TABLE p2p_matches DROP COLUMN IF EXISTS disconnect_deadline;
ALTER TABLE p2p_matches DROP COLUMN IF EXISTS disconnect_user_id;
