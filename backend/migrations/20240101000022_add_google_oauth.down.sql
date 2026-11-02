DROP TABLE IF EXISTS oauth_identities;

-- Accounts that only ever signed in with Google have no password to restore.
-- Clearing the empty hashes keeps the NOT NULL constraint satisfiable while
-- leaving those accounts unable to sign in until a password is set.
UPDATE users SET password_hash = '' WHERE password_hash IS NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
