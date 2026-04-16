CREATE TABLE IF NOT EXISTS user_permission (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    permission VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    granted_by UUID REFERENCES app_user(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, permission, resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_perm_user ON user_permission(user_id);
CREATE UNIQUE INDEX idx_unique_global_perm 
ON user_permission (user_id, permission) 
WHERE resource_id IS NULL;
