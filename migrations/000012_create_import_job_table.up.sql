DO $$ BEGIN
    CREATE TYPE import_status_type AS ENUM ('pending', 'processing', 'completed', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS import_job (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE SET NULL,
    import_type VARCHAR(50) NOT NULL,
    file_path TEXT NOT NULL,
    status import_status_type NOT NULL DEFAULT 'pending',
    rows_total INTEGER,
    rows_imported INTEGER,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_import_job_user ON import_job(user_id);
CREATE INDEX IF NOT EXISTS idx_import_job_status ON import_job(status);