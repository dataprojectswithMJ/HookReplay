package main

import (
	"context"
	"time"

	"github.com/hookreplay/hookreplay/internal/store"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/hookreplay/hookreplay/templates"
)

// scheduleRetries spawns re-delivery goroutines per the provider's backoff
// schedule in delivery.yml. intervals[0] is the initial attempt (0s), so
// retries start at intervals[1]. Each retry records a separate executions row
// sharing parent_execution_id (§5.4 / §7.3).
func (s *server) scheduleRetries(pr *store.Principal, req apiclient.DispatchRequest, delivery templates.DeliverySpec, parentID string) {
	plan := retryPlan(delivery.AttemptIntervals, delivery.MaxAttempts)
	for i, d := range plan {
		attempt := i + 2 // attempt 1 is the initial dispatch
		go func(d time.Duration, attempt int) {
			time.Sleep(d)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// dispatchInternal re-resolves the secret and records the attempt
			// with parent_execution_id set; attempt > 1 skips idempotency dedup
			// and re-scheduling.
			s.dispatchInternal(ctx, pr, req, parentID, attempt)
		}(d, attempt)
	}
}

// retryPlan returns the backoff delays for re-deliveries (attempts 2..N),
// dropping invalid intervals and capping at maxAttempts. intervals[0] is the
// initial attempt and is ignored.
func retryPlan(intervals []string, maxAttempts int) []time.Duration {
	if len(intervals) <= 1 {
		return nil
	}
	if maxAttempts < 1 {
		maxAttempts = len(intervals)
	}
	var out []time.Duration
	for i := 1; i < len(intervals) && i < maxAttempts; i++ {
		d, err := time.ParseDuration(intervals[i])
		if err != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}
