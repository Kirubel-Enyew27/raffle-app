-- Remove SMS deposit unique index
DROP INDEX IF EXISTS idx_wallet_tx_sms_ref;

-- Drop SMS logs table
DROP TABLE IF EXISTS sms_logs;

-- Remove phone column from users
DROP INDEX IF EXISTS idx_users_phone;
ALTER TABLE users DROP COLUMN IF EXISTS phone;
