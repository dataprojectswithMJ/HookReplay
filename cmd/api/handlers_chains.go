package main

import (
	"context"
	"encoding/json"
	"net/http"

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

	runner := &chains.Runner{
		API:    &serverDispatcher{s: s, pr: pr},
		Target: target,
		Env:    env,
	}
	result, err := runner.Run(r.Context(), chain)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chain_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
