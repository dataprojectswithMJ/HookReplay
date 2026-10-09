-- Link individual webhook executions to the chain run that produced them, so
-- Logs can show step → execution lineage (run scoping, per-attempt trail).
-- Nullable: single fires and pre-chain-run executions stay NULL.

ALTER TABLE executions ADD COLUMN IF NOT EXISTS chain_run_id text REFERENCES chain_runs(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_executions_chain_run ON executions (chain_run_id, created_at);
