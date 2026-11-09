// Package storage handles filesystem storage (Blueprint §8).
// Files are stored under a configured base dir with generated safe names;
// uploaded filenames are never trusted as paths (prevent traversal).
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Save writes data to baseDir/subdir under a generated filename and
// returns the storage path relative to baseDir (e.g. books/<uuid>.pdf).
func Save(baseDir, subdir string, data []byte, ext string) (rel string, err error) {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext != "" {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
	}
	if err := validateExt(ext); err != nil {
		return "", err
	}
	dir := filepath.Join(baseDir, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create storage dir: %w", err)
	}
	name, err := randomName(ext)
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return filepath.Join(subdir, name), nil
}

// SafeExt validates an extension against the allowlist (caller supplies map).
func validateExt(ext string) error {
	// Extension content is caller-validated; here we only reject path tricks.
	if strings.Contains(ext, "/") || strings.Contains(ext, "\\") || strings.Contains(ext, "..") {
		return fmt.Errorf("invalid extension")
	}
	return nil
}

func randomName(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ext, nil
}

// Open returns full path for a stored relative path, rejecting traversal.
// Returns error if rel escapes baseDir.
func Open(baseDir, rel string) (string, error) {
	clean := filepath.Clean("/" + rel) // force relative
	rel = strings.TrimPrefix(clean, "/")
	full := filepath.Join(baseDir, rel)
	// Must stay inside baseDir.
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if abs != base && !strings.HasPrefix(abs, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal rejected")
	}
	return full, nil
}
