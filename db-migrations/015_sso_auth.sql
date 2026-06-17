-- Make password_hash nullable (SSO users have no password)
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- SSO identity columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider TEXT NOT NULL DEFAULT 'password';
ALTER TABLE users ADD COLUMN IF NOT EXISTS provider_user_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso ON users(auth_provider, provider_user_id)
  WHERE provider_user_id IS NOT NULL;

-- Trial tracking on tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ;
