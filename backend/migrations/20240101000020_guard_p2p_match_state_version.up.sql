-- Older application instances may still update match state during a rolling
-- deploy without knowing about state_version. Keep the sequence authoritative
-- at the database boundary until all writers have been upgraded.
CREATE OR REPLACE FUNCTION bump_legacy_p2p_match_state_version()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.state_version = OLD.state_version AND (
        NEW.status IS DISTINCT FROM OLD.status OR
        NEW.current_turn_user_id IS DISTINCT FROM OLD.current_turn_user_id OR
        NEW.turn_deadline IS DISTINCT FROM OLD.turn_deadline OR
        NEW.current_round IS DISTINCT FROM OLD.current_round OR
        NEW.max_rounds IS DISTINCT FROM OLD.max_rounds OR
        NEW.scores IS DISTINCT FROM OLD.scores OR
        NEW.board_state IS DISTINCT FROM OLD.board_state OR
        NEW.rematch_requested_by IS DISTINCT FROM OLD.rematch_requested_by OR
        NEW.rematch_match_id IS DISTINCT FROM OLD.rematch_match_id OR
        NEW.disconnect_user_id IS DISTINCT FROM OLD.disconnect_user_id OR
        NEW.disconnect_deadline IS DISTINCT FROM OLD.disconnect_deadline
    ) THEN
        NEW.state_version := OLD.state_version + 1;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER p2p_match_state_version_legacy_guard
BEFORE UPDATE ON p2p_matches
FOR EACH ROW
EXECUTE FUNCTION bump_legacy_p2p_match_state_version();
