-- WS-5/WS-6/WS-8: chains storage, domains verification, usage support.

CREATE TABLE IF NOT EXISTS chains (
    id           text PRIMARY KEY,
    workspace_id text NOT NULL,
    source       text NOT NULL DEFAULT 'dashboard',
    name         text NOT NULL,
    yaml_text    text NOT NULL,
    version      int  NOT NULL DEFAULT 1,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chains_workspace ON chains (workspace_id, name);

CREATE TABLE IF NOT EXISTS domains (
    id                 text PRIMARY KEY,
    workspace_id       text NOT NULL,
    domain             text NOT NULL,
    verification_token text NOT NULL,
    verified_at        timestamptz,
    method             text NOT NULL DEFAULT 'served_token',
    created_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, domain)
);
