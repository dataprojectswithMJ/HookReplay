package chains

import (
	"context"
	"strings"
	"time"

	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/hookreplay/hookreplay/templates"
)

// deliverySpecFor loads a webhook step's retry backoff schedule from the
// provider template. On any error it returns an empty spec (no retries).
func deliverySpecFor(webhook string) templates.DeliverySpec {
	provider, event, ok := strings.Cut(webhook, "/")
	if !ok {
		return templates.DeliverySpec{}
	}
	tmpl, err := templates.Get(provider, event)
	if err != nil {
		return templates.DeliverySpec{}
	}
	return tmpl.Delivery
}

// rehearseRetries drives a synchronous retry rehearsal: fire attempt 1, then
// on a non-2xx response re-fire after each backoff interval (up to
// maxAttempts), recording the observed retry count and delay sequence. It
// stops early on a 2xx. intervals[0] is the initial attempt and is ignored.
// A dispatch that returns an execution alongside its error (a recorded
// pre-send or transport failure, status 0) counts as a failed attempt; only
// an error with no execution aborts the rehearsal.
func rehearseRetries(ctx context.Context, dispatch func() (*apiclient.Execution, error), intervals []string, maxAttempts int) (retries int, delays []string, final *apiclient.Execution, err error) {
	exec, derr := dispatch()
	if derr != nil && exec == nil {
		return 0, nil, nil, derr
	}
	final = exec
	if exec.ResponseStatus >= 200 && exec.ResponseStatus < 300 {
		return 0, nil, final, nil
	}
	if len(intervals) <= 1 {
		return 0, nil, final, nil
	}
	if maxAttempts < 1 {
		maxAttempts = len(intervals)
	}
	for i := 1; i < len(intervals) && i < maxAttempts; i++ {
		d, perr := time.ParseDuration(intervals[i])
		if perr != nil {
			continue
		}
		waitStart := time.Now()
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return retries, delays, final, ctx.Err()
		}
		exec, derr = dispatch()
		if derr != nil && exec == nil {
			return retries, delays, final, derr
		}
		final = exec
		retries++
		// Record the observed wait, not the configured interval, so the
		// ±20% tolerance in delaysMatch actually exercises real timing.
		delays = append(delays, time.Since(waitStart).Round(time.Millisecond).String())
		if exec.ResponseStatus >= 200 && exec.ResponseStatus < 300 {
			break
		}
	}
	return retries, delays, final, nil
}
