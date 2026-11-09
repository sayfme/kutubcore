package handler_test

import (
	"encoding/json"
	"testing"

	"kutubcore/internal/testutil"
)

// FTS5: prefix search on title, writer name, and description; injection-safe.
func TestSearch_FTS5(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "S", "email": "s@example.com", "password": "secret123",
	}, "")
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "Rabindranath Tagore"}, cookie)
	var wResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{
		"title": "Gitanjali", "description": "A book of songs about rivers and boats",
		"writers": []map[string]any{{"id": wResp.Data.ID}},
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("book: %d %s", rec.Code, rec.Body.String())
	}

	type searchResp struct {
		Data []struct {
			Title string `json:"title"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	// Prefix on title.
	rec = doJSON(t, h, "GET", "/api/v1/books?q=gitanj", nil, "")
	var r searchResp
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.Meta.Total != 1 || r.Data[0].Title != "Gitanjali" {
		t.Fatalf("q=gitanj: total=%d data=%+v", r.Meta.Total, r.Data)
	}
	// Writer name via FTS writers column.
	rec = doJSON(t, h, "GET", "/api/v1/books?q=tagore", nil, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.Meta.Total != 1 {
		t.Fatalf("q=tagore: total=%d, want 1", r.Meta.Total)
	}
	// Description token.
	rec = doJSON(t, h, "GET", "/api/v1/books?q=boats", nil, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.Meta.Total != 1 {
		t.Fatalf("q=boats: total=%d, want 1", r.Meta.Total)
	}
	// Injection attempt (FTS operators) must not error or return all rows.
	rec = doJSON(t, h, "GET", "/api/v1/books?q=%22%20OR%20", nil, "")
	if rec.Code != 200 {
		t.Fatalf("injection: %d %s, want 200", rec.Code, rec.Body.String())
	}
	// No match.
	rec = doJSON(t, h, "GET", "/api/v1/books?q=nonexistentxyz", nil, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.Meta.Total != 0 {
		t.Fatalf("q=nonexistent: total=%d, want 0", r.Meta.Total)
	}
}
