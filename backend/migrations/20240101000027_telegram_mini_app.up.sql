-- Telegram Mini App accounts.
--
-- Telegram does not expose an email address, and KWIK never invents one: an
-- account provisioned from a verified Telegram identity has no email and no
-- password. The unique index still applies, and PostgreSQL allows any number of
-- NULLs, so these accounts cannot collide with each other.
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- Telegram identities live in the same provider table as Google. The provider
-- user id is the immutable Telegram numeric id; usernames are not stored as a
-- key because they can change.
CREATE INDEX IF NOT EXISTS idx_oauth_identities_provider
    ON oauth_identities(provider);
