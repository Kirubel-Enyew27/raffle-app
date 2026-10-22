-- The crash P2P game records platform-fee and pool-revenue ledger entries in
-- wallet_transactions. These entries represent ledger movements that are not
-- tied to a single player wallet (the fee is collected by the platform, not
-- credited to an existing wallet balance). The original schema made wallet_id
-- NOT NULL because every transaction was a real wallet debit/credit. Platform
-- ledger entries have no owning wallet, so wallet_id must be allowed to be
-- NULL.
--
-- user_id remains NOT NULL: every platform entry is attributed to a configured
-- platform user account, so the FK to users(id) is still satisfied.

ALTER TABLE wallet_transactions
    ALTER COLUMN wallet_id DROP NOT NULL;
