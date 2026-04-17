CREATE TABLE IF NOT EXISTS bookmark (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    opportunity_id UUID NOT NULL REFERENCES opportunity(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, opportunity_id)
);

CREATE INDEX IF NOT EXISTS idx_bookmark_user ON bookmark(user_id);
CREATE INDEX IF NOT EXISTS idx_bookmark_opportunity ON bookmark(opportunity_id);