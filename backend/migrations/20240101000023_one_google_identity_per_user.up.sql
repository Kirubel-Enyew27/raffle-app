-- A local account may have at most one Google identity attached. Without this,
-- a user could silently accumulate several linked Google accounts (only the
-- (provider, provider_user_id) pair was unique), which makes "which account
-- signs me in?" ambiguous for support and for the user.
CREATE UNIQUE INDEX IF NOT EXISTS uq_oauth_identities_provider_user
    ON oauth_identities(provider, user_id);
