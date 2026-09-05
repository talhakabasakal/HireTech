-- +goose Up
CREATE TABLE IF NOT EXISTS interviews (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id),
    candidate_user_id UUID REFERENCES users(id),
    candidate_email VARCHAR(255) NOT NULL,
    candidate_display_name VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    position_title VARCHAR(255) NOT NULL,
    seniority VARCHAR(100) NOT NULL DEFAULT '',
    technology_tags TEXT[] NOT NULL DEFAULT '{}',
    mode VARCHAR(32) NOT NULL CHECK (mode IN ('ai_disabled','guided_ai','ai_collaboration')),
    rubric_version VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL CHECK (status IN ('draft','ready','invited','in_progress','completed','cancelled','expired')),
    starts_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_interviews_org_status ON interviews(organization_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_interviews_candidate_user ON interviews(candidate_user_id);

CREATE TABLE IF NOT EXISTS interview_questions (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    type VARCHAR(40) NOT NULL CHECK (type IN ('technical_discussion','coding','system_design','debugging')),
    prompt TEXT NOT NULL,
    competency_ids TEXT[] NOT NULL DEFAULT '{}',
    difficulty INTEGER NOT NULL CHECK (difficulty BETWEEN 1 AND 5),
    time_limit_seconds INTEGER CHECK (time_limit_seconds > 0),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(interview_id,sequence)
);
CREATE INDEX IF NOT EXISTS idx_interview_questions_org_interview ON interview_questions(organization_id,interview_id,sequence);

CREATE TABLE IF NOT EXISTS interview_invitations (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    token_hash VARCHAR(128) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_interview_invitations_interview ON interview_invitations(organization_id,interview_id);

CREATE TABLE IF NOT EXISTS interview_consents (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    policy_version VARCHAR(100) NOT NULL,
    locale VARCHAR(20) NOT NULL,
    purpose VARCHAR(255) NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL,
    UNIQUE(interview_id,user_id,policy_version,purpose)
);

CREATE TABLE IF NOT EXISTS interview_sessions (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES security_devices(id),
    status VARCHAR(32) NOT NULL CHECK (status IN ('ready','active','completed','expired')),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_interview_sessions_one_active ON interview_sessions(interview_id,user_id) WHERE status='active';

CREATE TABLE IF NOT EXISTS interview_answers (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES interview_questions(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL CHECK (status IN ('submitted','superseded')),
    answer_text TEXT,
    code_language VARCHAR(80),
    code_content TEXT,
    content_hash VARCHAR(128) NOT NULL,
    idempotency_key UUID NOT NULL,
    supersedes_id UUID REFERENCES interview_answers(id),
    submitted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(organization_id,user_id,idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_interview_answers_scope ON interview_answers(organization_id,interview_id,question_id,created_at);

CREATE TABLE IF NOT EXISTS audit_outbox (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_audit_outbox_pending ON audit_outbox(occurred_at) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS audit_outbox;
DROP TABLE IF EXISTS interview_answers;
DROP TABLE IF EXISTS interview_sessions;
DROP TABLE IF EXISTS interview_consents;
DROP TABLE IF EXISTS interview_invitations;
DROP TABLE IF EXISTS interview_questions;
DROP TABLE IF EXISTS interviews;
