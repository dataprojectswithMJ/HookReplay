package dispatch

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when a target host's breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open")

// CircuitBreaker opens a target host after `threshold` consecutive failures
// and stays open for `openDuration` before allowing a half-open probe (§5.4).
type CircuitBreaker struct {
	mu           sync.Mutex
	states       map[string]*breakerState
	threshold    int
	openDuration time.Duration
}

type breakerState struct {
	consecutive int
	openedAt    time.Time
	open        bool
}

// NewCircuitBreaker returns a breaker that opens after `threshold` failures
// for `openDuration`.
func NewCircuitBreaker(threshold int, openDuration time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		states:       map[string]*breakerState{},
		threshold:    threshold,
		openDuration: openDuration,
	}
}

// Allow reports whether a request may proceed to host. A closed or half-open
// breaker allows the request; an open breaker (within its cool-down) blocks it.
func (c *CircuitBreaker) Allow(host string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.states[host]
	if st == nil || !st.open {
		return nil
	}
	if time.Since(st.openedAt) >= c.openDuration {
		// Half-open: allow a single probe.
		st.open = false
		st.consecutive = 0
		return nil
	}
	return fmt.Errorf("%w: %s", ErrCircuitOpen, host)
}

// Record updates the breaker with the outcome of a request to host.
// A nil err (or <500 status, passed as nil) counts as success.
func (c *CircuitBreaker) Record(host string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.states[host]
	if st == nil {
		st = &breakerState{}
		c.states[host] = st
	}
	if err == nil {
		st.consecutive = 0
		st.open = false
		return
	}
	st.consecutive++
	if st.consecutive >= c.threshold {
		st.open = true
		st.openedAt = time.Now()
	}
}
