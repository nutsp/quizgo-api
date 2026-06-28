CREATE TABLE IF NOT EXISTS user_entitlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    entitlement_type VARCHAR(50) NOT NULL,
    ref_type VARCHAR(50) NULL,
    ref_id UUID NULL,
    source VARCHAR(50) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT NULL,
    granted_by UUID NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_entitlements_user_id ON user_entitlements(user_id);
CREATE INDEX IF NOT EXISTS idx_user_entitlements_ref ON user_entitlements(ref_type, ref_id);
CREATE INDEX IF NOT EXISTS idx_user_entitlements_active ON user_entitlements(is_active);
CREATE INDEX IF NOT EXISTS idx_user_entitlements_expires_at ON user_entitlements(expires_at);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_active_exam_set_entitlement
ON user_entitlements(user_id, entitlement_type, ref_type, ref_id)
WHERE is_active = TRUE
  AND entitlement_type = 'exam_set'
  AND ref_type = 'exam_set';
