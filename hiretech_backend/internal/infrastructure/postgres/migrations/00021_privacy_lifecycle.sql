-- +goose Up
CREATE TABLE IF NOT EXISTS privacy_requests (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    organization_id UUID REFERENCES organizations(id),
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('deletion','export')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending','blocked','running','complete','failed')),
    reason TEXT NOT NULL DEFAULT '',
    manifest JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_privacy_requests_user ON privacy_requests(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS privacy_legal_holds (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    reason TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL,
    released_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_privacy_legal_holds_active ON privacy_legal_holds(organization_id, user_id) WHERE released_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS privacy_legal_holds;
DROP TABLE IF EXISTS privacy_requests;
