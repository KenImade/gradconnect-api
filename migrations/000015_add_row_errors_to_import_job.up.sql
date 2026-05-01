ALTER TABLE import_job
ADD COLUMN row_errors JSONB NOT NULL DEFAULT '[]'::jsonb;