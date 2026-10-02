// Package templates is the provider template library. Fixtures live under
// templates/providers/{provider}/{event}/ as payload.json + sign.yml +
// delivery.yml + README.md and are embedded into the binary so the API and CLI
// are fully self-contained.
package templates

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/hookreplay/hookreplay/internal/signing"
	"gopkg.in/yaml.v3"
)

//go:embed providers
var FS embed.FS

// SignSpec describes how a template's payload is signed (§7.1).
type SignSpec struct {
	Provider        string `yaml:"provider" json:"provider"`
	Event           string `yaml:"event" json:"event"`
	Scheme          string `yaml:"scheme" json:"scheme"`
	SignatureHeader string `yaml:"signature_header" json:"signature_header"`
	Note            string `yaml:"note" json:"note"`
}

// DeliverySpec is the provider's retry backoff schedule (§7.3), stored as data.
type DeliverySpec struct {
	Provider         string   `yaml:"provider" json:"provider"`
	Event            string   `yaml:"event" json:"event"`
	AttemptIntervals []string `yaml:"attempt_intervals" json:"attempt_intervals"`
	MaxAttempts      int      `yaml:"max_attempts" json:"max_attempts"`
	TimeoutS         int      `yaml:"timeout_s" json:"timeout_s"`
}

// Template is a loaded provider/event fixture.
type Template struct {
	Provider string          `json:"provider"`
	Event    string          `json:"event"`
	Scheme   signing.Scheme  `json:"scheme"`
	Payload  json.RawMessage `json:"payload"`
	Sign     SignSpec        `json:"sign"`
	Delivery DeliverySpec    `json:"delivery"`
}

// Get loads a single template by provider/event.
func Get(provider, event string) (*Template, error) {
	base := "providers/" + provider + "/" + event

	payload, err := FS.ReadFile(base + "/payload.json")
	if err != nil {
		return nil, fmt.Errorf("templates: %s/%s: %w", provider, event, err)
	}

	signBytes, err := FS.ReadFile(base + "/sign.yml")
	if err != nil {
		return nil, fmt.Errorf("templates: %s/%s: %w", provider, event, err)
	}
	var sign SignSpec
	if err := yaml.Unmarshal(signBytes, &sign); err != nil {
		return nil, fmt.Errorf("templates: %s/%s sign.yml: %w", provider, event, err)
	}

	deliveryBytes, err := FS.ReadFile(base + "/delivery.yml")
	if err != nil {
		return nil, fmt.Errorf("templates: %s/%s: %w", provider, event, err)
	}
	var delivery DeliverySpec
	if err := yaml.Unmarshal(deliveryBytes, &delivery); err != nil {
		return nil, fmt.Errorf("templates: %s/%s delivery.yml: %w", provider, event, err)
	}

	scheme, err := signing.NormalizeScheme(sign.Scheme)
	if err != nil {
		return nil, fmt.Errorf("templates: %s/%s: %w", provider, event, err)
	}

	return &Template{
		Provider: provider,
		Event:    event,
		Scheme:   scheme,
		Payload:  payload,
		Sign:     sign,
		Delivery: delivery,
	}, nil
}

// List returns every template in the library, sorted by provider then event.
func List() ([]Template, error) {
	var out []Template = []Template{}
	err := fs.WalkDir(FS, "providers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		parts := strings.Split(path, "/")
		if len(parts) != 3 { // providers/provider/event
			return nil
		}
		t, err := Get(parts[1], parts[2])
		if err != nil {
			return nil // skip dirs that are not complete templates
		}
		out = append(out, *t)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Event < out[j].Event
	})
	return out, nil
}
