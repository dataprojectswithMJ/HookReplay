package main

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hookreplay/hookreplay/internal/id"
	"github.com/hookreplay/hookreplay/internal/store"
)

// handleAuthStart redirects the CLI's login to the web login page, carrying
// the state so the CLI can later poll /v1/auth/result.
func (s *server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		state = id.New("cli_")
	}
	http.Redirect(w, r, s.webBaseURL+"/login?state="+url.QueryEscape(state), http.StatusFound)
}

// handleOAuthStart redirects the browser to a provider's authorization page.
func (s *server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	provider := r.URL.Query().Get("provider")
	state := r.URL.Query().Get("state")
	if state == "" {
		state = id.New("web_")
	}
	switch provider {
	case "google":
		if !s.google.Configured() {
			writeError(w, http.StatusNotImplemented, "oauth_not_configured",
				"Google OAuth is not configured (set GOOGLE_CLIENT_ID + GOOGLE_CLIENT_SECRET)", reqID)
			return
		}
		http.Redirect(w, r, s.google.AuthorizeURL(state), http.StatusFound)
	default:
		if !s.github.Configured() {
			writeError(w, http.StatusNotImplemented, "oauth_not_configured",
				"GitHub OAuth is not configured (set GITHUB_CLIENT_ID + GITHUB_CLIENT_SECRET)", reqID)
			return
		}
		http.Redirect(w, r, s.github.AuthorizeURL(state), http.StatusFound)
	}
}

// handleOAuthCallback exchanges the code, upserts the user, issues a key, and
// redirects back to the web login page (which polls /v1/auth/result).
func (s *server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	provider := "github"
	if strings.HasSuffix(r.URL.Path, "/google") {
		provider = "google"
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		writeError(w, http.StatusBadRequest, "oauth_failed", "missing code", reqID)
		return
	}

	var email, providerID, workspaceName string
	switch provider {
	case "google":
		token, err := s.google.Exchange(r.Context(), code)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "oauth_failed", err.Error(), reqID)
			return
		}
		gu, err := s.google.User(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "oauth_failed", err.Error(), reqID)
			return
		}
		email = gu.Email
		if email == "" {
			email = gu.ID + "@users.noreply.google.com"
		}
		providerID = gu.ID
		workspaceName = email
	default:
		token, err := s.github.Exchange(r.Context(), code)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "oauth_failed", err.Error(), reqID)
			return
		}
		ghUser, err := s.github.User(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "oauth_failed", err.Error(), reqID)
			return
		}
		email = s.github.PrimaryEmail(r.Context(), token)
		if email == "" {
			email = ghUser.Login + "@users.noreply.github.com"
		}
		providerID = strconv.FormatInt(ghUser.ID, 10)
		workspaceName = ghUser.Login + "'s Workspace"
	}

	userID, err := s.store.UpsertUserByOAuth(r.Context(), email, provider, providerID)
	if err != nil {
		if errors.Is(err, store.ErrEmailLinkedElsewhere) {
			writeError(w, http.StatusConflict, "email_taken", err.Error(), reqID)
			return
		}
		writeError(w, http.StatusInternalServerError, "oauth_failed", err.Error(), reqID)
		return
	}
	if _, err := s.issueKeyForUser(r.Context(), userID, workspaceName, state); err != nil {
		writeError(w, http.StatusInternalServerError, "oauth_failed", err.Error(), reqID)
		return
	}
	http.Redirect(w, r, s.webBaseURL+"/login?state="+url.QueryEscape(state)+"&done=1", http.StatusFound)
}

// handleOAuthResult lets the CLI (or the web login page) poll for the key
// issued by a completed login.
func (s *server) handleOAuthResult(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	key, ok := s.takePending(state)
	if !ok {
		writeError(w, http.StatusNotFound, "oauth_pending", "no login result for this state", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"api_key": key})
}

