package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"kutubcore/internal/testutil"
)

func TestFilesFlow_UploadDownloadExternal(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "F", "email": "f@example.com", "password": "secret123",
	}, "")
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "File Author"}, cookie)
	var wResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{"title": "File Book", "writers": []map[string]any{{"id": wResp.Data.ID}}}, cookie)
	var bResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bResp)
	bookID := bResp.Data.ID

	// Upload PDF (via shared helper) -> 201 with checksum + size.
	// We re-upload here to capture the file ID.
	uploadPDF(t, h, cookie, bookID)

	// List files (public metadata).
	rec = doJSON(t, h, "GET", "/api/v1/books/"+bookID+"/files", nil, "")
	if rec.Code != 200 {
		t.Fatalf("list files: %d %s", rec.Code, rec.Body.String())
	}
	var files struct {
		Data []struct {
			ID       string `json:"id"`
			Checksum string `json:"checksum_sha256"`
			Size     int64  `json:"file_size"`
			Format   string `json:"file_format"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &files)
	if len(files.Data) != 1 || files.Data[0].Checksum == "" || files.Data[0].Format != "pdf" {
		t.Fatalf("files = %+v", files.Data)
	}
	fileID := files.Data[0].ID

	// Download returns the bytes with attachment disposition.
	rec = doJSON(t, h, "GET", "/api/v1/files/"+fileID+"/download", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("download: %d %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "%PDF-1.4 minimal" {
		t.Fatalf("download body = %q", body)
	}

	// External URL records IsAvailable=false (rule 10: URL != downloaded).
	rec = doJSON(t, h, "POST", "/api/v1/books/"+bookID+"/files", map[string]any{
		"external_url": "https://example.com/book.pdf", "format": "pdf",
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("external file: %d %s", rec.Code, rec.Body.String())
	}
	var ext struct {
		Data struct {
			Locations []struct {
				ExternalURL string `json:"external_url"`
				IsAvailable bool   `json:"is_available"`
			} `json:"locations"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ext)
	if len(ext.Data.Locations) != 1 || ext.Data.Locations[0].IsAvailable != false {
		t.Fatalf("external loc = %+v, want IsAvailable=false", ext.Data.Locations)
	}

	// Unsupported format rejected (via service unit path through uploadPDF is fixed
	// to pdf, so exercise JSON external with bad format).
	rec = doJSON(t, h, "POST", "/api/v1/books/"+bookID+"/files", map[string]any{
		"external_url": "https://example.com/x.exe", "format": "exe",
	}, cookie)
	if rec.Code != 422 {
		t.Fatalf("bad format: %d %s, want 422", rec.Code, rec.Body.String())
	}
}
