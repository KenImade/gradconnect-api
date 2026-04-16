CREATE TABLE IF NOT EXISTS session (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    ip_address VARCHAR(45),
    user_agent VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '7 days')
);

CREATE INDEX idx_session_user ON session (user_id);
CREATE INDEX idx_session_expires ON session (expires_at);

CREATE TABLE IF NOT EXISTS email_verification_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '24 hours'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Lookups happen by the hash provided in the email link
CREATE INDEX idx_evt_hash ON email_verification_token (token_hash);
-- Used for the lifecycle rule: one token per user
CREATE INDEX idx_evt_user ON email_verification_token (user_id);

CREATE TABLE IF NOT EXISTS password_reset_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '1 hour'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Lookups happen by the hash provided in the reset link
CREATE INDEX idx_prt_hash ON password_reset_token (token_hash);
-- Used for the lifecycle rule: one token per user
CREATE INDEX idx_prt_user ON password_reset_token (user_id);