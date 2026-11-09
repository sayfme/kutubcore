package database

import (
	"context"
	"database/sql"
	"embed"
	"testing"

	_ "modernc.org/sqlite"
)

//go:embed testdata/*.sql
var testFS embed.FS

func TestRunIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	first, err := Run(ctx, db, testFS, "testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0] != "0001_probe.sql" {
		t.Fatalf("first run = %v, want [0001_probe.sql]", first)
	}

	second, err := Run(ctx, db, testFS, "testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second run = %v, want [] (idempotent)", second)
	}

	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM probe`).Scan(&n); err != nil {
		t.Fatalf("probe table missing: %v", err)
	}
}
