// Package chains implements the chain YAML schema (§6) and the sequential
// chain runner. Webhook steps execute through the API dispatch path; api steps
// call the user's app directly; custom steps sign and fire locally.
package chains

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Chain is a parsed hookreplay.yml (§6).
type Chain struct {
	Name        string `yaml:"name"`
	Target      string `yaml:"target"`
	Environment string `yaml:"environment"`
	Concurrency int    `yaml:"concurrency"`
	Steps       []Step `yaml:"steps"`
}

// Step is one step in a chain.
type Step struct {
	Name     string         `yaml:"name"`
	Webhook  string         `yaml:"webhook"`
	Custom   *CustomStep    `yaml:"custom"`
	API      *APIStep       `yaml:"api"`
	Secret   string         `yaml:"secret"`
	Override map[string]any `yaml:"override"`
	Delay    string         `yaml:"delay"`
	Deliver  *Deliver       `yaml:"deliver"`
	Expect   *Expect        `yaml:"expect"`
	When     string         `yaml:"when"`
}

// CustomStep fires an arbitrary signed payload (§6 custom).
type CustomStep struct {
	Payload any               `yaml:"payload"` // string (file path) or inline map
	Headers map[string]string `yaml:"headers"`
	Sign    *SignConfig       `yaml:"sign"`
}

// SignConfig is the custom signing block.
type SignConfig struct {
	Algorithm       string `yaml:"algorithm"`
	Secret          string `yaml:"secret"`
	Header          string `yaml:"header"`
	Format          string `yaml:"format"`
	Encoding        string `yaml:"encoding"`
	TimestampHeader string `yaml:"timestamp_header"`
}

// APIStep calls the user's app (the only direction we call them).
type APIStep struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Timeout string            `yaml:"timeout"`
}

// Deliver configures failure simulation.
type Deliver struct {
	Result string `yaml:"result"`
	Reason string `yaml:"reason"`
}

// Expect asserts on a step's outcome.
type Expect struct {
	Status       any         `yaml:"status"` // int or "2xx"
	LatencyUnder string      `yaml:"latency_under"`
	Retries      *int        `yaml:"retries"`
	Delays       []string    `yaml:"delays"`
	Body         *BodyExpect `yaml:"body"`
}

// BodyExpect asserts on a JSON path in the response body.
type BodyExpect struct {
	JSONPath string `yaml:"json_path"`
	Equals   any    `yaml:"equals"`
	Contains any    `yaml:"contains"`
	Exists   *bool  `yaml:"exists"`
}

// Parse parses chain YAML.
func Parse(data []byte) (*Chain, error) {
	var c Chain
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Name == "" {
		return nil, fmt.Errorf("chain.name is required")
	}
	if len(c.Steps) == 0 {
		return nil, fmt.Errorf("chain.steps is required")
	}
	if c.Concurrency == 0 {
		c.Concurrency = 3
	}
	return &c, nil
}

// Validate performs structural validation.
func (c *Chain) Validate() []string {
	var errs []string
	seen := map[string]bool{}
	for i, s := range c.Steps {
		if s.Name == "" {
			errs = append(errs, fmt.Sprintf("steps[%d].name is required", i))
		} else if seen[s.Name] {
			errs = append(errs, fmt.Sprintf("steps[%d].name %q is not unique", i, s.Name))
		}
		seen[s.Name] = true

		kinds := 0
		if s.Webhook != "" {
			kinds++
		}
		if s.Custom != nil {
			kinds++
		}
		if s.API != nil {
			kinds++
		}
		if kinds != 1 {
			errs = append(errs, fmt.Sprintf("steps[%d] must set exactly one of webhook/custom/api", i))
		}
		if s.Secret != "" && !strings.HasPrefix(s.Secret, "env:") {
			errs = append(errs, fmt.Sprintf("steps[%d].secret must be env:NAME", i))
		}
	}
	return errs
}

func parseDelay(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
