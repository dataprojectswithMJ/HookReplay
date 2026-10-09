-- Chain run persistence: one row per server-side chain execution, with
-- per-step outcomes. IDs follow the prefixed-text convention (run_, crs_).

CREATE TABLE IF NOT EXISTS chain_runs (
    id          text PRIMARY KEY,
    chain_id    text REFERENCES chains(id) ON DELETE SET NULL,
    name        text NOT NULL,
    status      text NOT NULL DEFAULT 'passed',
    target_url  text,
    started_at  timestamptz NOT NULL DEFAULT now(),
    duration_ms bigint,
    error       text
);

CREATE INDEX IF NOT EXISTS idx_chain_runs_chain_started
    ON chain_runs (chain_id, started_at DESC);

CREATE TABLE IF NOT EXISTS chain_run_steps (
    id              text PRIMARY KEY,
    run_id          text NOT NULL REFERENCES chain_runs(id) ON DELETE CASCADE,
    step_name       text NOT NULL,
    step_type       text,
    request_summary text,
    response_status int,
    response_body   text,
    duration_ms     bigint,
    passed          boolean NOT NULL DEFAULT false,
    error           text,
    position        int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_chain_run_steps_run
    ON chain_run_steps (run_id, position);
