CREATE TABLE IF NOT EXISTS payment_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    package_id VARCHAR(80) NOT NULL,
    package_name VARCHAR(160) NOT NULL,
    duration_months INTEGER NOT NULL,
    original_price INTEGER NOT NULL,
    sale_price INTEGER NOT NULL,
    discount_percent INTEGER NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(40) NOT NULL,
    provider VARCHAR(40) NOT NULL,
    provider_reference VARCHAR(160) NOT NULL,
    qr_image_url TEXT NOT NULL,
    qr_expires_at TIMESTAMPTZ NOT NULL,
    proof_storage_key TEXT NULL,
    proof_original_name TEXT NULL,
    proof_mime_type VARCHAR(100) NULL,
    proof_size BIGINT NOT NULL DEFAULT 0,
    proof_uploaded_at TIMESTAMPTZ NULL,
    reviewed_by UUID NULL REFERENCES users(id),
    reviewed_at TIMESTAMPTZ NULL,
    rejection_reason TEXT NULL,
    entitlement_id UUID NULL REFERENCES user_entitlements(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_requests_user_id ON payment_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_requests_package_id ON payment_requests(package_id);
CREATE INDEX IF NOT EXISTS idx_payment_requests_status ON payment_requests(status);
CREATE INDEX IF NOT EXISTS idx_payment_requests_provider_reference ON payment_requests(provider_reference);
CREATE INDEX IF NOT EXISTS idx_payment_requests_qr_expires_at ON payment_requests(qr_expires_at);
