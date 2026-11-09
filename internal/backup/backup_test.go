package backup_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"kutubcore/internal/backup"
	"kutubcore/internal/database"
	"kutubcore/internal/testutil"
	"kutubcore/migrations"
)

// fullDB returns a file-backed DB with migrations + one row + one uploaded file.
func fullDB(t *testing.T, dir string) (*sql.DB, string, string) {
	t.Helper()
	dbPath := filepath.Join(dir, "live.db")
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Run(context.Background(), db, migrations.SQLite, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO users(id, display_name, is_active) VALUES('u1','Backup User',1)`); err != nil {
		t.Fatal(err)
	}
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filepath.Join(filesDir, "books"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "books", "demo.pdf"), []byte("%PDF backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	return db, dbPath, filesDir
}

func TestCreateVerifyRestore(t *testing.T) {
	dir := t.TempDir()
	db, _, filesDir := fullDB(t, dir)
	defer db.Close()
	root := filepath.Join(dir, "backups")
	ctx := context.Background()

	bdir, err := backup.Create(ctx, db, filesDir, root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := backup.Verify(bdir)
	if err != nil {
		t.Fatalf("verify good backup: %v", err)
	}
	if m.Product != "KutubCore" || m.SchemaVersion == "" || len(m.Files) < 2 {
		t.Fatalf("manifest = %+v", m)
	}

	// Tamper with a file: verify must fail.
	if err := os.WriteFile(filepath.Join(bdir, "files", "books", "demo.pdf"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := backup.Verify(bdir); err == nil {
		t.Fatal("verify should fail after tamper")
	}

	// Fresh backup, restore into a new location.
	bdir2, err := backup.Create(ctx, db, filesDir, root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "restored")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	restoredDB := filepath.Join(out, "kutubcore.db")
	restoredFiles := filepath.Join(out, "files")
	if _, err := backup.Restore(ctx, bdir2, restoredDB, restoredFiles); err != nil {
		t.Fatalf("restore: %v", err)
	}
	// A junk row inserted into the live DB (uncheckpointed WAL) must not
	// survive restore: WAL sidecars must be removed before installing.
	liveWAL, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "wal.db"))
	if err != nil {
		t.Fatal(err)
	}
	liveWAL.SetMaxOpenConns(1)
	if _, err := liveWAL.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Run(context.Background(), liveWAL, migrations.SQLite, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err := liveWAL.Exec(`INSERT INTO users(id, display_name, is_active) VALUES('junk','Junk',1)`); err != nil {
		t.Fatal(err)
	}
	// Snapshot the WAL db, then restore it to itself: junk must be gone.
	walBackup, err := backup.Create(context.Background(), liveWAL, "", filepath.Join(dir, "wal-backups"))
	if err != nil {
		t.Fatal(err)
	}
	liveWAL.Exec(`INSERT INTO users(id, display_name, is_active) VALUES('junk2','Junk2',1)`)
	liveWAL.Close()
	walRestored := filepath.Join(dir, "wal-restored.db")
	walFiles := filepath.Join(dir, "wal-files")
	if _, err := backup.Restore(context.Background(), walBackup, walRestored, walFiles); err != nil {
		t.Fatalf("wal restore: %v", err)
	}
	wdb, _ := sql.Open("sqlite", "file:"+walRestored+"?mode=ro")
	var n, n2 int
	_ = wdb.QueryRow(`SELECT COUNT(*) FROM users WHERE id='junk2'`).Scan(&n)
	if n != 0 {
		t.Fatalf("post-snapshot junk2 survived restore: %d (stale WAL replay?)", n)
	}
	_ = wdb.QueryRow(`SELECT COUNT(*) FROM users WHERE id='junk'`).Scan(&n2)
	if n2 != 1 {
		t.Fatalf("pre-snapshot junk missing after restore: %d", n2)
	}
	wdb.Close()

	// Restored data present.
	rdb, err := sql.Open("sqlite", "file:"+restoredDB+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	var name string
	if err := rdb.QueryRow(`SELECT display_name FROM users WHERE id='u1'`).Scan(&name); err != nil {
		t.Fatalf("restored user missing: %v", err)
	}
	if name != "Backup User" {
		t.Fatalf("restored user = %q", name)
	}
	data, err := os.ReadFile(filepath.Join(restoredFiles, "books", "demo.pdf"))
	if err != nil || string(data) != "%PDF backup" {
		t.Fatalf("restored file = %q err %v", data, err)
	}
	_ = testutil.NewDB // keep testutil import for migration parity in other runs
}
