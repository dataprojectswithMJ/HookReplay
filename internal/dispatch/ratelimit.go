package dispatch

import (
	"sync"
	"time"
)

// RateLimiter is an in-memory token bucket keyed by an arbitrary string
// (workspace ID). It enforces the §5.4 per-workspace rate limit.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64 // bucket capacity
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimiter returns a limiter allowing `rate` tokens/sec with `burst` cap.
func NewRateLimiter(rate, burst float64) *RateLimiter {
	return &RateLimiter{
		buckets: map[string]*bucket{},
		rate:    rate,
		burst:   burst,
	}
}

// Allow reports whether one token is available for key.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	b, ok := r.buckets[key]
	if !ok {
		b = &bucket{tokens: r.burst, last: now}
		r.buckets[key] = b
	}
	// Refill.
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * r.rate
	if b.tokens > r.burst {
		b.tokens = r.burst
	}
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
