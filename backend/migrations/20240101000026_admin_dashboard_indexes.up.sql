-- Admin dashboard range and live queries. Additive only; existing queries keep
-- working if this migration is rolled back.
CREATE INDEX IF NOT EXISTS idx_p2p_matches_admin_created
    ON p2p_matches (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_p2p_matches_admin_active
    ON p2p_matches (created_at DESC)
    WHERE status IN ('waiting', 'ready', 'in_progress');

CREATE INDEX IF NOT EXISTS idx_wallet_tx_created_at
    ON wallet_transactions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_wallet_tx_type_status_created
    ON wallet_transactions (type, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_wallet_tx_user_created
    ON wallet_transactions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at
    ON audit_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_users_active_created
    ON users (created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_winners_paid_payment
    ON winners (payment_date DESC)
    WHERE prize_paid = TRUE;
