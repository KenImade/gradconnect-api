-- Reverse: review.user_id back to NOT NULL with ON DELETE CASCADE.
-- This will fail if any reviews currently have user_id IS NULL — that's
-- intentional. Hard-deleted users mean lost data; the migration can't
-- safely reverse without restoring it.
ALTER TABLE review
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE review
    DROP CONSTRAINT IF EXISTS review_user_id_fkey,
    ADD CONSTRAINT review_user_id_fkey
        FOREIGN KEY (user_id)
            REFERENCES app_user(id)
            ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_app_user_deleted_at;

ALTER TABLE app_user
    DROP COLUMN IF EXISTS deletion_reason,
    DROP COLUMN IF EXISTS deleted_at;