package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrIdempotencyConflict signals that an execution with the same
// (workspace_id, idempotency_key) already exists.
var ErrIdempotencyConflict = errors.New("store: idempotency key already used")

// Execution is a persisted dispatch result.
type Execution struct {
	ID                string
	WorkspaceID       string
	ChainID           string
	StepName          string
	TargetKind        string
	Target            string
	Provider          string
	TemplateID        string
	RequestBody       []byte
	RequestHeaders    map[string]string
	SignatureHeader   map[string]string
	Deliver           map[string]string
	ErrorCode         string
	ResponseStatus    int
	ResponseBody      []byte
	LatencyMS         int64
	Attempt           int
	ParentExecutionID string
	IdempotencyKey    string
	CreatedAt         time.Time
}

func jsonStr(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func mapFromJSON(b []byte) map[string]string {
	if len(b) == 0 {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]string{}
	}
	return m
}

// CreateExecution inserts an execution. A unique-violation on the idempotency
// index returns ErrIdempotencyConflict.
func (s *Store) CreateExecution(ctx context.Context, e *Execution) error {
	if e.RequestBody == nil {
		e.RequestBody = []byte{}
	}
	if e.ResponseBody == nil {
		e.ResponseBody = []byte{}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO executions (
			id, workspace_id, chain_id, step_name, target_kind, target,
			provider, template_id, request_body, request_headers,
			signature_header, deliver, error_code, response_status, response_body,
			latency_ms, attempt, parent_execution_id, idempotency_key
		) VALUES (
			$1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,
			NULLIF($7,''),NULLIF($8,''),$9,$10::jsonb,
			$11::jsonb,$12::jsonb,NULLIF($13,''),$14,$15,
			NULLIF($16,0),$17,NULLIF($18,''),NULLIF($19,'')
		)`,
		e.ID, e.WorkspaceID, e.ChainID, e.StepName, e.TargetKind, e.Target,
		e.Provider, e.TemplateID, e.RequestBody, jsonStr(e.RequestHeaders),
		jsonStr(e.SignatureHeader), jsonStr(e.Deliver), e.ErrorCode, e.ResponseStatus, e.ResponseBody,
		e.LatencyMS, e.Attempt, e.ParentExecutionID, e.IdempotencyKey,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrIdempotencyConflict
		}
		return fmt.Errorf("store: create execution: %w", err)
	}
	return nil
}

const executionColumns = `id, workspace_id, chain_id, step_name, target_kind, target,
	provider, template_id, request_body, request_headers, signature_header,
	deliver, error_code, response_status, response_body, latency_ms, attempt,
	parent_execution_id, idempotency_key, created_at`

func scanExecution(row pgx.Row) (*Execution, error) {
	var e Execution
	var chainID, stepName, provider, templateID, parentID, idemKey *string
	var errCode *string
	var latency *int64
	var respBody []byte
	var reqHdr, sigHdr, deliver []byte
	err := row.Scan(
		&e.ID, &e.WorkspaceID, &chainID, &stepName, &e.TargetKind, &e.Target,
		&provider, &templateID, &e.RequestBody, &reqHdr, &sigHdr,
		&deliver, &errCode, &e.ResponseStatus, &respBody, &latency, &e.Attempt,
		&parentID, &idemKey, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if chainID != nil {
		e.ChainID = *chainID
	}
	if stepName != nil {
		e.StepName = *stepName
	}
	if provider != nil {
		e.Provider = *provider
	}
	if templateID != nil {
		e.TemplateID = *templateID
	}
	if parentID != nil {
		e.ParentExecutionID = *parentID
	}
	if idemKey != nil {
		e.IdempotencyKey = *idemKey
	}
	if errCode != nil {
		e.ErrorCode = *errCode
	}
	if latency != nil {
		e.LatencyMS = *latency
	}
	e.ResponseBody = respBody
	e.RequestHeaders = mapFromJSON(reqHdr)
	e.SignatureHeader = mapFromJSON(sigHdr)
	e.Deliver = mapFromJSON(deliver)
	return &e, nil
}

// GetExecution returns one execution scoped to a workspace.
func (s *Store) GetExecution(ctx context.Context, workspaceID, id string) (*Execution, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+executionColumns+` FROM executions WHERE id = $1 AND workspace_id = $2`,
		id, workspaceID)
	e, err := scanExecution(row)
	if err != nil {
		return nil, fmt.Errorf("store: get execution: %w", err)
	}
	return e, nil
}

// GetExecutionByIdempotency returns the original execution for a key.
func (s *Store) GetExecutionByIdempotency(ctx context.Context, workspaceID, key string) (*Execution, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+executionColumns+` FROM executions
		 WHERE workspace_id = $1 AND idempotency_key = $2
		 ORDER BY created_at ASC LIMIT 1`, workspaceID, key)
	e, err := scanExecution(row)
	if err != nil {
		return nil, fmt.Errorf("store: get execution by idempotency: %w", err)
	}
	return e, nil
}

// ListExecutions returns recent executions for a workspace, newest first.
func (s *Store) ListExecutions(ctx context.Context, workspaceID string, limit int) ([]*Execution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+executionColumns+` FROM executions
		 WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list executions: %w", err)
	}
	defer rows.Close()
	var out []*Execution = []*Execution{}
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan execution: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
