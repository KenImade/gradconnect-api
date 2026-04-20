CREATE TABLE IF NOT EXISTS assessment_profile (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employer_id UUID NOT NULL REFERENCES employer(id) ON DELETE RESTRICT,
    programme_type VARCHAR(100) NOT NULL,
    stages JSONB NOT NULL DEFAULT '[]',
    aptitude_test_provider VARCHAR(100),
    interview_format VARCHAR(100),
    timeline_weeks INTEGER,
    prep_guide TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assessment_employer ON assessment_profile(employer_id);