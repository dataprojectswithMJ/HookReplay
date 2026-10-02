-- User sessions: login issues a session token (identity), distinct from API
-- keys (operations). The raw token is usr_… and only its hash is stored.
CREATE TABLE IF NOT EXISTS sessions (
    id           text PRIMARY KEY,
    user_id      text NOT NULL,
    token_hash   text NOT NULL UNIQUE,
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
