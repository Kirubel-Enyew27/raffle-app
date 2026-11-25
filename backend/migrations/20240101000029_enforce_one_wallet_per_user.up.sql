-- Migration 20240101000029_enforce_one_wallet_per_user.up.sql
-- A wallet is the financial identity of a user. Concurrent first access could
-- create two wallets because wallets.user_id was only indexed, not unique.
-- Reads and stake reservations then picked an arbitrary row, so the balance a
-- player saw and the balance validation used could come from different wallets.
--
-- Consolidate every duplicate into the oldest wallet, move all references to
-- that canonical row, and add the missing uniqueness invariant. Historical
-- transactions and their amounts are preserved; nothing is deleted from the
-- ledger.

CREATE TEMP TABLE wallet_merge_map AS
SELECT
    w.id AS wallet_id,
    canonical.canonical_id
FROM wallets w
JOIN (
    SELECT
        user_id,
        (array_agg(id ORDER BY created_at ASC, id ASC))[1] AS canonical_id
    FROM wallets
    GROUP BY user_id
    HAVING COUNT(*) > 1
) canonical ON canonical.user_id = w.user_id;

-- Repoint every foreign key before the duplicate wallet rows are removed.
UPDATE wallet_transactions wt
SET wallet_id = map.canonical_id
FROM wallet_merge_map map
WHERE wt.wallet_id = map.wallet_id
  AND map.wallet_id <> map.canonical_id;

UPDATE deposits d
SET wallet_id = map.canonical_id
FROM wallet_merge_map map
WHERE d.wallet_id = map.wallet_id
  AND map.wallet_id <> map.canonical_id;

UPDATE sms_logs sl
SET credited_wallet_id = map.canonical_id
FROM wallet_merge_map map
WHERE sl.credited_wallet_id = map.wallet_id
  AND map.wallet_id <> map.canonical_id;

UPDATE game_bets gb
SET wallet_id = map.canonical_id
FROM wallet_merge_map map
WHERE gb.wallet_id = map.wallet_id
  AND map.wallet_id <> map.canonical_id;

-- Merge the balances and reservations into the canonical wallet. Summing the
-- rows consolidates funds that were split across duplicates without creating
-- new money, and each row already satisfies reserved_balance <= balance.
UPDATE wallets canonical
SET
    balance = merged.total_balance,
    reserved_balance = merged.total_reserved_balance,
    reserved_cents = merged.total_reserved_cents,
    updated_at = NOW()
FROM (
    SELECT
        map.canonical_id,
        SUM(w.balance) AS total_balance,
        SUM(w.reserved_balance) AS total_reserved_balance,
        SUM(w.reserved_cents) AS total_reserved_cents
    FROM wallet_merge_map map
    JOIN wallets w ON w.id = map.wallet_id
    GROUP BY map.canonical_id
) merged
WHERE canonical.id = merged.canonical_id;

DELETE FROM wallets duplicate
USING wallet_merge_map map
WHERE duplicate.id = map.wallet_id
  AND map.wallet_id <> map.canonical_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_wallets_user_id ON wallets(user_id);
