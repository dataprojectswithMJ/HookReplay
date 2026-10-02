-- Named API keys: optional human-readable name so users can select which key
-- to use with `hookreplay api-keys use <name>`.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS name text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_workspace_name
    ON api_keys (workspace_id, name) WHERE name IS NOT NULL;
