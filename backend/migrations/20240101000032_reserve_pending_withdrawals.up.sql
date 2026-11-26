-- Migration 20240101000032_reserve_pending_withdrawals.up.sql
-- Pending withdrawals must reserve funds in escrow immediately so they cannot be double-spent.
-- For any pre-existing pending withdrawals:
-- If the wallet has sufficient available balance, reserve the pending amount in reserved_balance.
-- If the wallet balance was already spent below the pending amount, reserve the available amount.

UPDATE wallets w
SET
    reserved_balance = w.reserved_balance + LEAST(GREATEST(0, w.balance - w.reserved_balance), p.total_amount),
    reserved_cents = w.reserved_cents + ROUND(LEAST(GREATEST(0, w.balance - w.reserved_balance), p.total_amount) * 100),
    updated_at = NOW()
FROM (
    SELECT
        wallet_id,
        SUM(amount) AS total_amount
    FROM wallet_transactions
    WHERE type = 'withdrawal' AND status = 'pending'
    GROUP BY wallet_id
) p
WHERE w.id = p.wallet_id
  AND w.balance > w.reserved_balance;
