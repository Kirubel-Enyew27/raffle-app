-- Migrate winners table to full winner management schema
-- Add draw_id column (FK to raffle_draws)
ALTER TABLE winners ADD COLUMN IF NOT EXISTS draw_id UUID REFERENCES raffle_draws(id) ON DELETE CASCADE;

-- Add prize_paid / payment columns (replacing claimed/claimed_at)
ALTER TABLE winners ADD COLUMN IF NOT EXISTS prize_paid BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE winners ADD COLUMN IF NOT EXISTS payment_date TIMESTAMP;
ALTER TABLE winners ADD COLUMN IF NOT EXISTS payment_reference VARCHAR(255);
ALTER TABLE winners ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NOT NULL DEFAULT NOW();
ALTER TABLE winners ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT NOW();

-- Add ticket_id column if missing (original schema had user_id but not ticket_id)
ALTER TABLE winners ADD COLUMN IF NOT EXISTS ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE;

-- Prevent duplicate winner records for same draw + ticket
CREATE UNIQUE INDEX IF NOT EXISTS idx_winners_draw_ticket ON winners(draw_id, ticket_id);

-- Index for common queries
CREATE INDEX IF NOT EXISTS idx_winners_raffle_id ON winners(raffle_id);
CREATE INDEX IF NOT EXISTS idx_winners_draw_id ON winners(draw_id);
CREATE INDEX IF NOT EXISTS idx_winners_user_id ON winners(user_id);

-- Add status column to raffle_draws if missing
ALTER TABLE raffle_draws ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'completed';
