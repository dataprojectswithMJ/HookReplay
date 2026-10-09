-- Tenant scoping for chain runs. Adds workspace_id so Logs can safely list a
-- workspace's runs (and GET /v1/runs/{id} can scope reads by tenant). Legacy
-- rows created before this migration keep a NULL workspace_id (unscoped).
ALTER TABLE chain_runs ADD COLUMN IF NOT EXISTS workspace_id text;

CREATE INDEX IF NOT EXISTS idx_chain_runs_workspace_started
    ON chain_runs (workspace_id, started_at DESC);
