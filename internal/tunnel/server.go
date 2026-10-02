package tunnel

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hookreplay/hookreplay/internal/id"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// In production this should be locked to the dashboard/api origin.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type clientConn struct {
	subdomain string
	ws        *websocket.Conn
	mu        sync.Mutex // serializes WS writes
	pending   map[string]chan message
}

// Server holds the live tunnel registry and relays requests.
type Server struct {
	mu    sync.RWMutex
	conns map[string]*clientConn
}

// NewServer returns an empty tunnel registry.
func NewServer() *Server {
	return &Server{conns: map[string]*clientConn{}}
}

// HandleConnect upgrades a WebSocket and registers the subdomain.
func (s *Server) HandleConnect(w http.ResponseWriter, r *http.Request) {
	subdomain := r.URL.Query().Get("subdomain")
	if subdomain == "" {
		http.Error(w, "missing subdomain", http.StatusBadRequest)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &clientConn{subdomain: subdomain, ws: ws, pending: map[string]chan message{}}
	s.mu.Lock()
	if old, ok := s.conns[subdomain]; ok {
		_ = old.ws.Close()
	}
	s.conns[subdomain] = c
	s.mu.Unlock()

	s.readLoop(c)
}

func (s *Server) readLoop(c *clientConn) {
	defer func() {
		s.mu.Lock()
		delete(s.conns, c.subdomain)
		s.mu.Unlock()
		_ = c.ws.Close()
	}()
	for {
		var m message
		if err := c.ws.ReadJSON(&m); err != nil {
			return
		}
		if m.Type != "response" {
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[m.ID]
		delete(c.pending, m.ID)
		c.mu.Unlock()
		if ok {
			ch <- m
		}
	}
}

// Handler relays path-based tunnel requests of the form /t/{subdomain}/...
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subdomain, rest := parseSubdomainPath(r.URL.Path)
		if subdomain == "" {
			http.NotFound(w, r)
			return
		}
		s.mu.RLock()
		c := s.conns[subdomain]
		s.mu.RUnlock()
		if c == nil {
			http.Error(w, `{"error":{"code":"tunnel_disconnected","message":"tunnel not connected"}}`, http.StatusBadGateway)
			return
		}

		body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		headers := map[string]string{}
		for k, vs := range r.Header {
			if isHopByHop(k) {
				continue
			}
			headers[k] = strings.Join(vs, ", ")
		}

		m := message{
			Type:    "request",
			ID:      id.New("req_"),
			Method:  r.Method,
			Path:    rest,
			Headers: headers,
			Body:    base64.StdEncoding.EncodeToString(body),
		}
		ch := make(chan message, 1)
		c.mu.Lock()
		c.pending[m.ID] = ch
		err := c.ws.WriteJSON(m)
		c.mu.Unlock()
		if err != nil {
			http.Error(w, `{"error":{"code":"relay_error","message":"failed to relay"}}`, http.StatusBadGateway)
			return
		}

		select {
		case resp := <-ch:
			writeResponse(w, resp)
		case <-time.After(30 * time.Second):
			http.Error(w, `{"error":{"code":"relay_timeout","message":"relay timed out"}}`, http.StatusGatewayTimeout)
		case <-r.Context().Done():
			return
		}
	})
}

func writeResponse(w http.ResponseWriter, m message) {
	for k, v := range m.Headers {
		if isHopByHop(k) {
			continue
		}
		w.Header().Set(k, v)
	}
	if m.Status == 0 {
		m.Status = http.StatusOK
	}
	w.WriteHeader(m.Status)
	body, err := base64.StdEncoding.DecodeString(m.Body)
	if err != nil {
		return
	}
	_, _ = w.Write(body)
}

// parseSubdomainPath splits "/t/{subdomain}/{rest}" into its parts.
func parseSubdomainPath(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 2 || parts[0] != "t" {
		return "", ""
	}
	rest := "/"
	if len(parts) == 3 {
		rest += parts[2]
	}
	return parts[1], rest
}

// MarshalMessage is exported for tests that inspect the protocol.
func MarshalMessage(m message) ([]byte, error) { return json.Marshal(m) }
