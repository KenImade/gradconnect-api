-- 00000N_create_opportunity_table.up.sql

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'opportunity_type') THEN
        CREATE TYPE opportunity_type AS ENUM ('graduate_trainee', 'internship', 'nysc', 'industrial_attachment');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS opportunity (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employer_id UUID NOT NULL REFERENCES employer(id) ON DELETE RESTRICT,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    type opportunity_type NOT NULL,
    intake_year INTEGER NOT NULL,
    description TEXT NOT NULL,
    requirements TEXT,
    location VARCHAR(255) NOT NULL,
    discipline_tags TEXT[] DEFAULT '{}',
    opens_at DATE,
    deadline DATE,
    application_url VARCHAR(512) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    source_url VARCHAR(512),
    search_vector TSVECTOR NOT NULL DEFAULT ''::tsvector,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_employer_title_year UNIQUE (employer_id, title, intake_year)
);

CREATE INDEX IF NOT EXISTS idx_opportunity_employer ON opportunity(employer_id);
CREATE INDEX IF NOT EXISTS idx_opportunity_intake_year ON opportunity(intake_year);
CREATE INDEX IF NOT EXISTS idx_opportunity_opens_at ON opportunity(opens_at);
CREATE INDEX IF NOT EXISTS idx_opportunity_deadline ON opportunity(deadline);
CREATE INDEX IF NOT EXISTS idx_opportunity_type ON opportunity(type);
CREATE INDEX IF NOT EXISTS idx_opportunity_search ON opportunity USING GIN (search_vector);
CREATE INDEX IF NOT EXISTS idx_opportunity_discipline ON opportunity USING GIN (discipline_tags);

CREATE OR REPLACE FUNCTION opportunity_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', NEW.title), 'A') ||
        setweight(to_tsvector('english', (SELECT name FROM employer WHERE id = NEW.employer_id)), 'A') ||
        setweight(to_tsvector('english', NEW.location), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.discipline_tags, ' '), '')), 'B') ||
        setweight(to_tsvector('english', (SELECT industry FROM employer WHERE id = NEW.employer_id)), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.requirements, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_opportunity_search_vector ON opportunity;
CREATE TRIGGER trg_opportunity_search_vector
    BEFORE INSERT OR UPDATE ON opportunity
    FOR EACH ROW EXECUTE FUNCTION opportunity_search_vector_update();