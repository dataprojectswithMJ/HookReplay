package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/hookreplay/hookreplay/internal/auth"
	"github.com/hookreplay/hookreplay/internal/store"
)

func (s *server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())

	var body struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body)
	if len(body.Scopes) == 0 {
		body.Scopes = auth.AllScopes
	}
	raw, keyID, err := s.store.CreateAPIKey(r.Context(), pr.UserID, pr.WorkspaceID, body.Name, body.Scopes)
	if err != nil {
		if errors.Is(err, store.ErrAPIKeyNameExists) {
			writeError(w, http.StatusConflict, "api_key_name_exists", err.Error(), reqID)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": keyID, "api_key": raw})
}

func (s *server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	keys, err := s.store.ListAPIKeys(r.Context(), pr.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	if err := s.store.DeleteAPIKey(r.Context(), pr.WorkspaceID, r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "api_key_not_found", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	list, err := s.store.ListWorkspaces(r.Context(), pr.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "name is required", reqID)
		return
	}
	wsID, err := s.store.CreateWorkspace(r.Context(), body.Name, pr.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": wsID, "name": body.Name})
}
