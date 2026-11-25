-- Retire the Aviator/Crash game without touching its history.
--
-- game_rounds, game_bets, crash_settings and the historical wallet ledger rows
-- are intentionally retained: they back admin revenue reporting and financial
-- audits. Only rounds that are still open when this migration runs are closed,
-- and their active stakes are refunded so retiring the engine cannot strand
-- player balances.
--
-- This migration is forward-only. The refunds are real ledger entries and must
-- not be reversed by a down migration.

-- Idempotency guard: only refund bets that are still active at migration time.
DO $$
DECLARE
    open_bet RECORD;
    wallet_balance DECIMAL(15,2);
BEGIN
    FOR open_bet IN
        SELECT b.id,
               b.round_id,
               b.user_id,
               b.wallet_id,
               b.stake_amount,
               w.balance
          FROM game_bets b
          JOIN game_rounds r ON r.id = b.round_id
          JOIN wallets w ON w.id = b.wallet_id
         WHERE b.status = 'active'
           AND r.state IN ('WAITING', 'BETTING', 'RUNNING')
         ORDER BY b.created_at, b.id
           FOR UPDATE OF b, w
    LOOP
        wallet_balance := open_bet.balance;

        UPDATE wallets
           SET balance = wallet_balance + open_bet.stake_amount,
               updated_at = NOW()
         WHERE id = open_bet.wallet_id;

        INSERT INTO wallet_transactions (
            wallet_id, user_id, type, status, amount,
            balance_before, balance_after, reference, description, metadata,
            created_at, updated_at
        ) VALUES (
            open_bet.wallet_id,
            open_bet.user_id,
            'crash_refund',
            'completed',
            open_bet.stake_amount,
            wallet_balance,
            wallet_balance + open_bet.stake_amount,
            'crash_refund:' || open_bet.id::text,
            'Stake refunded because the Crash game was retired',
            jsonb_build_object(
                'reason', 'game_retired',
                'crash_round_id', open_bet.round_id,
                'crash_bet_id', open_bet.id
            ),
            NOW(),
            NOW()
        );

        UPDATE game_bets
           SET status = 'refunded',
               settled_at = NOW(),
               updated_at = NOW()
         WHERE id = open_bet.id;
    END LOOP;

    UPDATE game_rounds
       SET state = 'CANCELLED',
           settled_at = COALESCE(settled_at, NOW()),
           updated_at = NOW()
     WHERE state IN ('WAITING', 'BETTING', 'RUNNING');
END $$;

-- Crash was never a p2p_games catalogue row, but deactivate one if an older
-- deployment added it. Historical p2p matches keep their foreign-key target.
UPDATE p2p_games SET is_active = FALSE WHERE slug = 'crash';
