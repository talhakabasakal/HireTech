-- +goose Up
CREATE TABLE IF NOT EXISTS evaluation_jobs (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    requested_by UUID NOT NULL REFERENCES users(id),
    status VARCHAR(32) NOT NULL CHECK (status IN ('queued','running','completed','failed')),
    rubric_version VARCHAR(100) NOT NULL,
    evaluator_version VARCHAR(100) NOT NULL,
    failure_code VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    UNIQUE(organization_id, interview_id)
);
CREATE INDEX IF NOT EXISTS idx_evaluation_jobs_pending ON evaluation_jobs(status, created_at);

CREATE TABLE IF NOT EXISTS evaluation_reports (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES evaluation_jobs(id),
    requested_by UUID NOT NULL REFERENCES users(id),
    status VARCHAR(40) NOT NULL CHECK (status IN ('draft','review_required','ready_for_human_decision','published','rejected')),
    rubric_id VARCHAR(160) NOT NULL,
    rubric_version VARCHAR(100) NOT NULL,
    evaluator_configuration_version VARCHAR(100) NOT NULL,
    overall_score NUMERIC(5,2) CHECK (overall_score IS NULL OR (overall_score >= 0 AND overall_score <= 100)),
    overall_confidence NUMERIC(4,3) NOT NULL CHECK (overall_confidence >= 0 AND overall_confidence <= 1),
    strengths JSONB NOT NULL DEFAULT '[]',
    gaps JSONB NOT NULL DEFAULT '[]',
    evidence_references JSONB NOT NULL DEFAULT '[]',
    limitations JSONB NOT NULL DEFAULT '[]',
    generated_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(organization_id, interview_id)
);
CREATE INDEX IF NOT EXISTS idx_evaluation_reports_org_status ON evaluation_reports(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS evaluation_criterion_scores (
    id UUID PRIMARY KEY,
    report_id UUID NOT NULL REFERENCES evaluation_reports(id) ON DELETE CASCADE,
    criterion_id VARCHAR(100) NOT NULL,
    applicable BOOLEAN NOT NULL,
    score NUMERIC(4,2) CHECK (score IS NULL OR (score >= 0 AND score <= 4)),
    maximum_score NUMERIC(4,2) NOT NULL CHECK (maximum_score = 4),
    weight NUMERIC(6,5) NOT NULL CHECK (weight >= 0 AND weight <= 1),
    confidence NUMERIC(4,3) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    evidence_references JSONB NOT NULL DEFAULT '[]',
    rationale TEXT NOT NULL,
    limitations JSONB NOT NULL DEFAULT '[]',
    UNIQUE(report_id, criterion_id)
);

CREATE TABLE IF NOT EXISTS evaluation_human_reviews (
    id UUID PRIMARY KEY,
    report_id UUID NOT NULL UNIQUE REFERENCES evaluation_reports(id) ON DELETE CASCADE,
    required BOOLEAN NOT NULL,
    urgency VARCHAR(20) NOT NULL CHECK (urgency IN ('none','normal','high','immediate')),
    reason_codes JSONB NOT NULL DEFAULT '[]',
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending','approved','rejected')),
    reviewer_user_id UUID REFERENCES users(id),
    notes TEXT NOT NULL DEFAULT '',
    completed_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE IF EXISTS evaluation_human_reviews;
DROP TABLE IF EXISTS evaluation_criterion_scores;
DROP TABLE IF EXISTS evaluation_reports;
DROP TABLE IF EXISTS evaluation_jobs;
