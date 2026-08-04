// Package migrations embeds the SQL schema applied on every service
// startup. The schema is small enough (one table) that a full migration
// tool is unnecessary ceremony: CREATE TABLE IF NOT EXISTS is idempotent
// and applying it on boot is enough to keep it in sync.
package migrations

import _ "embed"

// Schema is the full database schema, applied by adapter/postgres.EnsureSchema.
//
//go:embed schema.sql
var Schema string
