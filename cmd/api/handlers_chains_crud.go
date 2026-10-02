package main

import (
	"encoding/json"
	"net/http"

	"github.com/hookreplay/hookreplay/internal/chains"
)

func (s *server) handleCreateChain(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	var body struct {
		Name     string `json:"name"`
		YAMLText string `json:"yaml_text"`
		Source   string `json:"source"`
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
	name := body.Name
	if name == "" {
		name = chain.Name
	}
	source := body.Source
	if source == "" {
		source = "dashboard"
	}
	c, err := s.store.CreateChain(r.Context(), pr.WorkspaceID, source, name, body.YAMLText)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *server) handleListChains(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	list, err := s.store.ListChains(r.Context(), pr.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleGetChain(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	c, err := s.store.GetChain(r.Context(), pr.WorkspaceID, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "chain_not_found", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *server) handleUpdateChain(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	pr := principalFrom(r.Context())
	var body struct {
		YAMLText string `json:"yaml_text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body", reqID)
		return
	}
	if err := s.store.UpdateChain(r.Context(), pr.WorkspaceID, r.PathValue("id"), body.YAMLText); err != nil {
		writeError(w, http.StatusNotFound, "chain_not_found", err.Error(), reqID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *server) handleDeleteChain(w http.ResponseWriter, r *http.Request) {
	pr := principalFrom(r.Context())
	if err := s.store.DeleteChain(r.Context(), pr.WorkspaceID, r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "chain_not_found", err.Error(), requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
