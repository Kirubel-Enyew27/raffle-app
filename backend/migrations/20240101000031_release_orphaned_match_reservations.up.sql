-- Migration 20240101000031_release_orphaned_match_reservations.up.sql
-- A reservation that points at no existing match can never be settled or
-- refunded by normal match lifecycle code. Older test data contains rows with
-- an empty match id (`p2p_stake:`), and any deleted room can leave the same
-- shape. Close only these matchless reservations; every reservation tied to an
-- existing match, active or terminal, is handled by migration 30 or the normal
-- settlement paths.

CREATE TEMP TABLE orphaned_match_reservations AS
SELECT
    wt.id AS transaction_id,
    wt.wallet_id,
    wt.user_id,
    wt.amount,
    wt.reference,
    w.balance,
    w.reserved_balance,
    SUM(wt.amount) OVER (PARTITION BY wt.wallet_id) AS wallet_orphan_total
FROM wallet_transactions wt
JOIN wallets w ON w.id = wt.wallet_id
WHERE wt.type = 'p2p_match_stake'
  AND wt.status = 'reserved'
  AND NOT EXISTS (
      SELECT 1 FROM p2p_matches m
      WHERE wt.reference = 'p2p_stake:' || m.id
  );

-- Release the wallet hold only when the wallet holds at least the full amount
-- of its orphaned rows. This avoids consuming a reservation that belongs to an
-- active match if the ledger and wallet are temporarily inconsistent.
UPDATE wallets w
SET
    reserved_balance = GREATEST(0, w.reserved_balance - orphaned.total_amount),
    reserved_cents = GREATEST(0, w.reserved_cents - orphaned.total_cents),
    updated_at = NOW()
FROM (
    SELECT
        wallet_id,
        SUM(amount) AS total_amount,
        SUM(ROUND(amount * 100)) AS total_cents
    FROM orphaned_match_reservations
    GROUP BY wallet_id, reserved_balance
    HAVING reserved_balance >= SUM(amount)
) orphaned
WHERE w.id = orphaned.wallet_id;

INSERT INTO wallet_transactions (
    wallet_id, user_id, type, status, amount, balance_before, balance_after,
    reference, description, metadata, created_at, updated_at
)
SELECT
    wallet_id,
    user_id,
    'p2p_stake_refund',
    'completed',
    amount,
    balance,
    balance,
    'p2p_refund:orphaned:' || transaction_id,
    'Refunded P2P stake: orphaned match reservation reconciliation',
    jsonb_build_object(
        'reason', 'orphaned match reservation reconciliation',
        'original_reference', reference
    ),
    NOW(),
    NOW()
FROM orphaned_match_reservations
WHERE reserved_balance >= wallet_orphan_total;

UPDATE wallet_transactions wt
SET
    status = 'refunded',
    balance_after = orphaned.balance,
    updated_at = NOW()
FROM orphaned_match_reservations orphaned
WHERE wt.id = orphaned.transaction_id;
