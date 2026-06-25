-- Add full_name column to users for payer-name matching
ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name VARCHAR(255);

-- Add phone column to users (kept for future use, not used in SMS flow)
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(20) UNIQUE;
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_users_full_name ON users(full_name);

-- SMS logs table for audit trail and dedup
CREATE TABLE IF NOT EXISTS sms_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sender VARCHAR(50) NOT NULL,
    raw_message TEXT NOT NULL,
    parsed_amount DECIMAL(15,2),
    parsed_sender_name VARCHAR(255),
    parsed_sender_phone VARCHAR(20),
    parsed_transaction_id VARCHAR(255) NOT NULL,
    parsed_timestamp TIMESTAMP,
    credited BOOLEAN NOT NULL DEFAULT FALSE,
    credited_amount DECIMAL(15,2),
    credited_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    credited_wallet_id UUID REFERENCES wallets(id) ON DELETE SET NULL,
    receipt_verified BOOLEAN NOT NULL DEFAULT FALSE,
    receipt_amount DECIMAL(15,2),
    receipt_payer_name VARCHAR(255),
    receipt_status VARCHAR(50),
    error_message TEXT,
    ip_address INET,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Unique constraint on reference for dedup (only for sms_deposit type transactions)
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_tx_sms_ref ON wallet_transactions(reference) WHERE type = 'sms_deposit';
