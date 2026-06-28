DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS access_logs;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS status;
