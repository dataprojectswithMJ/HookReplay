-- HookReplay vertical slice schema (subset of §4).
-- IDs use prefixed text primary keys (hrk_, evt_, chn_, tmpl_, sec_).

CREATE TABLE IF NOT EXISTS workspaces (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    tier       text NOT NULL DEFAULT 'free',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id             text PRIMARY KEY,
    email          text UNIQUE NOT NULL,
    oauth_provider text NOT NULL DEFAULT 'dev',
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         text NOT NULL DEFAULT 'owner',
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS api_keys (
    id           text PRIMARY KEY,
    user_id      text NOT NULL,
    workspace_id text NOT NULL,
    prefix       text NOT NULL,
    hashed_key   text NOT NULL UNIQUE,
    scopes       jsonb NOT NULL DEFAULT '[]',
    last_used_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS secrets (
    id           text PRIMARY KEY,
    workspace_id text NOT NULL,
    env          text NOT NULL,
    name         text NOT NULL,
    ciphertext   bytea NOT NULL,
    key_version  int  NOT NULL DEFAULT 1,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, env, name)
);

CREATE TABLE IF NOT EXISTS executions (
    id                  text PRIMARY KEY,
    workspace_id        text NOT NULL,
    chain_id            text,
    step_name           text,
    target_kind         text NOT NULL,
    target              text NOT NULL,
    provider            text,
    template_id         text,
    request_body        bytea NOT NULL,
    request_headers     jsonb NOT NULL DEFAULT '{}',
    signature_header    jsonb NOT NULL DEFAULT '{}',
    deliver             jsonb NOT NULL DEFAULT '{}',
    error_code          text,
    response_status     int  NOT NULL DEFAULT 0,
    response_body       bytea,
    latency_ms          bigint,
    attempt             int  NOT NULL DEFAULT 1,
    parent_execution_id text,
    idempotency_key     text,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_executions_workspace_created
    ON executions (workspace_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_executions_idempotency
    ON executions (workspace_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS tunnels (
    id           text PRIMARY KEY,
    workspace_id text NOT NULL,
    subdomain    text UNIQUE NOT NULL,
    connected_at timestamptz,
    last_seen_at timestamptz,
    cli_version  text
);
