-- Create the custom enum type for task status
CREATE TYPE task_status_type AS ENUM ('pending', 'processing', 'completed', 'failed', 'dead');

CREATE TABLE IF NOT EXISTS task_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status task_status_type NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    last_error TEXT,
    run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for efficient polling by the worker
-- We use a partial index because the worker only cares about 'pending' tasks
CREATE INDEX idx_task_poll ON task_queue (run_at ASC) 
WHERE status = 'pending';

-- Index for monitoring and admin dashboards
CREATE INDEX idx_task_type_status ON task_queue (job_type, status);