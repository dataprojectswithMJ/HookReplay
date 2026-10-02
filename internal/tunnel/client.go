package tunnel

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client connects to the tunnel server and forwards relayed requests to a
// local HTTP server (localhost:port).
type Client struct {
	APIToken  string
	Subdomain string
	LocalPort int
	WSBaseURL string // ws://host (derived from API base)
	Dialer    *websocket.Dialer

	writeMu sync.Mutex // gorilla allows only one concurrent writer
}

// PublicURL returns the URL a provider/API can POST to.
func (c *Client) PublicURL() string {
	return strings.TrimRight(strings.Replace(c.WSBaseURL, "ws", "http", 1), "/") + "/t/" + c.Subdomain + "/"
}

// Run connects (with backoff) and serves requests until ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	dialer := c.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	u := c.WSBaseURL + "/tunnel/connect?subdomain=" + url.QueryEscape(c.Subdomain)

	backoff := time.Second
	for {
		header := http.Header{}
		if c.APIToken != "" {
			header.Set("Authorization", "Bearer "+c.APIToken)
		}
		conn, _, err := dialer.DialContext(ctx, u, header)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}
		backoff = time.Second

		err = c.serve(ctx, conn)
		_ = conn.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

func (c *Client) serve(ctx context.Context, conn *websocket.Conn) error {
	for {
		var m message
		if err := conn.ReadJSON(&m); err != nil {
			return err
		}
		if m.Type != "request" {
			continue
		}
		go c.handleRequest(conn, m)
	}
}

func (c *Client) handleRequest(conn *websocket.Conn, m message) {
	resp := c.forward(m)
	c.writeMu.Lock()
	_ = conn.WriteJSON(resp)
	c.writeMu.Unlock()
}

func (c *Client) forward(m message) message {
	body, err := base64.StdEncoding.DecodeString(m.Body)
	if err != nil {
		body = nil
	}
	target := "http://localhost:" + strconv.Itoa(c.LocalPort) + m.Path

	req, err := http.NewRequest(m.Method, target, bytes.NewReader(body))
	if err != nil {
		return message{Type: "response", ID: m.ID, Status: http.StatusBadGateway, Error: err.Error()}
	}
	for k, v := range m.Headers {
		if isHopByHop(k) {
			continue
		}
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return message{Type: "response", ID: m.ID, Status: http.StatusBadGateway, Error: err.Error()}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	headers := map[string]string{}
	for k, vs := range resp.Header {
		if isHopByHop(k) {
			continue
		}
		headers[k] = strings.Join(vs, ", ")
	}

	return message{
		Type:    "response",
		ID:      m.ID,
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    base64.StdEncoding.EncodeToString(respBody),
	}
}
