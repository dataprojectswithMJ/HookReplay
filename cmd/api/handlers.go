package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hookreplay/hookreplay/internal/dispatch"
	"github.com/hookreplay/hookreplay/internal/id"
	"github.com/hookreplay/hookreplay/internal/store"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/hookreplay/hookreplay/templates"
)

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	writeJSON(w, http.StatusOK, apiclient.Me{
		UserID:      pr.UserID,
		WorkspaceID: pr.WorkspaceID,
		Tier:        pr.Tier,
		Email:       pr.Email,
	})
}

func (s *server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	list, err := templates.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	out := make([]apiclient.Template, 0, len(list))
	for _, t := range list {
		out = append(out, apiclient.Template{
			Provider: t.Provider,
			Event:    t.Event,
			Scheme:   string(t.Scheme),
			Note:     t.Sign.Note,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := templates.Get(r.PathValue("provider"), r.PathValue("event"))
	if err != nil {
		writeError(w, http.StatusNotFound, "template_not_found", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, apiclient.Template{
		Provider:         t.Provider,
		Event:            t.Event,
		Scheme:           string(t.Scheme),
		Note:             t.Sign.Note,
		SignatureHeader:  t.Sign.SignatureHeader,
		Payload:          t.Payload,
		AttemptIntervals: t.Delivery.AttemptIntervals,
		MaxAttempts:      t.Delivery.MaxAttempts,
		TimeoutS:         t.Delivery.TimeoutS,
	})
}

func (s *server) handleDispatch(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	if pr == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing principal", reqID)
		return
	}

	var req apiclient.DispatchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body", reqID)
		return
	}
	if req.Target == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "target is required", reqID)
		return
	}
	if req.Template == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "template is required", reqID)
		return
	}
	if req.Deliver != nil && req.Deliver.Result == "fail" && !validDeliverReason(req.Deliver.Reason) {
		writeError(w, http.StatusBadRequest, "invalid_request", "unknown deliver.reason", reqID)
		return
	}

	exec, err := s.dispatchOnce(r.Context(), pr, req)
	if err != nil {
		writeAPIError(w, err, reqID)
		return
	}
	writeJSON(w, http.StatusOK, exec)
}

// dispatchOnce is the shared dispatch core, used by both the HTTP handler and
// the server-side chain executor.
func (s *server) dispatchOnce(ctx context.Context, pr *store.Principal, req apiclient.DispatchRequest) (*apiclient.Execution, error) {
	return s.dispatchInternal(ctx, pr, req, "", 1)
}

func (s *server) dispatchInternal(ctx context.Context, pr *store.Principal, req apiclient.DispatchRequest, parentID string, attempt int) (*apiclient.Execution, error) {
	if pr.Tier == "free" && !isTunnelOrLocal(req.Target) {
		return nil, errAPI(http.StatusForbidden, "tier_required",
			"Free tier can only fire at tunnel targets. Upgrade to Pro to fire at public URLs.")
	}
	if !s.limiter.Allow(pr.WorkspaceID) {
		return nil, errAPI(http.StatusTooManyRequests, "rate_limited", "too many requests")
	}

	provider, event, ok := strings.Cut(req.Template, "/")
	if !ok || provider == "" || event == "" {
		return nil, errAPI(http.StatusBadRequest, "invalid_request", "template must be provider/event")
	}
	tmpl, err := templates.Get(provider, event)
	if err != nil {
		return nil, errAPI(http.StatusNotFound, "template_not_found", err.Error())
	}

	body, err := renderPayload(tmpl.Payload, req.Overrides)
	if err != nil {
		return nil, errAPI(http.StatusBadRequest, "invalid_override", err.Error())
	}

	secretBytes, err := s.resolveSecret(ctx, pr.WorkspaceID, req)
	if err != nil {
		return nil, errAPI(http.StatusBadRequest, "secret_resolution_failed", err.Error())
	}
	defer clear(secretBytes)

	if attempt == 1 && req.IdempotencyKey != "" {
		if existing, err := s.store.GetExecutionByIdempotency(ctx, pr.WorkspaceID, req.IdempotencyKey); err == nil {
			e := toExecutionJSON(existing)
			return &e, nil
		}
	}

	dreq := dispatch.Request{
		Scheme: tmpl.Scheme,
		Body:   body,
		Secret: secretBytes,
		Target: dispatch.Target{Kind: targetKind(req.Target), URL: req.Target},
	}
	if req.Deliver != nil {
		dreq.Deliver = dispatch.Deliver{Result: req.Deliver.Result, Reason: req.Deliver.Reason}
	}

	res, _ := s.dispatcher.Dispatch(ctx, dreq)
	if res.RequestBody == nil {
		res.RequestBody = []byte{}
	}

	exec := &store.Execution{
		ID:                id.New("evt_"),
		WorkspaceID:       pr.WorkspaceID,
		StepName:          event,
		TargetKind:        dreq.Target.Kind,
		Target:            req.Target,
		Provider:          provider,
		TemplateID:        req.Template,
		RequestBody:       res.RequestBody,
		RequestHeaders:    headerToMap(res.RequestHeaders),
		SignatureHeader:   headerToMap(res.SignatureHeader),
		Deliver:           deliverToMap(req.Deliver),
		ErrorCode:         res.ErrorCode,
		ResponseStatus:    res.Status,
		ResponseBody:      res.ResponseBody,
		LatencyMS:         res.LatencyMS,
		Attempt:           attempt,
		ParentExecutionID: parentID,
		IdempotencyKey:    req.IdempotencyKey,
	}
	if err := s.store.CreateExecution(ctx, exec); err != nil {
		if errors.Is(err, store.ErrIdempotencyConflict) {
			if existing, err := s.store.GetExecutionByIdempotency(ctx, pr.WorkspaceID, req.IdempotencyKey); err == nil {
				e := toExecutionJSON(existing)
				return &e, nil
			}
		}
		return nil, fmt.Errorf("persist execution: %w", err)
	}

	// Retry observation (§5.4 / §7.3): on non-2xx, schedule re-deliveries per
	// the provider's delivery.yml backoff, sharing parent_execution_id.
	if attempt == 1 && !req.SuppressRetries && (res.Status >= 400 || res.Status == 0) && (req.Deliver == nil || req.Deliver.Result != "fail") {
		s.scheduleRetries(pr, req, tmpl.Delivery, exec.ID)
	}

	e := toExecutionJSON(exec)
	return &e, nil
}
