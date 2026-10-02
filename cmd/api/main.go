package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hookreplay/hookreplay/internal/auth"
	"github.com/hookreplay/hookreplay/internal/crypt"
	"github.com/hookreplay/hookreplay/internal/dispatch"
	"github.com/hookreplay/hookreplay/internal/oauth"
	"github.com/hookreplay/hookreplay/internal/store"
	"github.com/hookreplay/hookreplay/internal/tunnel"
)

type server struct {
	store      *store.Store
	dispatcher *dispatch.Dispatcher
	tunnel     *tunnel.Server
	limiter    *dispatch.RateLimiter
	masterKey  []byte
	logger     *slog.Logger
	github     *oauth.GitHub
	google     *oauth.Google
	webBaseURL string

	pendingMu sync.Mutex
	pending   map[string]pendingAuth
}

// pendingAuth holds a freshly issued key until the CLI polls for it.
type pendingAuth struct {
	APIKey    string
	ExpiresAt time.Time
}

func (s *server) storePending(state, apiKey string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if s.pending == nil {
		s.pending = map[string]pendingAuth{}
	}
	s.pending[state] = pendingAuth{APIKey: apiKey, ExpiresAt: time.Now().Add(5 * time.Minute)}
}

func (s *server) takePending(state string) (string, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	p, ok := s.pending[state]
	if !ok {
		return "", false
	}
	delete(s.pending, state)
	if time.Now().After(p.ExpiresAt) {
		return "", false
	}
	return p.APIKey, true
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	masterKey, err := crypt.MasterKeyFromEnv()
	if err != nil {
		logger.Error("master key", "err", err)
		os.Exit(1)
	}

	databaseURL := envOr("HOOKREPLAY_DATABASE_URL",
		"postgres://hookreplay:hookreplay@localhost:5432/hookreplay?sslmode=disable")
	st, err := store.New(ctx, databaseURL, os.Getenv("HOOKREPLAY_DEV_API_KEY"))
	if err != nil {
		logger.Error("store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	publicBase := envOr("HOOKREPLAY_PUBLIC_BASE_URL", "http://localhost:8080")
	gh := oauth.NewGitHub(
		os.Getenv("GITHUB_CLIENT_ID"),
		os.Getenv("GITHUB_CLIENT_SECRET"),
		publicBase+"/v1/auth/oauth/callback",
	)
	goog := oauth.NewGoogle(
		os.Getenv("GOOGLE_CLIENT_ID"),
		os.Getenv("GOOGLE_CLIENT_SECRET"),
		publicBase+"/v1/auth/oauth/callback/google",
	)

	srv := &server{
		store:      st,
		dispatcher: dispatch.New(),
		tunnel:     tunnel.NewServer(),
		limiter:    dispatch.NewRateLimiter(10, 30), // 10/s, burst 30 (§5.4)
		masterKey:  masterKey,
		logger:     logger,
		github:     gh,
		google:     goog,
		webBaseURL: envOr("HOOKREPLAY_WEB_BASE_URL", "http://localhost:3000"),
	}

	mux := http.NewServeMux()

	// Public.
	mux.HandleFunc("GET /v1/healthz", srv.handleHealthz)
	mux.HandleFunc("GET /v1/templates", srv.handleListTemplates)
	mux.HandleFunc("GET /v1/templates/{provider}/{event}", srv.handleGetTemplate)
	mux.HandleFunc("GET /v1/auth/start", srv.handleAuthStart)
	mux.HandleFunc("GET /v1/auth/result", srv.handleOAuthResult)
	mux.HandleFunc("POST /v1/auth/register", srv.handleRegister)
	mux.HandleFunc("POST /v1/auth/login", srv.handleLogin)
	mux.HandleFunc("GET /v1/auth/oauth/start", srv.handleOAuthStart)
	mux.HandleFunc("GET /v1/auth/oauth/callback", srv.handleOAuthCallback)
	mux.HandleFunc("GET /v1/auth/oauth/callback/google", srv.handleOAuthCallback)
	mux.HandleFunc("GET /v1/auth/oauth/result", srv.handleOAuthResult)

	// Scoped (scope enforcement per §5.1 / WS-8).
	mux.Handle("POST /v1/dispatch", srv.requireScope(auth.ScopeDispatch, srv.handleDispatch))
	mux.Handle("POST /v1/chains/execute", srv.requireScope(auth.ScopeDispatch, srv.handleExecuteChain))
	mux.Handle("GET /v1/chains", srv.requireScope(auth.ScopeRead, srv.handleListChains))
	mux.Handle("GET /v1/chains/{id}", srv.requireScope(auth.ScopeRead, srv.handleGetChain))
	mux.Handle("POST /v1/chains", srv.requireScope(auth.ScopeDispatch, srv.handleCreateChain))
	mux.Handle("PUT /v1/chains/{id}", srv.requireScope(auth.ScopeDispatch, srv.handleUpdateChain))
	mux.Handle("DELETE /v1/chains/{id}", srv.requireScope(auth.ScopeDispatch, srv.handleDeleteChain))
	mux.Handle("GET /v1/usage", srv.requireScope(auth.ScopeRead, srv.handleUsage))
	mux.Handle("GET /v1/domains", srv.requireScope(auth.ScopeRead, srv.handleListDomains))
	mux.Handle("POST /v1/domains", srv.requireScope(auth.ScopeAdmin, srv.handleCreateDomain))
	mux.Handle("POST /v1/domains/{id}/verify", srv.requireScope(auth.ScopeAdmin, srv.handleVerifyDomain))
	mux.Handle("GET /v1/executions", srv.requireScope(auth.ScopeRead, srv.handleListExecutions))
	mux.Handle("GET /v1/executions/{id}", srv.requireScope(auth.ScopeRead, srv.handleGetExecution))
	mux.Handle("GET /v1/me", srv.requireScope(auth.ScopeRead, srv.handleMe))
	mux.Handle("GET /v1/secrets", srv.requireScope(auth.ScopeRead, srv.handleListSecrets))
	mux.Handle("GET /v1/workspaces", srv.requireScope(auth.ScopeRead, srv.handleListWorkspaces))
	mux.Handle("PUT /v1/secrets/{env}/{name}", srv.requireScope(auth.ScopeSecrets, srv.handleSetSecret))
	mux.Handle("DELETE /v1/secrets/{env}/{name}", srv.requireScope(auth.ScopeSecrets, srv.handleDeleteSecret))
	mux.Handle("POST /v1/api-keys", srv.requireScope(auth.ScopeAdmin, srv.handleCreateAPIKey))
	mux.Handle("GET /v1/api-keys", srv.requireScope(auth.ScopeAdmin, srv.handleListAPIKeys))
	mux.Handle("DELETE /v1/api-keys/{id}", srv.requireScope(auth.ScopeAdmin, srv.handleDeleteAPIKey))
	mux.Handle("POST /v1/workspaces", srv.requireScope(auth.ScopeAdmin, srv.handleCreateWorkspace))

	// Tunnel.
	mux.HandleFunc("GET /tunnel/connect", srv.handleTunnelConnect)
	mux.Handle("/t/", srv.tunnel.Handler())

	handler := srv.withRequestID(srv.withLogging(mux))

	addr := envOr("HOOKREPLAY_HTTP_ADDR", ":8080")
	logger.Info("listening", "addr", addr)

	httpSrv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("serve", "err", err)
		os.Exit(1)
	}
}

