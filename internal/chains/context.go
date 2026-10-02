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
	Name         string   `json:"name"`
	Status       int      `json:"status"`
	Body         []byte   `json:"body,omitempty"` // base64 in JSON
	LatencyMS    int64    `json:"latency_ms"`
	Retries      int      `json:"retries,omitempty"`
	Delays       []string `json:"delays,omitempty"`
	Error        string   `json:"error,omitempty"`
	Skipped      bool     `json:"skipped"`
	Failed       bool     `json:"failed"`
	ExpectErrors []string `json:"expect_errors,omitempty"`
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

// resolveScalar resolves a single {{...}} token or returns a literal.
func (rc *runCtx) resolveScalar(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
		v, ok := rc.lookup(strings.TrimSpace(s[2 : len(s)-2]))
		if !ok {
			return ""
		}
		return stringify(v)
	}
	return strings.Trim(s, "\"'")
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

// evalWhen returns whether the step should run (true) or be skipped (false).
// An empty expression means "always run".
func (rc *runCtx) evalWhen(expr string) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}
	for _, op := range []string{"==", "!=", "contains"} {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			left := rc.resolveScalar(parts[0])
			right := rc.resolveScalar(parts[1])
			switch op {
			case "==":
				return left == right, nil
			case "!=":
				return left != right, nil
			case "contains":
				return strings.Contains(left, right), nil
			}
		}
	}
	return false, fmt.Errorf("unsupported when expression %q", expr)
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
		if !delaysEqual(e.Delays, sr.Delays) {
			errs = append(errs, fmt.Sprintf("delays: expected %v, got %v", e.Delays, sr.Delays))
		}
	}

	if e.Body != nil {
		v, err := extractJSON(sr.Body, e.Body.JSONPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("body.json_path: %v", err))
		} else {
			if e.Body.Exists != nil && *e.Body.Exists {
				// existence already implied by successful extraction
			}
			if e.Body.Equals != nil && stringify(v) != stringify(e.Body.Equals) {
				errs = append(errs, fmt.Sprintf("body.%s: expected %v, got %v", e.Body.JSONPath, e.Body.Equals, v))
			}
			if e.Body.Contains != nil && !strings.Contains(stringify(v), stringify(e.Body.Contains)) {
				errs = append(errs, fmt.Sprintf("body.%s: %q does not contain %q", e.Body.JSONPath, stringify(v), stringify(e.Body.Contains)))
			}
		}
	}

	return errs
}

// delaysEqual compares two backoff sequences by duration, so "5m" matches
// "300s". Falls back to string equality when either side is not a duration.
func delaysEqual(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		w, err1 := time.ParseDuration(want[i])
		g, err2 := time.ParseDuration(got[i])
		if err1 != nil || err2 != nil {
			if want[i] != got[i] {
				return false
			}
			continue
		}
		if w != g {
			return false
		}
	}
	return true
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
