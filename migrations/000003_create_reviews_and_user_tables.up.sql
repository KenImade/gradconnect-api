-- ============================================================
-- Users
-- ============================================================

CREATE TYPE auth_provider_type AS ENUM ('google', 'email');

CREATE TABLE IF NOT EXISTS app_user (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash bytea,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    auth_provider auth_provider_type NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    degree_discipline VARCHAR(255),
    graduation_year INTEGER,
    target_industries TEXT[] DEFAULT '{}',
    preferred_locations TEXT[] DEFAULT '{}',
    preferences JSONB DEFAULT '{}',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE app_user ADD CONSTRAINT check_password_required
CHECK (auth_provider != 'email' OR password_hash IS NOT NULL);

-- ============================================================
-- Reviews
-- ============================================================

CREATE TYPE review_outcome_type AS ENUM ('offer', 'waitlisted', 'rejected', 'withdrew');
CREATE TYPE review_status_type AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE IF NOT EXISTS review (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employer_id UUID NOT NULL REFERENCES employer(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    programme_name VARCHAR(255) NOT NULL,
    application_year INTEGER NOT NULL,
    outcome review_outcome_type NOT NULL,
    stage_breakdown JSONB NOT NULL DEFAULT '[]',
    difficulty_rating SMALLINT NOT NULL CHECK (difficulty_rating BETWEEN 1 AND 5),
    experience_rating SMALLINT NOT NULL CHECK (experience_rating BETWEEN 1 AND 5),
    tips TEXT,
    degree_discipline VARCHAR(255),
    university VARCHAR(255),
    status review_status_type NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_review_employer_status ON review(employer_id, status);
CREATE INDEX IF NOT EXISTS idx_review_user ON review(user_id);