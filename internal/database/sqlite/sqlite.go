// Package sqlite provides the SQLite adapter for KutubCore.
// Blueprint §6: portable SQL, UTC timestamps, UUID PKs, parameterized
// queries, and foreign keys enforced on every connection.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at path, configures
// per-connection pragmas, and verifies connectivity.
func Open(ctx context.Context, path string, logger *slog.Logger) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir %q: %w", dir, err)
		}
	}

	// DSN: WAL for better concurrency, busy_timeout for LAN use,
	// foreign_keys enforced per connection via init query.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite: single writer; avoid "database is locked".
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Belt-and-braces: ensure FKs are on even if DSN parsing changes.
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if logger != nil {
		logger.Info("sqlite ready", "path", path)
	}
	return db, nil
}
