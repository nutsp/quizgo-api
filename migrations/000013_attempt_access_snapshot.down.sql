ALTER TABLE exam_attempts
DROP COLUMN IF EXISTS access_source,
DROP COLUMN IF EXISTS access_entitlement_id,
DROP COLUMN IF EXISTS access_granted_at,
DROP COLUMN IF EXISTS access_expires_at;
