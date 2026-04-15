CREATE TABLE IF NOT EXISTS employer (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    industry VARCHAR(100) NOT NULL,
    size VARCHAR(50),
    hq_location VARCHAR(255),
    offices JSONB DEFAULT '[]',
    logo_url VARCHAR(512),
    overview TEXT,
    culture TEXT,
    website VARCHAR(512),
    social_links JSONB DEFAULT '{}',
    is_verified BOOLEAN NOT NULL DEFAULT false,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_employer_slug
ON employer (slug);

CREATE INDEX IF NOT EXISTS idx_employer_industry
ON employer (industry);

CREATE INDEX IF NOT EXISTS idx_employer_search
ON employer 
USING GIN (to_tsvector('english', name || ' ' || COALESCE(overview, '') || ' ' || industry || ' ' || COALESCE(hq_location, '')));