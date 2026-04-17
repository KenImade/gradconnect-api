DO $$ BEGIN
    CREATE TYPE application_status_type AS ENUM ('interested', 'applied', 'assessment', 'interview', 'offer', 'rejected');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS application_track (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    opportunity_id UUID NOT NULL REFERENCES opportunity(id) ON DELETE CASCADE,
    status application_status_type NOT NULL DEFAULT 'interested',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, opportunity_id)
);

CREATE INDEX IF NOT EXISTS idx_apptrack_user ON application_track(user_id);
CREATE INDEX IF NOT EXISTS idx_apptrack_status ON application_track (user_id, status);