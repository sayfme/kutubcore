package handler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"kutubcore/internal/testutil"
)

// uploadPDF posts a tiny valid PDF upload to the book files endpoint.
func uploadPDF(t *testing.T, h http.Handler, cookie, bookID string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "book.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("%PDF-1.4 minimal")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/books/"+bookID+"/files", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "kutubcore_session", Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("upload pdf: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPersonalFlow_StatusProgressNoteReview(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	// Organizer user.
	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Org", "email": "org2@example.com", "password": "secret123",
	}, "")
	if rec.Code != 201 {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	if rec.Code != 200 {
		t.Fatalf("organizer: %d %s", rec.Code, rec.Body.String())
	}
	// Book.
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "Test Author"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("writer: %d %s", rec.Code, rec.Body.String())
	}
	var wResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{
		"title": "Personal Test Book", "writers": []map[string]any{{"id": wResp.Data.ID}},
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("book: %d %s", rec.Code, rec.Body.String())
	}
	var bResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bResp)
	bookID := bResp.Data.ID

	// Guest cannot access /me/books.
	rec = doJSON(t, h, "GET", "/api/v1/me/books", nil, "")
	if rec.Code != 401 {
		t.Fatalf("guest /me/books: %d %s", rec.Code, rec.Body.String())
	}

	// Owned without a copy -> 422 (Rule 1); downloaded without a file -> 422 (Rule 2).
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{
		"book_id": bookID, "physical_status": "owned", "digital_status": "downloaded",
		"reading_status": "currently_reading", "is_favorite": true,
	}, cookie)
	if rec.Code != 422 {
		t.Fatalf("owned without copy: %d %s, want 422", rec.Code, rec.Body.String())
	}
	// Record acquisition (creates a copy) + upload a PDF (creates a file).
	rec = doJSON(t, h, "POST", "/api/v1/me/acquisitions", map[string]any{
		"items": []map[string]any{{"book_id": bookID, "format": "paperback", "quantity": 1}},
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("acquisition: %d %s", rec.Code, rec.Body.String())
	}
	// Upload a minimal PDF via the files endpoint (multipart is covered in files_test;
	// here we add an external available file via JSON? No — external is not available.
	// Instead upload multipart through the handler's multipart path helper below.)
	uploadPDF(t, h, cookie, bookID)

	// Now add to library: owned + downloaded + currently_reading + favorite.
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{
		"book_id": bookID, "physical_status": "owned", "digital_status": "downloaded",
		"reading_status": "currently_reading", "is_favorite": true,
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("add my book: %d %s", rec.Code, rec.Body.String())
	}
	var ub struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ub)
	ubID := ub.Data.ID
	if ubID == "" {
		t.Fatal("user_book id empty")
	}

	// Duplicate add -> 422 (validation).
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{"book_id": bookID}, cookie)
	if rec.Code != 422 {
		t.Fatalf("duplicate add: %d %s, want 422", rec.Code, rec.Body.String())
	}

	// Invalid status -> 422.
	rec = doJSON(t, h, "PATCH", "/api/v1/me/books/"+ubID, map[string]any{"reading_status": "bogus"}, cookie)
	if rec.Code != 422 {
		t.Fatalf("invalid status: %d %s, want 422", rec.Code, rec.Body.String())
	}

	// Physical vs reading independent: change reading only, physical stays owned.
	rec = doJSON(t, h, "PATCH", "/api/v1/me/books/"+ubID, map[string]any{"reading_status": "completed"}, cookie)
	if rec.Code != 200 {
		t.Fatalf("update reading: %d %s", rec.Code, rec.Body.String())
	}
	var ub2 struct {
		Data struct {
			Physical string `json:"physical_status"`
			Reading  string `json:"reading_status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ub2)
	if ub2.Data.Physical != "owned" || ub2.Data.Reading != "completed" {
		t.Fatalf("after update: %+v", ub2.Data)
	}

	// Progress: 50/100 => 50%.
	rec = doJSON(t, h, "PUT", "/api/v1/me/books/"+ubID+"/progress", map[string]any{"current_page": 50, "total_pages": 100}, cookie)
	if rec.Code != 200 {
		t.Fatalf("progress: %d %s", rec.Code, rec.Body.String())
	}
	var prog struct {
		Data struct {
			Percent float64 `json:"progress_percent"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &prog)
	if prog.Data.Percent != 50 {
		t.Fatalf("percent = %v, want 50", prog.Data.Percent)
	}
	// current > total -> 422.
	rec = doJSON(t, h, "PUT", "/api/v1/me/books/"+ubID+"/progress", map[string]any{"current_page": 900, "total_pages": 100}, cookie)
	if rec.Code != 422 {
		t.Fatalf("progress overflow: %d %s, want 422", rec.Code, rec.Body.String())
	}

	// Session.
	rec = doJSON(t, h, "POST", "/api/v1/me/books/"+ubID+"/sessions", map[string]any{"pages_read": 10, "reading_note": "Ch 1 done"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("session: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/api/v1/me/books/"+ubID+"/sessions", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("list sessions: %d %s", rec.Code, rec.Body.String())
	}

	// Note (private by default).
	rec = doJSON(t, h, "POST", "/api/v1/me/books/"+ubID+"/notes", map[string]any{"title": "Thought", "content": "Great metaphor", "page_number": 12}, cookie)
	if rec.Code != 201 {
		t.Fatalf("note: %d %s", rec.Code, rec.Body.String())
	}
	var note struct {
		Data struct {
			Visibility string `json:"visibility"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &note)
	if note.Data.Visibility != "private" {
		t.Fatalf("visibility = %q, want private", note.Data.Visibility)
	}

	// Review: rating + reaction separate.
	rec = doJSON(t, h, "PUT", "/api/v1/me/books/"+ubID+"/review", map[string]any{"rating": 5, "reaction": "legend", "review_text": "A masterpiece"}, cookie)
	if rec.Code != 200 {
		t.Fatalf("review: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "PUT", "/api/v1/me/books/"+ubID+"/review", map[string]any{"rating": 99}, cookie)
	if rec.Code != 422 {
		t.Fatalf("bad rating: %d %s, want 422", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "PUT", "/api/v1/me/books/"+ubID+"/review", map[string]any{"reaction": "bogus"}, cookie)
	if rec.Code != 422 {
		t.Fatalf("bad reaction: %d %s, want 422", rec.Code, rec.Body.String())
	}

	// Dashboard counts.
	rec = doJSON(t, h, "GET", "/api/v1/me/dashboard", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("dashboard: %d %s", rec.Code, rec.Body.String())
	}
	var dash struct {
		Data struct {
			Total     int `json:"total_books"`
			Owned     int `json:"physical_owned"`
			Completed int `json:"completed"`
			Favorites int `json:"favorites"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &dash)
	if dash.Data.Total != 1 || dash.Data.Owned != 1 || dash.Data.Completed != 1 || dash.Data.Favorites != 1 {
		t.Fatalf("dashboard = %+v, want total/owned/completed/fav = 1", dash.Data)
	}

	// Cross-user isolation: second user cannot see first user's entry.
	rec = doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Other", "email": "other@example.com", "password": "secret123",
	}, "")
	if rec.Code != 201 {
		t.Fatalf("register other: %d %s", rec.Code, rec.Body.String())
	}
	otherCookie := cookieOf(rec)
	rec = doJSON(t, h, "GET", "/api/v1/me/books/"+ubID, nil, otherCookie)
	if rec.Code != 404 {
		t.Fatalf("cross-user get: %d %s, want 404", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/api/v1/me/books", nil, otherCookie)
	var otherList struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &otherList)
	if len(otherList.Data) != 0 {
		t.Fatalf("other user library should be empty, got %d", len(otherList.Data))
	}
}

func TestProgressCompleteSetsCompleted(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "P", "email": "p@example.com", "password": "secret123",
	}, "")
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "A"}, cookie)
	var wResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{"title": "Prog Book", "writers": []map[string]any{{"id": wResp.Data.ID}}}, cookie)
	var bResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bResp)
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{"book_id": bResp.Data.ID, "reading_status": "currently_reading"}, cookie)
	var ub struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ub)

	rec = doJSON(t, h, "PUT", "/api/v1/me/books/"+ub.Data.ID+"/progress", map[string]any{"current_page": 100, "total_pages": 100}, cookie)
	if rec.Code != 200 {
		t.Fatalf("progress 100: %d %s", rec.Code, rec.Body.String())
	}
	var prog struct {
		Data struct {
			StartedAt   string `json:"started_at"`
			CompletedAt string `json:"completed_at"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &prog)
	if prog.Data.StartedAt == "" || prog.Data.CompletedAt == "" {
		t.Fatalf("started/completed = %+v", prog.Data)
	}
	// Reading status auto-updated to completed.
	rec = doJSON(t, h, "GET", "/api/v1/me/books/"+ub.Data.ID, nil, cookie)
	var got struct {
		Data struct {
			Reading string `json:"reading_status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Data.Reading != "completed" {
		t.Fatalf("reading = %q, want completed", got.Data.Reading)
	}
}
