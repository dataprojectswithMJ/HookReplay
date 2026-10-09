package chains

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StepResult is the outcome of one step.
type StepResult struct {
	Name           string            `json:"name"`
	Type           string            `json:"type,omitempty"`            // webhook | api | custom
	RequestSummary string            `json:"request_summary,omitempty"` // method+URL for api, event for webhook
	Status         int               `json:"status"`
	Body           []byte            `json:"body,omitempty"` // base64 in JSON
	LatencyMS      int64             `json:"latency_ms"`
	Retries        int               `json:"retries,omitempty"`
	Delays         []string          `json:"delays,omitempty"`
	Error          string            `json:"error,omitempty"`
	Deliver        map[string]string `json:"deliver,omitempty"`
	Skipped        bool              `json:"skipped"`
	Failed         bool              `json:"failed"`
	Passed         bool              `json:"passed"`
	ExpectErrors   []string          `json:"expect_errors,omitempty"`
	ExpectError    string            `json:"expect_error,omitempty"` // joined ExpectErrors
}

// runCtx carries the runtime context shared across steps.
type runCtx struct {
	target string
	env    string
	steps  map[string]StepResult
}

var tmplRE = regexp.MustCompile(`\{\{[^}]+\}\}`)

// resolve replaces {{target}} and {{steps.*}} references in s.
func (rc *runCtx) resolve(s string) string {
	return tmplRE.ReplaceAllStringFunc(s, func(m string) string {
		inner := strings.TrimSpace(m[2 : len(m)-2])
		v, ok := rc.lookup(inner)
		if !ok {
			return m // leave unresolved
		}
		return stringify(v)
	})
}

func (rc *runCtx) lookup(inner string) (any, bool) {
	if inner == "target" {
		return rc.target, true
	}
	if !strings.HasPrefix(inner, "steps.") {
		return nil, false
	}
	rest := strings.TrimPrefix(inner, "steps.")
	name, field, _ := strings.Cut(rest, ".")
	sr, ok := rc.steps[name]
	if !ok {
		return nil, false
	}
	switch {
	case field == "status":
		return sr.Status, true
	case field == "body":
		return stringifyJSON(sr.Body), true
	case strings.HasPrefix(field, "body."):
		v, err := extractJSON(sr.Body, strings.TrimPrefix(field, "body."))
		if err != nil {
			return nil, false
		}
		return v, true
	default:
		return nil, false
	}
}

// evalExpect returns a list of unmet expectations.
func evalExpect(e *Expect, sr StepResult) []string {
	var errs []string

	if e.Status != nil {
		if !matchStatus(e.Status, sr.Status) {
			errs = append(errs, fmt.Sprintf("status: expected %v, got %d", e.Status, sr.Status))
		}
	}

	if e.LatencyUnder != "" {
		if d, err := time.ParseDuration(e.LatencyUnder); err == nil {
			if sr.LatencyMS > d.Milliseconds() {
				errs = append(errs, fmt.Sprintf("latency_under: expected < %s, got %dms", e.LatencyUnder, sr.LatencyMS))
			}
		}
	}

	if e.Retries != nil {
		if sr.Retries != *e.Retries {
			errs = append(errs, fmt.Sprintf("retries: expected %d, got %d", *e.Retries, sr.Retries))
		}
	}
	if len(e.Delays) > 0 {
		errs = append(errs, delaysMatch(e.Delays, sr.Delays)...)
	}

	if e.Body != nil {
		v, err := extractJSON(sr.Body, e.Body.JSONPath)
		if err != nil {
			if e.Body.Exists != nil && !*e.Body.Exists {
				// path absent, as required
			} else {
				errs = append(errs, fmt.Sprintf("body.%s: %v (body: %s)", e.Body.JSONPath, err, bodySnippet(sr.Body)))
			}
		} else {
			if e.Body.Exists != nil && !*e.Body.Exists {
				errs = append(errs, fmt.Sprintf("body.%s: expected path to not exist, got %v", e.Body.JSONPath, v))
			}
			if e.Body.Equals != nil && stringify(v) != stringify(e.Body.Equals) {
				errs = append(errs, fmt.Sprintf("body.%s: expected %v, got %v (body: %s)", e.Body.JSONPath, e.Body.Equals, v, bodySnippet(sr.Body)))
			}
			if e.Body.Contains != nil && !strings.Contains(stringify(v), stringify(e.Body.Contains)) {
				errs = append(errs, fmt.Sprintf("body.%s: %q does not contain %q (body: %s)", e.Body.JSONPath, stringify(v), stringify(e.Body.Contains), bodySnippet(sr.Body)))
			}
		}
	}

	return errs
}

// bodySnippet returns a trimmed (≤300 char) snippet of the body for error messages.
func bodySnippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// delayTolerance is the ±20% window around a configured delay within which
// an observed delay still counts as a match, so scheduler slop or a loaded
// test machine doesn't flake an otherwise-correct schedule.
const delayTolerance = 0.2

// delaysMatch compares two backoff sequences by duration, so "30s" matches
// "30000ms", allowing ±20% tolerance per entry. Non-duration entries fall
// back to string equality. It returns one error per mismatching delay plus a
// length mismatch error, each naming the expected range and observed value.
func delaysMatch(want, got []string) []string {
	var errs []string
	n := min(len(want), len(got))
	for i := 0; i < n; i++ {
		w, err1 := time.ParseDuration(want[i])
		g, err2 := time.ParseDuration(got[i])
		if err1 != nil || err2 != nil {
			if want[i] != got[i] {
				errs = append(errs, fmt.Sprintf("delay %d of %d: expected %q, observed %q", i+1, len(want), want[i], got[i]))
			}
			continue
		}
		lo := time.Duration(float64(w) * (1 - delayTolerance))
		hi := time.Duration(float64(w) * (1 + delayTolerance))
		if g < lo || g > hi {
			errs = append(errs, fmt.Sprintf("delay %d of %d: expected %s–%s, observed %s", i+1, len(want), lo, hi, g))
		}
	}
	if len(want) != len(got) {
		errs = append(errs, fmt.Sprintf("delays: expected %d delay(s) [%s], observed %d [%s]",
			len(want), strings.Join(want, ", "), len(got), strings.Join(got, ", ")))
	}
	return errs
}

func matchStatus(want any, got int) bool {
	switch w := want.(type) {
	case int:
		return got == w
	case int64:
		return got == int(w)
	case string:
		if strings.HasSuffix(w, "xx") {
			first := int(w[0] - '0')
			return got >= first*100 && got < (first+1)*100
		}
		n, err := strconv.Atoi(w)
		if err != nil {
			return false
		}
		return got == n
	default:
		return false
	}
}

func extractJSON(data []byte, path string) (any, error) {
	path = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(path), "$"), ".")
	if path == "" {
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	cur := v
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q: cannot descend into %T", path, cur)
		}
		cur, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("path %q: key %q not found", path, part)
		}
	}
	return cur, nil
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func stringifyJSON(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	b2, _ := json.Marshal(v)
	return string(b2)
}
