-- +goose Up
CREATE TABLE IF NOT EXISTS interview_question_drafts (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    requested_by UUID NOT NULL REFERENCES users(id),
    reviewed_by UUID REFERENCES users(id),
    type VARCHAR(40) NOT NULL CHECK (type IN ('technical_discussion', 'coding', 'system_design', 'debugging')),
    prompt TEXT NOT NULL CHECK (char_length(prompt) BETWEEN 1 AND 8000),
    competency_ids TEXT[] NOT NULL DEFAULT '{}',
    difficulty INTEGER NOT NULL CHECK (difficulty BETWEEN 1 AND 5),
    time_limit_seconds INTEGER CHECK (time_limit_seconds IS NULL OR time_limit_seconds BETWEEN 30 AND 7200),
    language VARCHAR(10) NOT NULL CHECK (language IN ('tr', 'en')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    model_id VARCHAR(200) NOT NULL,
    model_version VARCHAR(100) NOT NULL,
    review_notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    reviewed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_question_drafts_scope ON interview_question_drafts(organization_id,interview_id,status,created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS interview_question_drafts;
