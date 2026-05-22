ALTER TABLE app_user
    ADD COLUMN email_status TEXT NOT NULL DEFAULT 'active'
        CHECK (email_status IN ('active', 'bounced', 'complained')),
    ADD COLUMN email_status_updated_at TIMESTAMPTZ;

-- Partial index: only includes non-active rows. Small (most users are
-- active) but lets admin queries list/count suppressed addresses fast.
CREATE INDEX idx_app_user_email_status
    ON app_user(email_status)
    WHERE email_status != 'active';