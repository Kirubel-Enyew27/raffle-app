-- Google sign-in support.
--
-- Accounts created through an OAuth provider have no local password, so
-- password_hash must be nullable instead of being filled with a fake hash that
-- could never be compared successfully. Login already treats an empty hash as
-- "no password set on this account".
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- One row per external identity linked to a local account. Google's stable
-- `sub` claim is the identity key: email addresses can be reassigned, so they
-- are recorded for support purposes only and never used for authentication.
CREATE TABLE IF NOT EXISTS oauth_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    provider_email VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_oauth_identities_provider_user UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_identities_user_id ON oauth_identities(user_id);
