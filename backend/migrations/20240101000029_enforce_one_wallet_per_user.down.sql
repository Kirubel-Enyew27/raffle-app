-- The consolidation of duplicate wallets is intentionally irreversible: the
-- rows were merged and their transactions repointed. Only the new uniqueness
-- invariant is removed so an older build can start against the schema.
DROP INDEX IF EXISTS uq_wallets_user_id;
