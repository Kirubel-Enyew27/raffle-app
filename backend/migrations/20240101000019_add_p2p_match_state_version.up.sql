-- Every persisted match snapshot gets a consecutive version. Clients use it to
-- ignore delayed events and detect missing updates across SSE reconnects.
ALTER TABLE p2p_matches
    ADD COLUMN IF NOT EXISTS state_version BIGINT NOT NULL DEFAULT 1;
