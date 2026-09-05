-- +goose Up
ALTER TABLE ai_configuration_versions
    ADD COLUMN IF NOT EXISTS lifecycle_status VARCHAR(32) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;

ALTER TABLE ai_configuration_versions
    DROP CONSTRAINT IF EXISTS ai_configuration_versions_lifecycle_status_check;
ALTER TABLE ai_configuration_versions
    ADD CONSTRAINT ai_configuration_versions_lifecycle_status_check
    CHECK (lifecycle_status IN ('draft','active','archived'));

CREATE INDEX IF NOT EXISTS idx_ai_configuration_versions_active
    ON ai_configuration_versions(organization_id, resource, resource_key, version DESC)
    WHERE lifecycle_status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_ai_configuration_versions_active;
ALTER TABLE ai_configuration_versions
    DROP CONSTRAINT IF EXISTS ai_configuration_versions_lifecycle_status_check;
ALTER TABLE ai_configuration_versions
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS lifecycle_status;
