-- Migration 20240101000030_release_terminal_match_reservations.up.sql
-- Older builds could finish a match without settling when one player's stake
-- reservation was missing, leaving the opponent's reservation held for ever.
-- Cancelled test rooms were also marked terminal without releasing their stake.
--
-- A terminal match can never legitimately hold escrow: its settlement or refund
-- should already have happened. Close those stale reservations now. This does
-- not touch active matches and does not delete or change the amount of any
-- historical ledger row. The original reservation is marked refunded and a
-- matching audit row records the release, exactly like a normal cancellation.

CREATE TEMP TABLE stale_terminal_reservations AS
SELECT
    wt.id AS transaction_id,
    wt.wallet_id,
    wt.user_id,
    wt.amount,
    wt.reference,
    w.balance
FROM wallet_transactions wt
JOIN wallets w ON w.id = wt.wallet_id
JOIN p2p_matches m ON wt.reference = 'p2p_stake:' || m.id
WHERE wt.type = 'p2p_match_stake'
  AND wt.status = 'reserved'
  AND m.status IN ('completed', 'tied', 'forfeited', 'cancelled');

UPDATE wallets w
SET
    reserved_balance = GREATEST(0, w.reserved_balance - stale.total_amount),
    reserved_cents = GREATEST(0, w.reserved_cents - stale.total_cents),
    updated_at = NOW()
FROM (
    SELECT
        wallet_id,
        SUM(amount) AS total_amount,
        SUM(ROUND(amount * 100)) AS total_cents
    FROM stale_terminal_reservations
    GROUP BY wallet_id
) stale
WHERE w.id = stale.wallet_id;

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
    'p2p_refund:' || split_part(reference, 'p2p_stake:', 2),
    'Refunded P2P stake: terminal match reservation reconciliation',
    jsonb_build_object(
        'match_id', split_part(reference, 'p2p_stake:', 2),
        'reason', 'terminal match reservation reconciliation'
    ),
    NOW(),
    NOW()
FROM stale_terminal_reservations;

UPDATE wallet_transactions wt
SET
    status = 'refunded',
    balance_after = stale.balance,
    updated_at = NOW()
FROM stale_terminal_reservations stale
WHERE wt.id = stale.transaction_id;
