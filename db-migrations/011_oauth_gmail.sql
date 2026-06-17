CREATE TABLE IF NOT EXISTS oauth_tokens (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider      VARCHAR(32) NOT NULL DEFAULT 'google',
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expiry  TIMESTAMPTZ NOT NULL,
    gmail_address TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, provider)
);

ALTER TABLE quarantine ADD COLUMN IF NOT EXISTS gmail_message_id TEXT;
CREATE INDEX IF NOT EXISTS idx_quarantine_gmail_msg
    ON quarantine(tenant_id, gmail_message_id)
    WHERE gmail_message_id IS NOT NULL;
