// Package dispatch delivers signed webhooks to a target (§5.4, §7.2). Signing
// happens at dispatch time, over the raw body bytes that are transmitted; the
// returned Result carries the exact bytes sent so callers can persist them
// byte-for-byte (the "hash sent == hash stored" invariant).
package dispatch

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/hookreplay/hookreplay/internal/signing"
)

const defaultTimeout = 30 * time.Second

// Target is the destination for a dispatch.
type Target struct {
	Kind string // "tunnel" | "url"
	URL  string
}

// Deliver configures failure simulation (§7.2).
type Deliver struct {
	Result string // "success" (default) | "fail"
	Reason string // invalid_signature|missing_signature|stale_signature|malformed_payload|connection_refused
}

// Request is a single webhook fire.
type Request struct {
	Scheme  signing.Scheme
	Body    []byte
	Secret  []byte
	Custom  *signing.CustomConfig
	Target  Target
	Deliver Deliver
	Timeout time.Duration
}

// Result is the outcome of a dispatch, including the exact bytes sent.
type Result struct {
	RequestBody     []byte
	SignatureHeader http.Header
	RequestHeaders  http.Header
	Status          int
	ResponseBody    []byte
	LatencyMS       int64
	ErrorCode       string
}

// Dispatcher is an HTTP sender with a per-host circuit breaker.
type Dispatcher struct {
	client  *http.Client
	breaker *CircuitBreaker
}

// New returns a Dispatcher with a 30s default timeout.
func New() *Dispatcher {
	return &Dispatcher{
		client:  &http.Client{},
		breaker: NewCircuitBreaker(5, 60*time.Second),
	}
}

// Dispatch signs (unless instructed otherwise) and POSTs body to the target.
func (d *Dispatcher) Dispatch(ctx context.Context, req Request) (Result, error) {
	if req.Timeout <= 0 {
		req.Timeout = defaultTimeout
	}

	body := append([]byte(nil), req.Body...)
	secret := append([]byte(nil), req.Secret...)
	host := hostFromURL(req.Target.URL)

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("User-Agent", "HookReplay/0.1.0")
	headers.Set("Accept", "*/*")

	// Gate before doing any work: if the breaker is open, nothing is sent or
	// signed.
	if err := d.breaker.Allow(host); err != nil {
		return Result{ErrorCode: "circuit_open", RequestHeaders: headers}, err
	}

	var sigHeader http.Header
	skipSign := false
	signTS := time.Now()

	if req.Deliver.Result == "fail" {
		switch req.Deliver.Reason {
		case "invalid_signature":
			secret = randomBytes(32)
		case "missing_signature":
			skipSign = true
		case "stale_signature":
			signTS = time.Now().Add(-(240 * time.Second))
		case "malformed_payload":
			body = corruptBytes(body)
		case "connection_refused":
			res, err := d.connectionRefused(req.Target.URL)
			res.RequestHeaders = headers
			return res, err
		}
	}

	if !skipSign && req.Scheme != "" {
		hdr, err := signing.Sign(req.Scheme, body, signing.Material{Key: secret, Custom: req.Custom}, signTS)
		if err != nil {
			return Result{}, err
		}
		sigHeader = hdr.Clone()
		for k, vs := range hdr {
			for _, v := range vs {
				headers.Add(k, v)
			}
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.Target.URL, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("dispatch: build request: %w", err)
	}
	httpReq.Header = headers

	start := time.Now()
	resp, err := d.client.Do(httpReq)
	latency := time.Since(start).Milliseconds()

	res := Result{
		RequestBody:     body,
		SignatureHeader: sigHeader,
		RequestHeaders:  headers,
		LatencyMS:       latency,
	}
	if err != nil {
		res.ErrorCode = classifyError(err)
		d.breaker.Record(host, err)
		return res, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	res.Status = resp.StatusCode
	res.ResponseBody = respBody

	if resp.StatusCode >= 500 {
		d.breaker.Record(host, fmt.Errorf("status %d", resp.StatusCode))
	} else {
		d.breaker.Record(host, nil)
	}
	return res, nil
}

// connectionRefused simulates a target that refuses connections (§7.2).
// It dials the target and records the refusal; no request is sent.
func (d *Dispatcher) connectionRefused(rawURL string) (Result, error) {
	host := hostFromURL(rawURL)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{ErrorCode: "connection_refused", LatencyMS: latency}, err
	}
	conn.Close()
	return Result{ErrorCode: "connection_refused", LatencyMS: latency},
		fmt.Errorf("dispatch: connection_refused simulation: target %s unexpectedly accepted a connection", rawURL)
}

func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Host
}

func classifyError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "target_timeout"
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return "target_timeout"
	}
	return "connection_error"
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a fixed nonce so we never panic.
		return bytes.Repeat([]byte{0xAB}, n)
	}
	return b
}

// corruptBytes flips a byte in the middle of body to simulate a malformed
// payload while preserving content-type and length.
func corruptBytes(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	out := append([]byte(nil), body...)
	out[len(out)/2] ^= 0xFF
	return out
}
