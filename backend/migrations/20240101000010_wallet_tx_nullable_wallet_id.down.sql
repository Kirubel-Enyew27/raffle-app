-- Reverse: restore the NOT NULL constraint on wallet_transactions.wallet_id.
-- Note: this will fail if any existing rows have NULL wallet_id (platform
-- ledger entries). Run a cleanup of those rows before applying in production.

ALTER TABLE wallet_transactions
    ALTER COLUMN wallet_id SET NOT NULL;
