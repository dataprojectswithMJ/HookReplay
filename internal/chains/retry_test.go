package chains

import (
	"context"
	"testing"

	"github.com/hookreplay/hookreplay/pkg/apiclient"
)

// fakeDispatch returns the given statuses in order, then repeats the last.
func fakeDispatch(statuses ...int) func() (*apiclient.Execution, error) {
	i := 0
	return func() (*apiclient.Execution, error) {
		s := statuses[i]
		if i < len(statuses)-1 {
			i++
		}
		return &apiclient.Execution{ResponseStatus: s}, nil
	}
}

func TestRehearseRetries(t *testing.T) {
	t.Run("success first try", func(t *testing.T) {
		retries, delays, final, err := rehearseRetries(context.Background(),
			fakeDispatch(200), []string{"0s", "1s"}, 3)
		if err != nil {
			t.Fatal(err)
		}
		if retries != 0 || len(delays) != 0 || final.ResponseStatus != 200 {
			t.Fatalf("got retries=%d delays=%v status=%d", retries, delays, final.ResponseStatus)
		}
	})

	t.Run("recovers on second attempt", func(t *testing.T) {
		retries, delays, final, err := rehearseRetries(context.Background(),
			fakeDispatch(500, 200), []string{"0s", "1ms"}, 3)
		if err != nil {
			t.Fatal(err)
		}
		if retries != 1 || len(delays) != 1 || delays[0] != "1ms" || final.ResponseStatus != 200 {
			t.Fatalf("got retries=%d delays=%v status=%d", retries, delays, final.ResponseStatus)
		}
	})

	t.Run("exhausts attempts", func(t *testing.T) {
		retries, delays, final, err := rehearseRetries(context.Background(),
			fakeDispatch(500, 500, 500), []string{"0s", "1ms", "1ms"}, 3)
		if err != nil {
			t.Fatal(err)
		}
		if retries != 2 || len(delays) != 2 || final.ResponseStatus != 500 {
			t.Fatalf("got retries=%d delays=%v status=%d", retries, delays, final.ResponseStatus)
		}
	})

	t.Run("no intervals means no retries", func(t *testing.T) {
		retries, _, final, err := rehearseRetries(context.Background(),
			fakeDispatch(500), []string{"0s"}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if retries != 0 || final.ResponseStatus != 500 {
			t.Fatalf("got retries=%d status=%d", retries, final.ResponseStatus)
		}
	})

	t.Run("context cancellation stops rehearsal", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		retries, _, _, err := rehearseRetries(ctx,
			fakeDispatch(500, 500), []string{"0s", "1h"}, 3)
		if err == nil {
			t.Fatal("expected context cancellation error")
		}
		if retries != 0 {
			t.Fatalf("expected 0 retries on cancel, got %d", retries)
		}
	})
}

func TestEvalExpectRetriesDelays(t *testing.T) {
	sr := StepResult{Status: 200, Retries: 2, Delays: []string{"5s", "30s"}}

	if errs := evalExpect(&Expect{Retries: intPtr(2), Delays: []string{"5s", "30s"}}, sr); len(errs) != 0 {
		t.Fatalf("expected pass, got %v", errs)
	}

	if errs := evalExpect(&Expect{Retries: intPtr(3)}, sr); len(errs) == 0 {
		t.Fatal("expected retries mismatch to fail")
	}

	if errs := evalExpect(&Expect{Delays: []string{"5s", "1m"}}, sr); len(errs) == 0 {
		t.Fatal("expected delays mismatch to fail")
	}

	// Duration equivalence: "30s" == "30000ms".
	if errs := evalExpect(&Expect{Delays: []string{"5s", "30000ms"}}, sr); len(errs) != 0 {
		t.Fatalf("expected duration-equivalent delays to pass, got %v", errs)
	}
}

func intPtr(i int) *int { return &i }

// fakeDispatcher implements WebhookDispatcher with a canned status sequence.
type fakeDispatcher struct {
	statuses []int
	i        int
}

func (f *fakeDispatcher) Dispatch(_ context.Context, _ apiclient.DispatchRequest) (*apiclient.Execution, error) {
	s := f.statuses[f.i]
	if f.i < len(f.statuses)-1 {
		f.i++
	}
	return &apiclient.Execution{ResponseStatus: s}, nil
}

// TestExecuteWebhookRetryRehearsal runs a real chain through executeWebhook,
// using the Slack template's 5s backoff to observe a 500 → 200 recovery.
func TestExecuteWebhookRetryRehearsal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 5s rehearsal in -short mode")
	}
	runner := &Runner{
		API:    &fakeDispatcher{statuses: []int{500, 200}},
		Target: "http://example.test",
	}
	chain := &Chain{
		Name: "retry-rehearsal",
		Steps: []Step{{
			Name:    "callback",
			Webhook: "slack/event_callback",
			Expect:  &Expect{Retries: intPtr(1), Delays: []string{"5s"}},
		}},
	}
	res, err := runner.Run(context.Background(), chain)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatalf("expected pass, got steps: %+v", res.Steps)
	}
	s := res.Steps[0]
	if s.Retries != 1 || len(s.Delays) != 1 || s.Delays[0] != "5s" {
		t.Fatalf("unexpected retry result: %+v", s)
	}
}

