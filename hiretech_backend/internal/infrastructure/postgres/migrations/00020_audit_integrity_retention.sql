-- +goose Up
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS previous_hash CHAR(64);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS entry_hash CHAR(64);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS retention_until TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '2 years');
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS legal_hold BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_audit_logs_retention ON audit_logs(retention_until) WHERE legal_hold = FALSE;
CREATE INDEX IF NOT EXISTS idx_audit_logs_integrity_chain ON audit_logs(organization_id, created_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_audit_logs_integrity_chain;
DROP INDEX IF EXISTS idx_audit_logs_retention;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS legal_hold;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS retention_until;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS entry_hash;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS previous_hash;
