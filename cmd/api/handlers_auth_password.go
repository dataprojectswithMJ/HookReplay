package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// issueUserSession ensures the user has a workspace and creates a session token
// (identity). If state != "", the token is also staged for CLI polling. This is
// login — it does NOT issue an API key.
func (s *server) issueUserSession(ctx context.Context, userID, workspaceName, state string) (string, error) {
	if _, err := s.store.EnsureWorkspace(ctx, userID, workspaceName); err != nil {
		return "", err
	}
	raw, err := s.store.CreateSession(ctx, userID)
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
	raw, err := s.issueUserSession(r.Context(), userID, "My Workspace", body.State)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	if body.State != "" {
		writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"user_token": raw})
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
	raw, err := s.issueUserSession(r.Context(), userID, "My Workspace", body.State)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
		return
	}
	if body.State != "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"user_token": raw})
}
