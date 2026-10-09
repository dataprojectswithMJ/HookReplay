package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/hookreplay/hookreplay/internal/chains"
	"github.com/hookreplay/hookreplay/internal/store"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
)

// serverDispatcher adapts dispatchOnce to the chains.WebhookDispatcher
// interface, so server-side chain execution reuses the exact dispatch path.
type serverDispatcher struct {
	s  *server
	pr *store.Principal
}

func (d *serverDispatcher) Dispatch(ctx context.Context, req apiclient.DispatchRequest) (*apiclient.Execution, error) {
	return d.s.dispatchOnce(ctx, d.pr, req)
}

// handleExecuteChain runs a chain server-side and returns per-step results.
func (s *server) handleExecuteChain(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	if pr == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing principal", reqID)
		return
	}

	var body struct {
		YAMLText    string `json:"yaml_text"`
		Target      string `json:"target"`
		Environment string `json:"environment"`
		ChainID     string `json:"chain_id"` // optional: links the run to a saved chain
		Async       bool   `json:"async"`    // return {run_id} immediately, run in background
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body", reqID)
		return
	}
	if body.YAMLText == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "yaml_text is required", reqID)
		return
	}

	chain, err := chains.Parse([]byte(body.YAMLText))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_chain", err.Error(), reqID)
		return
	}
	if errs := chain.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, "invalid_chain", errs[0], reqID)
		return
	}

	// Link the run to a saved chain only when the caller names one that
	// actually exists in this workspace; ad-hoc YAML runs stay unlinked.
	chainID := ""
	if body.ChainID != "" {
		if _, err := s.store.GetChain(r.Context(), pr.WorkspaceID, body.ChainID); err != nil {
			writeError(w, http.StatusNotFound, "chain_not_found", "chain not found", reqID)
			return
		}
		chainID = body.ChainID
	}

	target := body.Target
	if target == "" {
		target = chain.Target
	}
	env := body.Environment
	if env == "" {
		env = chain.Environment
	}
	if env == "" {
		env = "local"
	}

	// Pre-create the run as "running" so per-step webhook executions can link
	// to it via chain_run_id while the chain is still executing.
	run := &store.ChainRun{WorkspaceID: pr.WorkspaceID, ChainID: chainID, Name: chain.Name, TargetURL: target, Status: "running"}
	if _, err := s.store.CreateChainRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error(), reqID)
		return
	}

	if body.Async {
		// Run detached from the request context (which dies with the
		// response); the UI polls GET /v1/runs/{id} for live status.
		go s.runChainAsync(pr, chain, chainID, target, env, run.ID)
		writeJSON(w, http.StatusAccepted, map[string]string{"run_id": run.ID})
		return
	}

	result, err := s.runChain(r.Context(), pr, chain, chainID, target, env, run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chain_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// runChainAsync executes a chain in the background and finalizes its run row
// on completion or failure, so a run never stays "running" forever.
func (s *server) runChainAsync(pr *store.Principal, chain *chains.Chain, chainID, target, env, runID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if _, err := s.runChain(ctx, pr, chain, chainID, target, env, runID); err != nil {
		fail := &store.ChainRun{ID: runID, Status: "failed", Error: err.Error()}
		_ = s.store.FinalizeChainRun(context.Background(), fail)
	}
}

// runChain executes a chain and finalizes its previously created run row.
func (s *server) runChain(ctx context.Context, pr *store.Principal, chain *chains.Chain, chainID, target, env, runID string) (*chains.RunResult, error) {
	runner := &chains.Runner{
		API:        &serverDispatcher{s: s, pr: pr},
		Target:     target,
		Env:        env,
		ChainID:    chainID,
		ChainRunID: runID,
		TargetAllowed: func(u string) bool {
			return s.targetAllowed(ctx, pr, u)
		},
	}
	result, err := runner.Run(ctx, chain)
	if err != nil {
		return nil, err
	}

	// Persist the outcome: status, duration, and per-step results (response
	// bodies truncated to 4 KiB).
	final := chainRunFromResult(result, chainID, target)
	final.ID = runID
	var total int64
	for _, st := range result.Steps {
		total += st.LatencyMS
	}
	final.DurationMS = total
	if err := s.store.FinalizeChainRun(ctx, final); err != nil {
		return nil, err
	}
	return result, nil
}

// chainRunFromResult converts a runner result into a persisted chain run.
func chainRunFromResult(res *chains.RunResult, chainID, target string) *store.ChainRun {
	status := "passed"
	if !res.Passed {
		status = "failed"
	}
	run := &store.ChainRun{
		ChainID:   chainID,
		Name:      res.Name,
		Status:    status,
		TargetURL: target,
	}
	for _, sr := range res.Steps {
		errStr := sr.ExpectError
		if errStr == "" {
			errStr = sr.Error
		}
		body := string(sr.Body)
		if len(body) > 4<<10 {
			body = body[:4<<10]
		}
		run.Steps = append(run.Steps, store.ChainRunStep{
			StepName:       sr.Name,
			StepType:       sr.Type,
			RequestSummary: sr.RequestSummary,
			ResponseStatus: sr.Status,
			ResponseBody:   body,
			DurationMS:     sr.LatencyMS,
			Passed:         sr.Passed,
			Error:          errStr,
		})
	}
	return run
}

// handleListChainRuns returns recent runs for a chain, newest first.
func (s *server) handleListChainRuns(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	if pr == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing principal", reqID)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	runs, err := s.store.ListChainRuns(r.Context(), r.PathValue("id"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

// handleListRuns returns a workspace's recent chain runs, newest first.
func (s *server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	if pr == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing principal", reqID)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	runs, err := s.store.ListRecentRuns(r.Context(), pr.WorkspaceID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

// handleGetChainRun returns one run with its steps ordered by position.
func (s *server) handleGetChainRun(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	if pr == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing principal", reqID)
		return
	}
	run, err := s.store.GetChainRun(r.Context(), pr.WorkspaceID, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "run_not_found", "run not found", reqID)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// handleListRunExecutions returns the webhook executions of one chain run,
// oldest first, for step → execution lineage in Logs.
func (s *server) handleListRunExecutions(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	if pr == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing principal", reqID)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	execs, err := s.store.ListExecutionsByChainRun(r.Context(), pr.WorkspaceID, r.PathValue("id"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error(), reqID)
		return
	}
	out := make([]apiclient.Execution, 0, len(execs))
	for _, e := range execs {
		out = append(out, toExecutionJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": out})
}
