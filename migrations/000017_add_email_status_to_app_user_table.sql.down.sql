DROP INDEX IF EXISTS idx_app_user_email_status;

ALTER TABLE app_user
    DROP COLUMN IF EXISTS email_status_updated_at,
    DROP COLUMN IF EXISTS email_status;