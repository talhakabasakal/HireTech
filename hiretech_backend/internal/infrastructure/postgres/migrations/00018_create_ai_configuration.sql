-- +goose Up
CREATE TABLE IF NOT EXISTS ai_configuration_versions (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    resource VARCHAR(32) NOT NULL CHECK (resource IN ('model','prompt','routing','rubric')),
    resource_key VARCHAR(160) NOT NULL,
    payload JSONB NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    action VARCHAR(32) NOT NULL CHECK (action IN ('created','activated','archived')),
    actor_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (organization_id, resource, resource_key, version)
);
CREATE INDEX IF NOT EXISTS idx_ai_configuration_versions_latest ON ai_configuration_versions(organization_id, resource, resource_key, version DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_configuration_versions;
