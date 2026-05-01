CREATE TABLE IF NOT EXISTS cron_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name VARCHAR(100) NOT NULL,
    run_date DATE NOT NULL,
    enqueued_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,

    UNIQUE (job_name, run_date)
);

CREATE INDEX IF NOT EXISTS idx_cron_run_job_date ON cron_run (job_name, run_date);