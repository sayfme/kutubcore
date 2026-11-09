// Package database provides migration support shared by adapters.
// Blueprint §6/§13: explicit, ordered migrations; never ad-hoc DDL.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// EnsureSchemaMigrations creates the bookkeeping table. It is idempotent.
func EnsureSchemaMigrations(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);`)
	return err
}

// applied returns the set of already-applied migration versions.
func applied(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

// Run applies all unapplied *.sql files from fsys in lexical order.
// Each migration runs in its own transaction and is recorded only
// on success, so a failed migration is retried on next startup.
func Run(ctx context.Context, db *sql.DB, fsys embed.FS, dir string) (appliedNow []string, err error) {
	if err := EnsureSchemaMigrations(ctx, db); err != nil {
		return nil, fmt.Errorf("ensure schema_migrations: %w", err)
	}
	already, err := applied(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		if already[name] {
			continue
		}
		body, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", name, err)
		}
		if err := applyOne(ctx, db, name, string(body)); err != nil {
			return nil, err
		}
		appliedNow = append(appliedNow, name)
	}
	return appliedNow, nil
}

func applyOne(ctx context.Context, db *sql.DB, name, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", name, err)
	}
	defer tx.Rollback() // no-op after Commit

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("apply migration %q: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
		return fmt.Errorf("record migration %q: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", name, err)
	}
	return nil
}
