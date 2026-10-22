-- Migration 20240101000011_wallet_escrow_reserved_balance.up.sql
-- Adds reserved balance (escrow) support to wallets to protect active P2P match
-- stakes and pending withdrawals from double-spending.

ALTER TABLE wallets
    ADD COLUMN IF NOT EXISTS reserved_balance DECIMAL(15,2) NOT NULL DEFAULT 0.00,
    ADD COLUMN IF NOT EXISTS reserved_cents BIGINT NOT NULL DEFAULT 0;

-- Ensure balances can never go negative and reserved cannot exceed total balance
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_wallets_balance_positive'
    ) THEN
        ALTER TABLE wallets ADD CONSTRAINT chk_wallets_balance_positive CHECK (balance >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_wallets_reserved_positive'
    ) THEN
        ALTER TABLE wallets ADD CONSTRAINT chk_wallets_reserved_positive CHECK (reserved_balance >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_wallets_reserved_le_balance'
    ) THEN
        ALTER TABLE wallets ADD CONSTRAINT chk_wallets_reserved_le_balance CHECK (reserved_balance <= balance);
    END IF;
END $$;
