-- WS-8 auth: GitHub OAuth + multi-key support.

ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_provider_id text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oauth_provider
    ON users (oauth_provider, oauth_provider_id)
    WHERE oauth_provider_id IS NOT NULL;
