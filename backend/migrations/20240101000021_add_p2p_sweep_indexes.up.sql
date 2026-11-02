-- Background sweeps run on every instance: expired turns and expired disconnect
-- grace periods every second, idle waiting rooms every thirty seconds. Without a
-- supporting index each of those queries sequentially scans p2p_matches and
-- reads every candidate row's board_state (TOAST), only to re-read the winner
-- under FOR UPDATE. Measured baseline: 1.57 ms per sweep at 6.5k rows with 415
-- shared buffers, executed once per second per instance.
CREATE INDEX IF NOT EXISTS idx_p2p_matches_turn_deadline
    ON p2p_matches (turn_deadline)
    WHERE status = 'in_progress' AND turn_deadline IS NOT NULL;

-- FindIdleWaitingRooms filters on status + created_at; the existing lobby index
-- is ordered by (game_slug, status, match_type) and cannot serve a range scan on
-- created_at.
CREATE INDEX IF NOT EXISTS idx_p2p_matches_idle_waiting
    ON p2p_matches (created_at)
    WHERE status = 'waiting';
