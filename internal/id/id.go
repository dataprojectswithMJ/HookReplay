// Package id generates prefixed string identifiers (§2):
// hrk_ API keys, evt_ executions, chn_ chains, tmpl_ templates, sec_ secrets.
package id

import "github.com/google/uuid"

// New returns "<prefix><uuid>", e.g. id.New("evt_") -> "evt_...".
func New(prefix string) string {
	return prefix + uuid.NewString()
}
