// Package migrations embeds the ordered SQL migration files so they
// are bundled into the binary (no external files needed at runtime).
// Go embed cannot reference ../ paths, so this package bridges
// migrations/sqlite/*.sql into cmd/kutubcore.
package migrations

import "embed"

// SQLite holds the SQLite migration files (dir "sqlite").
//
//go:embed sqlite/*.sql
var SQLite embed.FS
