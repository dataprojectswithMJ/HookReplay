package chains

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hookreplay/hookreplay/pkg/apiclient"
)

// RunResult is the aggregate outcome of a chain run.
type RunResult struct {
	Name   string       `json:"name"`
	Steps  []StepResult `json:"steps"`
	Passed bool         `json:"passed"`
}

// WebhookDispatcher fires a signed webhook and returns the execution outcome.
// The CLI satisfies this with apiclient.Client (HTTP → API); the API satisfies
// it with a direct internal dispatcher, so dashboard and CLI share one path.
type WebhookDispatcher interface {
	Dispatch(ctx context.Context, req apiclient.DispatchRequest) (*apiclient.Execution, error)
}

// Runner executes chains. Webhook steps go through the dispatcher; api steps
// call the user's app; custom steps sign and fire locally.
type Runner struct {
	API    WebhookDispatcher
	HTTP   *http.Client
	Target string
	Env    string

	// Chain context stamped onto each webhook dispatch so executions can be
	// linked back to the chain run (Logs lineage). Optional; server-side runs
	// set these, CLI runs leave them empty.
	ChainID    string
	ChainRunID string

	// TargetAllowed gates api/custom step URLs when the chain runs server-side.
	// nil means no check (CLI runs fire from the user's own machine).
	TargetAllowed func(string) bool
}

// Run executes the chain sequentially.
func (r *Runner) Run(ctx context.Context, c *Chain) (*RunResult, error) {
	res := &RunResult{Name: c.Name}
	rc := &runCtx{target: r.Target, env: r.Env, steps: map[string]StepResult{}}

	for i := range c.Steps {
		step := &c.Steps[i]

		if d, derr := parseDelay(step.Delay); derr == nil && d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return res, ctx.Err()
			}
		}

		sr, err := r.execute(ctx, step, rc)
		if err != nil {
			if sr.Error != "" {
				// execute already recorded a structured error code (e.g.
				// sign_error); keep the code and append the underlying detail.
				sr.ExpectErrors = append(sr.ExpectErrors, err.Error())
				sr.ExpectError = fmt.Sprintf("%s (%s)", sr.Error, err.Error())
			} else {
				sr.Error = err.Error()
			}
			sr.Failed = true
		} else if step.Expect != nil {
			if errs := evalExpect(step.Expect, sr); len(errs) > 0 {
				sr.Failed = true
				sr.ExpectErrors = errs
				sr.ExpectError = strings.Join(errs, "; ")
			}
		}
		sr.Passed = !sr.Failed && !sr.Skipped

		rc.steps[step.Name] = sr
		res.Steps = append(res.Steps, sr)

		// Fail-fast: a failed step aborts the chain. Mark the remaining steps
		// skipped so the UI/CLI can render "not run".
		if sr.Failed {
			for _, rest := range c.Steps[i+1:] {
				res.Steps = append(res.Steps, StepResult{Name: rest.Name, Skipped: true})
			}
			break
		}
	}

	res.Passed = true
	for _, s := range res.Steps {
		if s.Failed {
			res.Passed = false
		}
	}
	return res, nil
}

func (r *Runner) execute(ctx context.Context, step *Step, rc *runCtx) (StepResult, error) {
	switch {
	case step.Webhook != "":
		return r.executeWebhook(ctx, step, rc)
	case step.API != nil:
		return r.executeAPI(ctx, step, rc)
	case step.Custom != nil:
		return r.executeCustom(ctx, step, rc)
	default:
		return StepResult{}, fmt.Errorf("step %q has no webhook/api/custom", step.Name)
	}
}
