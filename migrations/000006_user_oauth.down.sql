DROP INDEX IF EXISTS idx_user_oauth_accounts_user_id;
DROP TABLE IF EXISTS user_oauth_accounts;

-- Restore NOT NULL only if no OAuth-only users remain.
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
