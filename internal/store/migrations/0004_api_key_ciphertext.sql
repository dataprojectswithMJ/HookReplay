-- API keys: store the raw key encrypted at rest so the CLI can resolve keys
-- by name (reveal on demand). The hash remains the auth lookup form.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS ciphertext bytea;
