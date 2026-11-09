// Package backup implements SQLite backup/restore (Blueprint §15).
// A backup is a directory containing:
//
//	kutubcore.db            consistent snapshot (VACUUM INTO)
//	files/…                 uploaded file tree, byte-for-byte
//	manifest.json           product + UTC timestamp + schema version +
//	                        sha256/size of every file (integrity proof)
//
// Restore verifies manifest checksums + DB integrity before replacing
// live data, staging through a safety copy of the current DB.
package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Manifest describes one backup directory.
type Manifest struct {
	Product       string      `json:"product"`
	CreatedAt     string      `json:"created_at"`
	SchemaVersion string      `json:"schema_version"`
	Files         []FileEntry `json:"files"`
}

// FileEntry is one backed-up file with its integrity hash.
type FileEntry struct {
	Path   string `json:"path"` // relative to backup dir (forward slashes)
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

const (
	dbName       = "kutubcore.db"
	filesSubdir  = "files"
	manifestName = "manifest.json"
)

// Create snapshots db (VACUUM INTO) + fileDir into backupRoot/<timestamp>/.
func Create(ctx context.Context, db *sql.DB, fileDir, backupRoot string) (string, error) {
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		return "", fmt.Errorf("create backup root: %w", err)
	}
	// Nanosecond precision avoids same-second collisions (two backups back to back).
	stamp := time.Now().UTC().Format("20060102-150405.000000000")
	dir := filepath.Join(backupRoot, stamp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	// 1) Consistent DB snapshot. VACUUM INTO writes a fresh file.
	snapshot := filepath.Join(dir, dbName)
	// Escape single quotes for the SQL literal.
	literal := "'" + strings.ReplaceAll(snapshot, "'", "''") + "'"
	if _, err := db.ExecContext(ctx, "VACUUM INTO "+literal); err != nil {
		return "", fmt.Errorf("snapshot db: %w", err)
	}

	// 2) Copy the uploaded-files tree (if any).
	srcFiles := filepath.Join(dir, filesSubdir)
	if fileDir != "" {
		if _, err := os.Stat(fileDir); err == nil {
			if err := copyTree(fileDir, srcFiles); err != nil {
				return "", fmt.Errorf("copy files: %w", err)
			}
		}
	}

	// 3) Manifest with checksums of every file except manifest.json itself.
	schemaVersion := ""
	var sv string
	_ = db.QueryRowContext(ctx, `SELECT value FROM app_meta WHERE key='schema_version'`).Scan(&sv)
	schemaVersion = sv

	entries, err := collectFiles(dir)
	if err != nil {
		return "", err
	}
	m := Manifest{Product: "KutubCore", CreatedAt: time.Now().UTC().Format(time.RFC3339), SchemaVersion: schemaVersion, Files: entries}
	raw, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, manifestName), append(raw, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}
	return dir, nil
}

// Verify checks manifest + checksums + DB integrity without modifying anything.
// Returns the manifest on success.
func Verify(backupDir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(backupDir, manifestName))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	for _, e := range m.Files {
		full := filepath.Join(backupDir, filepath.FromSlash(e.Path))
		got, err := fileHash(full)
		if err != nil {
			return nil, fmt.Errorf("checksum %s: %w", e.Path, err)
		}
		if got != e.SHA256 {
			return nil, fmt.Errorf("checksum mismatch for %s", e.Path)
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, err
		}
		if info.Size() != e.Size {
			return nil, fmt.Errorf("size mismatch for %s", e.Path)
		}
	}
	if err := checkDBIntegrity(filepath.Join(backupDir, dbName)); err != nil {
		return nil, err
	}
	return &m, nil
}

// Restore verifies the backup, keeps a safety copy of the current DB,
// then installs the snapshot + files. Call with the server STOPPED.
func Restore(ctx context.Context, backupDir, dbPath, fileDir string) (Manifest, error) {
	m, err := Verify(backupDir)
	if err != nil {
		return Manifest{}, err
	}
	// Safety copy of the current live DB (if any).
	if _, err := os.Stat(dbPath); err == nil {
		safety := dbPath + ".pre-restore-" + time.Now().UTC().Format("20060102-150405") + ".bak"
		if err := copyFile(dbPath, safety); err != nil {
			return Manifest{}, fmt.Errorf("safety copy current db: %w", err)
		}
	}
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Manifest{}, err
		}
	}
	// WAL/journal sidecars from the old DB must not survive: their stale
	// frames would replay writes that are NOT in the snapshot.
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		_ = os.Remove(dbPath + suffix)
	}
	if err := copyFile(filepath.Join(backupDir, dbName), dbPath); err != nil {
		return Manifest{}, fmt.Errorf("restore db: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		_ = os.Remove(dbPath + suffix)
	}
	if fileDir != "" {
		srcFiles := filepath.Join(backupDir, filesSubdir)
		if _, err := os.Stat(srcFiles); err == nil {
			if err := copyTree(srcFiles, fileDir); err != nil {
				return Manifest{}, fmt.Errorf("restore files: %w", err)
			}
		}
	}
	// PRAGMA foreign_keys is per-connection (not persisted); migrations run
	// on next server start. Verify the restored DB opens + integrity again.
	if err := checkDBIntegrity(dbPath); err != nil {
		return Manifest{}, fmt.Errorf("restored db integrity: %w", err)
	}
	return *m, nil
}

func checkDBIntegrity(dbPath string) error {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity_check failed: %s", result)
	}
	var version string
	if err := db.QueryRow(`SELECT value FROM app_meta WHERE key='schema_version'`).Scan(&version); err != nil {
		return fmt.Errorf("schema_version missing: %w", err)
	}
	return nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func collectFiles(root string) ([]FileEntry, error) {
	var out []FileEntry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.Base(rel) == manifestName {
			return nil
		}
		sum, err := fileHash(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, FileEntry{Path: filepath.ToSlash(rel), Size: info.Size(), SHA256: sum})
		return nil
	})
	return out, err
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if dir := filepath.Dir(target); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
