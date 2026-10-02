package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hookreplay/hookreplay/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

// issueKeyForUser issues a full-scoped API key for a user, creating a workspace
// if needed. If state != "", the key is also staged for CLI polling.
func (s *server) issueKeyForUser(ctx context.Context, userID, workspaceName, state string) (string, error) {
	wsID, err := s.store.EnsureWorkspace(ctx, userID, workspaceName)
	if err != nil {
		return "", err
	}
	raw, _, err := s.store.CreateAPIKey(ctx, userID, wsID, auth.AllScopes)
	if err != nil {
		return "", err
	}
	if state != "" {
		s.storePending(state, raw)
	}
	return raw, nil
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		State    string `json:"state"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body", reqID)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || !strings.Contains(email, "@") || len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"a valid email and a password (min 8 chars) are required", reqID)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	userID, err := s.store.CreateUserWithPassword(r.Context(), email, string(hash))
	if err != nil {
		writeError(w, http.StatusConflict, "email_taken", err.Error(), reqID)
		return
	}
	raw, err := s.issueKeyForUser(r.Context(), userID, "My Workspace", body.State)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	if body.State != "" {
		writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"api_key": raw})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		State    string `json:"state"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body", reqID)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	userID, hash, err := s.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	if userID == "" || hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password", reqID)
		return
	}
	raw, err := s.issueKeyForUser(r.Context(), userID, "My Workspace", body.State)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	if body.State != "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"api_key": raw})
}
