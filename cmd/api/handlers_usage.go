package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *server) handleUsage(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	since := time.Now().AddDate(0, 0, -30)
	count, err := s.store.CountExecutionsSince(r.Context(), pr.WorkspaceID, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"period":       "30d",
		"executions":   count,
		"replays":      0,
		"tier":         pr.Tier,
		"exec_limit":   tierExecLimit(pr.Tier),
		"chains_limit": tierChainsLimit(pr.Tier),
	})
}

func tierExecLimit(tier string) any {
	if tier == "free" {
		return 1000
	}
	return nil // unlimited (fair use)
}

func tierChainsLimit(tier string) int {
	if tier == "free" {
		return 3
	}
	return -1 // unlimited
}

func (s *server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	var body struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil || body.Domain == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "domain is required", reqID)
		return
	}
	d, err := s.store.CreateDomain(r.Context(), pr.WorkspaceID, body.Domain)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (s *server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	list, err := s.store.ListDomains(r.Context(), pr.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	d, err := s.store.GetDomain(r.Context(), pr.WorkspaceID, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "domain_not_found", err.Error(), reqID)
		return
	}
	if !verifyServedToken(d.Domain, d.VerificationToken) {
		writeError(w, http.StatusBadRequest, "verification_failed",
			"token not found at /.well-known/hookreplay-verify", reqID)
		return
	}
	if err := s.store.MarkDomainVerified(r.Context(), pr.WorkspaceID, d.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true, "domain": d.Domain})
}

func verifyServedToken(domain, token string) bool {
	for _, scheme := range []string{"https", "http"} {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(scheme + "://" + domain + "/.well-known/hookreplay-verify")
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		if strings.TrimSpace(string(body)) == token {
			return true
		}
	}
	return false
}
