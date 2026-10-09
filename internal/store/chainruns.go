package store

import (
	"context"
	"fmt"
	"time"

	"github.com/hookreplay/hookreplay/internal/id"
	"github.com/jackc/pgx/v5"
)

// ChainRunStep is one step's persisted outcome within a chain run.
type ChainRunStep struct {
	ID             string `json:"id"`
	RunID          string `json:"run_id"`
	StepName       string `json:"step_name"`
	StepType       string `json:"step_type"`
	RequestSummary string `json:"request_summary"`
	ResponseStatus int    `json:"response_status"`
	ResponseBody   string `json:"response_body"`
	DurationMS     int64  `json:"duration_ms"`
	Passed         bool   `json:"passed"`
	Error          string `json:"error"`
	Position       int    `json:"position"`
}

// ChainRun is a persisted chain execution with its steps.
type ChainRun struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	ChainID     string         `json:"chain_id,omitempty"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	TargetURL  string         `json:"target_url"`
	StartedAt  time.Time      `json:"started_at"`
	DurationMS int64          `json:"duration_ms"`
	Error      string         `json:"error"`
	Steps      []ChainRunStep `json:"steps,omitempty"`
}

// CreateChainRun inserts a run and its steps in a single transaction.
// A nil ChainID stores NULL (ad-hoc YAML runs).
func (s *Store) CreateChainRun(ctx context.Context, run *ChainRun) (*ChainRun, error) {
	if run.Status == "" {
		run.Status = "passed"
	}
	run.ID = id.New("run_")

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: begin chain run: %w", err)
	}
	defer tx.Rollback(ctx)

	var chainID *string
	if run.ChainID != "" {
		chainID = &run.ChainID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO chain_runs (id, workspace_id, chain_id, name, status, target_url, duration_ms, error)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,0),NULLIF($8,''))
		RETURNING started_at`,
		run.ID, run.WorkspaceID, chainID, run.Name, run.Status, run.TargetURL, run.DurationMS, run.Error,
	).Scan(&run.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("store: create chain run: %w", err)
	}

	if err := insertChainRunSteps(ctx, tx, run.ID, run.Steps); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("store: commit chain run: %w", err)
	}
	return run, nil
}

// FinalizeChainRun updates a previously created run (e.g. one inserted as
// "running" before execution started) with its outcome and steps.
func (s *Store) FinalizeChainRun(ctx context.Context, run *ChainRun) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin finalize chain run: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE chain_runs SET status = $2, duration_ms = NULLIF($3,0), error = NULLIF($4,'')
		WHERE id = $1`,
		run.ID, run.Status, run.DurationMS, run.Error)
	if err != nil {
		return fmt.Errorf("store: finalize chain run: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: finalize chain run: run %s not found", run.ID)
	}

	if err := insertChainRunSteps(ctx, tx, run.ID, run.Steps); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit finalize chain run: %w", err)
	}
	return nil
}

// insertChainRunSteps inserts a run's steps in position order.
func insertChainRunSteps(ctx context.Context, tx pgx.Tx, runID string, steps []ChainRunStep) error {
	for i := range steps {
		st := &steps[i]
		st.ID = id.New("crs_")
		st.RunID = runID
		st.Position = i
		_, err := tx.Exec(ctx, `
			INSERT INTO chain_run_steps (
				id, run_id, step_name, step_type, request_summary,
				response_status, response_body, duration_ms, passed, error, position
			) VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7,NULLIF($8,0),$9,NULLIF($10,''),$11)`,
			st.ID, st.RunID, st.StepName, st.StepType, st.RequestSummary,
			st.ResponseStatus, st.ResponseBody, st.DurationMS, st.Passed, st.Error, st.Position,
		)
		if err != nil {
			return fmt.Errorf("store: create chain run step: %w", err)
		}
	}
	return nil
}

const chainRunStepColumns = `id, run_id, step_name, step_type, request_summary,
	response_status, response_body, duration_ms, passed, error, position`

func scanChainRunStep(row interface{ Scan(...any) error }) (*ChainRunStep, error) {
	var st ChainRunStep
	var stepType, reqSummary, respBody, errStr *string
	var duration *int64
	err := row.Scan(&st.ID, &st.RunID, &st.StepName, &stepType, &reqSummary,
		&st.ResponseStatus, &respBody, &duration, &st.Passed, &errStr, &st.Position)
	if err != nil {
		return nil, err
	}
	if stepType != nil {
		st.StepType = *stepType
	}
	if reqSummary != nil {
		st.RequestSummary = *reqSummary
	}
	if respBody != nil {
		st.ResponseBody = *respBody
	}
	if errStr != nil {
		st.Error = *errStr
	}
	if duration != nil {
		st.DurationMS = *duration
	}
	return &st, nil
}

func (s *Store) listChainRunSteps(ctx context.Context, runID string) ([]ChainRunStep, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+chainRunStepColumns+` FROM chain_run_steps
		 WHERE run_id = $1 ORDER BY position ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("store: list chain run steps: %w", err)
	}
	defer rows.Close()
	out := []ChainRunStep{}
	for rows.Next() {
		st, err := scanChainRunStep(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan chain run step: %w", err)
		}
		out = append(out, *st)
	}
	return out, rows.Err()
}

