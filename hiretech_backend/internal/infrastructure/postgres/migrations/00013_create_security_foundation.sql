-- +goose Up
CREATE TABLE IF NOT EXISTS security_otp_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL, code_hash VARCHAR(128) NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0, max_attempts INTEGER NOT NULL, used_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_security_otp_email_created ON security_otp_challenges(email, created_at DESC);
CREATE TABLE IF NOT EXISTS security_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL DEFAULT '', fingerprint_hash VARCHAR(128), state VARCHAR(32) NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_security_devices_user ON security_devices(user_id);
CREATE TABLE IF NOT EXISTS security_sessions (
    id UUID PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID, device_id UUID NOT NULL REFERENCES security_devices(id) ON DELETE CASCADE,
    family_id UUID NOT NULL, refresh_token_hash VARCHAR(128) NOT NULL, refresh_token_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL, last_used_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ, authentication_time TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_security_sessions_user ON security_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_security_sessions_family ON security_sessions(family_id);
CREATE TABLE IF NOT EXISTS security_refresh_tokens (
    session_id UUID NOT NULL REFERENCES security_sessions(id) ON DELETE CASCADE, token_hash VARCHAR(128) PRIMARY KEY,
    used_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_security_refresh_session ON security_refresh_tokens(session_id);
-- +goose Down
DROP TABLE IF EXISTS security_refresh_tokens;
DROP TABLE IF EXISTS security_sessions;
DROP TABLE IF EXISTS security_devices;
DROP TABLE IF EXISTS security_otp_challenges;
