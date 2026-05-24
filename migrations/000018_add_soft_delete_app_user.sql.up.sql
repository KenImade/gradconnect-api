-- Add soft-delete columns to app_user.
ALTER TABLE app_user
    ADD COLUMN deleted_at TIMESTAMPTZ,
    ADD COLUMN deletion_reason TEXT;

-- Partial index: only includes soft-deleted rows. Small (most users
-- aren't deleted), fast for the cleanup cron to find users due for
-- permanent removal.
CREATE INDEX idx_app_user_deleted_at
    ON app_user(deleted_at)
    WHERE deleted_at IS NOT NULL;

-- Make sure reviews survive permanent user deletion. The review's
-- value to other readers doesn't depend on knowing who wrote it; the
-- display layer already shows reviews anonymously.
ALTER TABLE review
    DROP CONSTRAINT IF EXISTS review_user_id_fkey,
    ADD CONSTRAINT review_user_id_fkey
        FOREIGN KEY (user_id)
            REFERENCES app_user(id)
            ON DELETE SET NULL;

-- review.user_id needs to be nullable for SET NULL to work.
ALTER TABLE review
    ALTER COLUMN user_id DROP NOT NULL;