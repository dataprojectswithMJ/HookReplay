package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hookreplay/hookreplay/internal/auth"
	"github.com/hookreplay/hookreplay/internal/store"
)

type ctxKey string

const principalKey ctxKey = "principal"

// auth validates the Bearer credential (user session `usr_` or API key `hrk_`)
// and injects the principal into context.
func (s *server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := requestID(r)
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing credential", reqID)
			return
		}
		var (
			pr  *store.Principal
			err error
		)
		if strings.HasPrefix(token, "usr_") {
			pr, err = s.store.PrincipalByUserTokenHash(r.Context(), auth.Hash(token))
		} else {
			pr, err = s.store.PrincipalByAPIKeyHash(r.Context(), auth.Hash(token))
		}
		if err != nil || pr == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credential", reqID)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey, pr)
		next(w, r.WithContext(ctx))
	})
}

func principalFrom(ctx context.Context) *store.Principal {
	pr, _ := ctx.Value(principalKey).(*store.Principal)
	return pr
}

// requireScope authenticates AND requires the key to grant a specific scope.
func (s *server) requireScope(scope string, next http.HandlerFunc) http.Handler {
	return s.auth(func(w http.ResponseWriter, r *http.Request) {
		pr := principalFrom(r.Context())
		if pr == nil || !pr.HasScope(scope) {
			writeError(w, http.StatusForbidden, "scope_required", "missing scope: "+scope, requestID(r))
			return
		}
		next(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// handleTunnelConnect authenticates then upgrades the WebSocket tunnel.
func (s *server) handleTunnelConnect(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing API key", requestID(r))
		return
	}
	if _, err := s.store.PrincipalByAPIKeyHash(r.Context(), auth.Hash(token)); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid API key", requestID(r))
		return
	}
	s.tunnel.HandleConnect(w, r)
}

func (s *server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = requestID(r)
			r.Header.Set("X-Request-ID", rid)
		}
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r)
	})
}

func (s *server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"dur", time.Since(start).String(),
			"request_id", r.Header.Get("X-Request-ID"),
		)
	})
}
