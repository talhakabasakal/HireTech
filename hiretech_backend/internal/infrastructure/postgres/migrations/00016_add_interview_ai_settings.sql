-- +goose Up
ALTER TABLE interviews
    ADD COLUMN IF NOT EXISTS language VARCHAR(10) NOT NULL DEFAULT 'en';
ALTER TABLE interviews
    ADD COLUMN IF NOT EXISTS question_source VARCHAR(10) NOT NULL DEFAULT 'human';
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'interviews_language_check' ) THEN
        ALTER TABLE interviews ADD CONSTRAINT interviews_language_check CHECK (language IN ('tr', 'en'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'interviews_question_source_check' ) THEN
        ALTER TABLE interviews ADD CONSTRAINT interviews_question_source_check CHECK (question_source IN ('human', 'ai'));
    END IF;
END
$$;

-- +goose Down
ALTER TABLE interviews DROP CONSTRAINT IF EXISTS interviews_question_source_check;
ALTER TABLE interviews DROP CONSTRAINT IF EXISTS interviews_language_check;
ALTER TABLE interviews DROP COLUMN IF EXISTS question_source;
ALTER TABLE interviews DROP COLUMN IF EXISTS language;
