// Package postgres is the future PostgreSQL adapter (Milestone 5).
// Foundation (Milestone 1) uses SQLite only; this package is a
// documented placeholder so the adapter boundary exists from day one
// (Blueprint §13: SQLite/PostgreSQL-specific code stays in adapters).
package postgres

import "errors"

// ErrNotImplemented is returned until the PostgreSQL adapter is built.
var ErrNotImplemented = errors.New("postgres adapter: not implemented (Milestone 5)")
