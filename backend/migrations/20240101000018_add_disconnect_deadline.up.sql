-- Add disconnect tracking columns for the reconnection grace period.
-- disconnect_user_id: the user_id of the player who has temporarily disconnected.
--   Cleared when they reconnect or when the match is forfeited due to expiry.
-- disconnect_deadline: the server-authoritative deadline. If the player has not
--   reconnected by this time, the background engine forfeits the match.
ALTER TABLE p2p_matches ADD COLUMN IF NOT EXISTS disconnect_user_id UUID REFERENCES users(id);
ALTER TABLE p2p_matches ADD COLUMN IF NOT EXISTS disconnect_deadline TIMESTAMPTZ;

-- Index for the background sweep to efficiently find expired disconnects.
CREATE INDEX IF NOT EXISTS idx_p2p_matches_disconnect_deadline
    ON p2p_matches (disconnect_deadline)
    WHERE status = 'in_progress' AND disconnect_deadline IS NOT NULL;
