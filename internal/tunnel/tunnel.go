// Package tunnel implements the WebSocket tunnel that relays HTTP requests to
// a developer's localhost (§5.4 / §8 `tunnel`). Requests are relayed
// hop-by-hop with proper header handling; hop-by-hop headers are stripped.
package tunnel

import "net/http"

// message is the JSON frame exchanged over the WebSocket.
type message struct {
	Type    string            `json:"type"` // "request" | "response"
	ID      string            `json:"id"`
	Method  string            `json:"method,omitempty"`
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"` // base64
	Status  int               `json:"status,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// hopByHopHeaders are stripped on relay per RFC 7230 §6.1.
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Proxy-Connection":    true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func isHopByHop(h string) bool {
	return hopByHopHeaders[http.CanonicalHeaderKey(h)]
}
