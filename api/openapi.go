// Package api embeds linkguard's OpenAPI 3.0 specification so it can be
// served directly by the running binary, with no separate build step.
package api

import _ "embed"

// Spec is the raw OpenAPI 3.0 document, served at GET /openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
