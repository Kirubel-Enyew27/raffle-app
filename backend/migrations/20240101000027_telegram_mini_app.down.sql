UPDATE users
SET email = 'telegram-' || id::text || '@local.invalid'
WHERE email IS NULL;

ALTER TABLE users ALTER COLUMN email SET NOT NULL;

DROP INDEX IF EXISTS idx_oauth_identities_provider;
