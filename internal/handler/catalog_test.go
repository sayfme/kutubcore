package handler_test

import (
	"encoding/json"
	"testing"

	"kutubcore/internal/testutil"
)

func TestCatalogFlow_WriterBookEdition(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	// Register + organizer
	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Organizer", "email": "org@example.com", "password": "secret123",
	}, "")
	if rec.Code != 201 {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	if rec.Code != 200 {
		t.Fatalf("organizer: %d %s", rec.Code, rec.Body.String())
	}

	// Create writer
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "Rabindranath Tagore"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("create writer: %d %s", rec.Code, rec.Body.String())
	}
	var wResp struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"full_name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	writerID := wResp.Data.ID
	if writerID == "" {
		t.Fatal("writer id empty")
	}

	// Create book linked to writer
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{
		"title": "Gitanjali", "writers": []map[string]any{{"id": writerID, "role": "author"}},
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("create book: %d %s", rec.Code, rec.Body.String())
	}
	var bResp struct {
		Data struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Ver    int    `json:"version"`
			Writer []struct {
				Name string `json:"name"`
			} `json:"writers"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bResp)
	bookID := bResp.Data.ID
	if bookID == "" || bResp.Data.Title != "Gitanjali" {
		t.Fatalf("book = %+v", bResp.Data)
	}
	if len(bResp.Data.Writer) != 1 || bResp.Data.Writer[0].Name != "Rabindranath Tagore" {
		t.Fatalf("writers = %+v", bResp.Data.Writer)
	}

	// Duplicate without force -> 409
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{
		"title": "Gitanjali", "writers": []map[string]any{{"id": writerID}},
	}, cookie)
	if rec.Code != 409 {
		t.Fatalf("duplicate: status %d body %s, want 409", rec.Code, rec.Body.String())
	}
	// With force -> 201
	rec = doJSON(t, h, "POST", "/api/v1/books?force=true", map[string]any{
		"title": "Gitanjali", "writers": []map[string]any{{"id": writerID}},
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("duplicate force: status %d body %s", rec.Code, rec.Body.String())
	}

	// Create edition (language code lookup via languages list)
	rec = doJSON(t, h, "GET", "/api/v1/languages", nil, "")
	if rec.Code != 200 {
		t.Fatalf("languages: %d %s", rec.Code, rec.Body.String())
	}
	var langs struct {
		Data []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &langs)
	var enID string
	for _, l := range langs.Data {
		if l.Code == "en" {
			enID = l.ID
		}
	}
	if enID == "" {
		t.Fatal("english language not seeded")
	}
	rec = doJSON(t, h, "POST", "/api/v1/books/"+bookID+"/editions", map[string]any{
		"language_id": enID, "publisher": "Test Pub", "publication_year": 1912,
		"format": "paperback", "page_count": 120,
		"identifiers": []map[string]any{{"identifier_type": "isbn13", "identifier_value": "9780000000000"}},
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("create edition: %d %s", rec.Code, rec.Body.String())
	}

	// Get book shows edition + identifiers
	rec = doJSON(t, h, "GET", "/api/v1/books/"+bookID, nil, "")
	if rec.Code != 200 {
		t.Fatalf("get book: %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Data struct {
			Editions []struct {
				Format      string `json:"format"`
				Identifiers []struct {
					Value string `json:"identifier_value"`
				} `json:"identifiers"`
			} `json:"editions"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Data.Editions) != 1 || got.Data.Editions[0].Format != "paperback" {
		t.Fatalf("editions = %+v", got.Data.Editions)
	}
	if len(got.Data.Editions[0].Identifiers) != 1 || got.Data.Editions[0].Identifiers[0].Value != "9780000000000" {
		t.Fatalf("identifiers = %+v", got.Data.Editions[0].Identifiers)
	}

	// Non-organizer cannot delete
	rec = doJSON(t, h, "GET", "/api/v1/writers?q=Tagore", nil, "")
	if rec.Code != 200 {
		t.Fatalf("list writers: %d %s", rec.Code, rec.Body.String())
	}

	// Update book with version + delete (soft)
	rec = doJSON(t, h, "PATCH", "/api/v1/books/"+bookID, map[string]any{"title": "Gitanjali (revised)", "version": bResp.Data.Ver}, cookie)
	if rec.Code != 200 {
		t.Fatalf("update book: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "DELETE", "/api/v1/books/"+bookID, nil, cookie)
	if rec.Code != 204 {
		t.Fatalf("delete book: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/api/v1/books/"+bookID, nil, "")
	if rec.Code != 404 {
		t.Fatalf("get deleted book: status %d body %s, want 404", rec.Code, rec.Body.String())
	}
}
