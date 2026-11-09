// Package tests contains KutubCore end-to-end tests (Milestone F).
// These exercise the full HTTP stack over real sockets (no ServeHTTP
// shortcuts): register -> organizer -> catalog -> acquisition -> library
// -> tracker -> offline catalog -> backup data still intact.
package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kutubcore/internal/testutil"

	_ "modernc.org/sqlite"
)

func mustCookie(t *testing.T, resp *http.Response) string {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == "kutubcore_session" && c.Value != "" {
			return c.Value
		}
	}
	t.Fatal("missing session cookie")
	return ""
}

func do(t *testing.T, client *http.Client, base, method, path, cookie string, body any) (*http.Response, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, base+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "kutubcore_session", Value: cookie})
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var raw bytes.Buffer
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp, raw.Bytes()
}

func idOf(t *testing.T, raw []byte) string {
	t.Helper()
	var v struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v.Data.ID
}

func TestE2E_HappyPath(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	app := testutil.NewApp(db)
	srv := httptest.NewServer(app.Handler)
	defer srv.Close()
	client := srv.Client()
	base := srv.URL

	// Health + catalog basics.
	resp, raw := do(t, client, base, "GET", "/healthz", "", nil)
	if resp.StatusCode != 200 || len(raw) == 0 {
		t.Fatalf("health: %d", resp.StatusCode)
	}

	// Register (auto-login) + organizer.
	resp, raw = do(t, client, base, "POST", "/api/v1/auth/register", "", map[string]any{
		"display_name": "E2E User", "email": "e2e@example.com", "password": "e2epassword1",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("register: %d %s", resp.StatusCode, raw)
	}
	cookie := mustCookie(t, resp)

	resp, raw = do(t, client, base, "POST", "/api/v1/users/me/roles/organizer", cookie, map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("organizer: %d %s", resp.StatusCode, raw)
	}

	// Writer + book + FTS search.
	resp, raw = do(t, client, base, "POST", "/api/v1/writers", cookie, map[string]any{"full_name": "E2E Author"})
	if resp.StatusCode != 201 {
		t.Fatalf("writer: %d %s", resp.StatusCode, raw)
	}
	writerID := idOf(t, raw)
	resp, raw = do(t, client, base, "POST", "/api/v1/books", cookie, map[string]any{
		"title": "E2E Title", "writers": []map[string]any{{"id": writerID}},
	})
	if resp.StatusCode != 201 {
		t.Fatalf("book: %d %s", resp.StatusCode, raw)
	}
	bookID := idOf(t, raw)

	resp, raw = do(t, client, base, "GET", "/api/v1/books?q=e2e", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("search: %d %s", resp.StatusCode, raw)
	}
	var search struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(raw, &search)
	if search.Meta.Total != 1 {
		t.Fatalf("search total = %d, want 1", search.Meta.Total)
	}

	// Acquisition (creates copies) -> library entry -> progress 100% auto-completes.
	resp, raw = do(t, client, base, "POST", "/api/v1/me/acquisitions", cookie, map[string]any{
		"items": []map[string]any{{"book_id": bookID, "format": "paperback", "quantity": 1}},
	})
	if resp.StatusCode != 201 {
		t.Fatalf("acquisition: %d %s", resp.StatusCode, raw)
	}
	resp, raw = do(t, client, base, "POST", "/api/v1/me/books", cookie, map[string]any{
		"book_id": bookID, "physical_status": "owned", "reading_status": "currently_reading",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("add library: %d %s", resp.StatusCode, raw)
	}
	ubID := idOf(t, raw)

	resp, raw = do(t, client, base, "PUT", fmt.Sprintf("/api/v1/me/books/%s/progress", ubID), cookie, map[string]any{"current_page": 10, "total_pages": 10})
	if resp.StatusCode != 200 {
		t.Fatalf("progress: %d %s", resp.StatusCode, raw)
	}
	resp, raw = do(t, client, base, "GET", "/api/v1/me/dashboard", cookie, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("dashboard: %d %s", resp.StatusCode, raw)
	}
	var dash struct {
		Data struct {
			Completed int `json:"completed"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &dash)
	if dash.Data.Completed != 1 {
		t.Fatalf("dashboard completed = %d, want 1", dash.Data.Completed)
	}

	// PWA: /sw.js present at root.
	resp, raw = do(t, client, base, "GET", "/sw.js", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("sw.js: %d %s", resp.StatusCode, raw)
	}

	// Data persisted in SQLite (spot-check, as backup would).
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM books WHERE id=?`, bookID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("db books = %d err %v", n, err)
	}
}

// Ensure the test binary can open a file DB too (backup/restore use files).
func TestE2E_FileDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE probe(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file db missing: %v", err)
	}
}
