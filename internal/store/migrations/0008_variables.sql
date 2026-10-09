-- Distinguish secrets (encrypted, write-only) from variables (plaintext,
-- readable/editable) in the same env-scoped store.
ALTER TABLE secrets ADD COLUMN IF NOT EXISTS kind  text NOT NULL DEFAULT 'secret';
ALTER TABLE secrets ADD COLUMN IF NOT EXISTS value text;
