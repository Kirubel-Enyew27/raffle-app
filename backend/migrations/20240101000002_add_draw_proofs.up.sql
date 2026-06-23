-- Add proof columns to raffle_draws
ALTER TABLE raffle_draws ADD COLUMN IF NOT EXISTS server_seed_hash VARCHAR(64);
ALTER TABLE raffle_draws ADD COLUMN IF NOT EXISTS revealed_seed TEXT;
ALTER TABLE raffle_draws ADD COLUMN IF NOT EXISTS client_seed VARCHAR(64);
ALTER TABLE raffle_draws ADD COLUMN IF NOT EXISTS nonce INTEGER;
ALTER TABLE raffle_draws ADD COLUMN IF NOT EXISTS combined_hash VARCHAR(64);
ALTER TABLE raffle_draws ADD COLUMN IF NOT EXISTS winner_ticket_id UUID;

-- Create draw_commitments table for pre-draw seed commitments
CREATE TABLE IF NOT EXISTS draw_commitments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    raffle_id UUID NOT NULL REFERENCES raffles(id) ON DELETE CASCADE UNIQUE,
    server_seed TEXT NOT NULL,
    commit_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_draw_commitments_raffle_id ON draw_commitments(raffle_id);