// ListChainRuns returns recent runs for a chain, newest first.
func (s *Store) ListChainRuns(ctx context.Context, chainID string, limit int) ([]*ChainRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, chain_id, name, status, target_url, started_at, duration_ms, error
		FROM chain_runs WHERE chain_id = $1 ORDER BY started_at DESC LIMIT $2`, chainID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list chain runs: %w", err)
	}
	defer rows.Close()
	out := []*ChainRun{}
	for rows.Next() {
		var r ChainRun
		var chainIDp, target, errStr *string
		var duration *int64
		if err := rows.Scan(&r.ID, &chainIDp, &r.Name, &r.Status, &target, &r.StartedAt, &duration, &errStr); err != nil {
			return nil, fmt.Errorf("store: scan chain run: %w", err)
		}
		if chainIDp != nil {
			r.ChainID = *chainIDp
		}
		if target != nil {
			r.TargetURL = *target
		}
		if duration != nil {
			r.DurationMS = *duration
		}
		if errStr != nil {
			r.Error = *errStr
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

// GetChainRun returns one run (scoped to a workspace) with its steps ordered
// by position.
func (s *Store) GetChainRun(ctx context.Context, wsID, id string) (*ChainRun, error) {
	var r ChainRun
	var chainID, target, errStr *string
	var duration *int64
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, chain_id, name, status, target_url, started_at, duration_ms, error
		FROM chain_runs WHERE id = $1 AND workspace_id = $2`, id, wsID).
		Scan(&r.ID, &r.WorkspaceID, &chainID, &r.Name, &r.Status, &target, &r.StartedAt, &duration, &errStr)
	if err != nil {
		return nil, fmt.Errorf("store: get chain run: %w", err)
	}
	if chainID != nil {
		r.ChainID = *chainID
	}
	if target != nil {
		r.TargetURL = *target
	}
	if duration != nil {
		r.DurationMS = *duration
	}
	if errStr != nil {
		r.Error = *errStr
	}
	steps, err := s.listChainRunSteps(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	r.Steps = steps
	return &r, nil
}

// ListRecentRuns returns a workspace's recent chain runs, newest first.
func (s *Store) ListRecentRuns(ctx context.Context, wsID string, limit int) ([]*ChainRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, workspace_id, chain_id, name, status, target_url, started_at, duration_ms, error
		FROM chain_runs WHERE workspace_id = $1 ORDER BY started_at DESC LIMIT $2`, wsID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list recent chain runs: %w", err)
	}
	defer rows.Close()
	out := []*ChainRun{}
	for rows.Next() {
		var r ChainRun
		var chainIDp, target, errStr *string
		var duration *int64
		if err := rows.Scan(&r.ID, &r.WorkspaceID, &chainIDp, &r.Name, &r.Status, &target, &r.StartedAt, &duration, &errStr); err != nil {
			return nil, fmt.Errorf("store: scan chain run: %w", err)
		}
		if chainIDp != nil {
			r.ChainID = *chainIDp
		}
		if target != nil {
			r.TargetURL = *target
		}
		if duration != nil {
			r.DurationMS = *duration
		}
		if errStr != nil {
			r.Error = *errStr
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}
