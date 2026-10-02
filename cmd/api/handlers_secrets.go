package main

import (
	"encoding/json"
	"net/http"

	"github.com/hookreplay/hookreplay/internal/crypt"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
)

func (s *server) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	list, err := s.store.ListExecutions(r.Context(), pr.WorkspaceID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	out := make([]apiclient.Execution, 0, len(list))
	for _, e := range list {
		out = append(out, toExecutionJSON(e))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	e, err := s.store.GetExecution(r.Context(), pr.WorkspaceID, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "execution_not_found", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, toExecutionJSON(e))
}

func (s *server) handleSetSecret(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	env := r.PathValue("env")
	name := r.PathValue("name")

	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil || body.Value == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "value is required", reqID)
		return
	}
	ct, err := crypt.Encrypt([]byte(body.Value), s.masterKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	if err := s.store.SetSecret(r.Context(), pr.WorkspaceID, env, name, ct); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stored", "env": env, "name": name})
}

func (s *server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	if err := s.store.DeleteSecret(r.Context(), pr.WorkspaceID, r.PathValue("env"), r.PathValue("name")); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	list, err := s.store.ListSecrets(r.Context(), pr.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	out := make([]apiclient.SecretName, 0, len(list))
	for _, m := range list {
		out = append(out, apiclient.SecretName{Env: m.Env, Name: m.Name, UpdatedAt: m.UpdatedAt})
	}
	writeJSON(w, http.StatusOK, out)
}
