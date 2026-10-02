package main

import (
	"testing"
	"time"
)

func TestRetryPlan(t *testing.T) {
	cases := []struct {
		name        string
		intervals   []string
		maxAttempts int
		want        []time.Duration
	}{
		{
			name:        "single attempt",
			intervals:   []string{"0s"},
			maxAttempts: 1,
			want:        nil,
		},
		{
			name:        "slack short backoff",
			intervals:   []string{"0s", "5s", "1m"},
			maxAttempts: 3,
			want:        []time.Duration{5 * time.Second, time.Minute},
		},
		{
			name:        "capped by max_attempts",
			intervals:   []string{"0s", "5m", "30m", "2h"},
			maxAttempts: 2,
			want:        []time.Duration{5 * time.Minute},
		},
		{
			name:        "drops invalid interval",
			intervals:   []string{"0s", "bogus", "1h"},
			maxAttempts: 4,
			want:        []time.Duration{time.Hour},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := retryPlan(tc.intervals, tc.maxAttempts)
			if len(got) != len(tc.want) {
				t.Fatalf("retryPlan() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("retryPlan() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
