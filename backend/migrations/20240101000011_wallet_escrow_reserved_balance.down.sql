-- Migration 20240101000011_wallet_escrow_reserved_balance.down.sql
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS chk_wallets_reserved_le_balance;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS chk_wallets_reserved_positive;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS chk_wallets_balance_positive;
ALTER TABLE wallets DROP COLUMN IF EXISTS reserved_cents;
ALTER TABLE wallets DROP COLUMN IF EXISTS reserved_balance;
