package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rel, err := Save(dir, "books", []byte("%PDF hello"), ".pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rel, "books/") {
		t.Fatalf("rel = %q", rel)
	}
	full, err := Open(dir, rel)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "%PDF hello" {
		t.Fatalf("data = %q", data)
	}
	if !strings.HasSuffix(full, filepath.Ext(rel)) {
		t.Fatalf("full = %q", full)
	}
}

func TestOpenRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	// Traversal that would escape the base must be rejected or confined:
	// either err != nil, or the returned path still lives under dir.
	for _, p := range []string{"../secret", "..\\..\\secret", "/etc/passwd", "../../etc/passwd"} {
		full, err := Open(dir, p)
		if err == nil {
			if !strings.HasPrefix(full, dir) {
				t.Fatalf("Open(%q) escaped base: %q", p, full)
			}
		}
	}
	// Explicit escape attempts must not return an outside path.
	if full, err := Open(dir, "../secret"); err == nil {
		if strings.Contains(full, "secret") && !strings.HasPrefix(full, dir) {
			t.Fatalf("escape allowed: %q", full)
		}
	}
}
