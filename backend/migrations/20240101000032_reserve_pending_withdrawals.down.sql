-- Migration 20240101000032_reserve_pending_withdrawals.down.sql
-- Releases any reservations held for pending withdrawals

UPDATE wallets w
SET
    reserved_balance = GREATEST(0, w.reserved_balance - p.total_amount),
    reserved_cents = GREATEST(0, w.reserved_cents - ROUND(p.total_amount * 100)),
    updated_at = NOW()
FROM (
    SELECT
        wallet_id,
        SUM(amount) AS total_amount
    FROM wallet_transactions
    WHERE type = 'withdrawal' AND status = 'pending'
    GROUP BY wallet_id
) p
WHERE w.id = p.wallet_id;
