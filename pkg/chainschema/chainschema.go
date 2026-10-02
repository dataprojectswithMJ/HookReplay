// Package chainschema is the single normative definition of the HookReplay
// chain YAML schema. It is shared by the CLI (fmt/validate) and the dashboard
// (chain editor). The JSON-schema document is embedded so the CLI is a single
// static binary with no external file dependencies.
package chainschema

import _ "embed"

// SchemaJSON is the canonical JSON-schema for hookreplay.yml (§6 of the spec).
//
//go:embed schema.json
var SchemaJSON []byte

// SchemaID is the stable identifier embedded in the schema document.
const SchemaID = "https://hookreplay.dev/schemas/chain.schema.json"
