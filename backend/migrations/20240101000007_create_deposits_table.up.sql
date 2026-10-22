CREATE TABLE IF NOT EXISTS deposits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    amount DECIMAL(15,2) NOT NULL CHECK (amount >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'ETB',
    payment_method VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    external_reference VARCHAR(255),
    receipt_token VARCHAR(255),
    receipt_url TEXT,
    payer_name VARCHAR(255),
    payer_account VARCHAR(255),
    receiver_name VARCHAR(255),
    receiver_account VARCHAR(255),
    actual_amount_credited DECIMAL(15,2),
    fee_amount DECIMAL(15,2),
    error_message TEXT,
    metadata JSONB,
    verified_at TIMESTAMP,
    credited_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '30 minutes'),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Unique index prevents duplicate credit of the same external bank transaction
CREATE UNIQUE INDEX IF NOT EXISTS idx_deposits_external_ref
ON deposits (external_reference)
WHERE external_reference IS NOT NULL AND status IN ('verified', 'credited');

-- Ensure wallet_transactions has a hard unique index on deposit references
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_tx_deposit_ref
ON wallet_transactions (reference)
WHERE type = 'deposit';

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_deposits_user_id ON deposits(user_id);
CREATE INDEX IF NOT EXISTS idx_deposits_status ON deposits(status);
CREATE INDEX IF NOT EXISTS idx_deposits_created_at ON deposits(created_at DESC);
