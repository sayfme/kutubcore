package handler_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"kutubcore/internal/testutil"
)

// Goals: set then read current month, validation, done-count.
func TestAnalyticsFlow_GoalsAndStats(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "An", "email": "an@example.com", "password": "secret123",
	}, "")
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "Analytics Author"}, cookie)
	var wResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{"title": "Analytics Book", "writers": []map[string]any{{"id": wResp.Data.ID}}}, cookie)
	var bResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bResp)
	bookID := bResp.Data.ID

	// Acquisition (copy) then own it.
	rec = doJSON(t, h, "POST", "/api/v1/me/acquisitions", map[string]any{"items": []map[string]any{{"book_id": bookID, "format": "paperback"}}}, cookie)
	if rec.Code != 201 {
		t.Fatalf("acquisition: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{"book_id": bookID, "physical_status": "owned", "reading_status": "currently_reading"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("add library: %d %s", rec.Code, rec.Body.String())
	}
	var ub struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ub)
	ubID := ub.Data.ID

	// Two sessions with pages (one today).
	for _, pages := range []int{30, 20} {
		rec = doJSON(t, h, "POST", fmt.Sprintf("/api/v1/me/books/%s/sessions", ubID), map[string]any{"pages_read": pages}, cookie)
		if rec.Code != 201 {
			t.Fatalf("session: %d %s", rec.Code, rec.Body.String())
		}
	}

	// Analytics: pages summed into current month bucket.
	rec = doJSON(t, h, "GET", "/api/v1/me/analytics?months=2", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("analytics: %d %s", rec.Code, rec.Body.String())
	}
	var sum struct {
		Data struct {
			TotalPages int `json:"total_pages"`
			TotalSess  int `json:"total_sessions"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sum)
	if sum.Data.TotalPages != 50 || sum.Data.TotalSess != 2 {
		t.Fatalf("analytics totals = %+v, want 50/2", sum.Data)
	}

	// Goal set + read + validation.
	rec = doJSON(t, h, "PUT", "/api/v1/me/goals", map[string]any{"books_target": 3}, cookie)
	if rec.Code != 200 {
		t.Fatalf("set goal: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "PUT", "/api/v1/me/goals", map[string]any{"books_target": 0}, cookie)
	if rec.Code != 422 {
		t.Fatalf("bad target: %d %s, want 422", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "PUT", "/api/v1/me/goals", map[string]any{"period_month": "bogus", "books_target": 2}, cookie)
	if rec.Code != 422 {
		t.Fatalf("bad month: %d %s, want 422", rec.Code, rec.Body.String())
	}
	curr := time.Now().UTC().Format("2006-01")
	rec = doJSON(t, h, "GET", "/api/v1/me/goals?month="+curr, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("get goal: %d %s", rec.Code, rec.Body.String())
	}
	var goal struct {
		Data struct {
			BooksTarget int    `json:"books_target"`
			PeriodMonth string `json:"period_month"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &goal)
	if goal.Data.BooksTarget != 3 || goal.Data.PeriodMonth != curr {
		t.Fatalf("goal = %+v", goal.Data)
	}

	// Complete the book -> books_done increments.
	rec = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/me/books/%s/progress", ubID), map[string]any{"current_page": 100, "total_pages": 100}, cookie)
	if rec.Code != 200 {
		t.Fatalf("progress: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/api/v1/me/goals?month="+curr, nil, cookie)
	var goal2 struct {
		Data struct {
			BooksDone int `json:"books_done"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &goal2)
	if goal2.Data.BooksDone != 1 {
		t.Fatalf("books_done = %d, want 1", goal2.Data.BooksDone)
	}

	// Backdate a session to a past month so analytics covers multiple buckets.
	_, err := db.Exec(`UPDATE reading_sessions SET started_at = date('now','-1 month') WHERE user_book_id=? AND id=(SELECT id FROM reading_sessions WHERE user_book_id=? ORDER BY started_at LIMIT 1)`, ubID, ubID)
	if err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, h, "GET", "/api/v1/me/analytics?months=3", nil, cookie)
	var sum2 struct {
		Data struct {
			Months []struct {
				Month    string `json:"month"`
				Sessions int    `json:"sessions"`
			} `json:"months"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sum2)
	if len(sum2.Data.Months) != 3 {
		t.Fatalf("months = %d, want 3", len(sum2.Data.Months))
	}
}

// Bookshelf Life: fresh book, dust/bookworm by backdating, completed badge, disable.
func TestAnalyticsFlow_ShelfStagesAndSettings(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Sh", "email": "sh@example.com", "password": "secret123",
	}, "")
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "Shelf Author"}, cookie)
	var wResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{"title": "Shelf Book", "writers": []map[string]any{{"id": wResp.Data.ID}}}, cookie)
	var bResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bResp)

	rec = doJSON(t, h, "POST", "/api/v1/me/acquisitions", map[string]any{"items": []map[string]any{{"book_id": bResp.Data.ID, "format": "paperback"}}}, cookie)
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{"book_id": bResp.Data.ID, "physical_status": "owned", "reading_status": "currently_reading"}, cookie)
	var ub struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ub)
	ubID := ub.Data.ID

	// Fresh: last activity is now -> "fresh".
	rec = doJSON(t, h, "GET", "/api/v1/me/shelf", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("shelf: %d %s", rec.Code, rec.Body.String())
	}
	var sh struct {
		Data struct {
			Entries []struct {
				Stage        string `json:"stage"`
				DaysInactive int    `json:"days_inactive"`
			} `json:"entries"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sh)
	if len(sh.Data.Entries) != 1 || sh.Data.Entries[0].Stage != "fresh" {
		t.Fatalf("shelf fresh = %+v", sh.Data.Entries)
	}

	// Backdate activity 200 days: no recent session exists for this book
	// except the one we add at -200 days, and updated_at must move back too
	// (last_activity = MAX(sessions, updated_at)).
	if _, err := db.Exec(`UPDATE user_books SET updated_at=datetime('now','-200 days') WHERE id=?`, ubID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO reading_sessions(id, user_book_id, started_at, pages_read) VALUES(?, ?, datetime('now','-200 days'), 0)`, "old-session", ubID); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, h, "GET", "/api/v1/me/shelf", nil, cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &sh)
	if sh.Data.Entries[0].Stage != "heavy" && sh.Data.Entries[0].Stage != "bookworm" {
		t.Fatalf("shelf after 200d = %q, want bookworm/heavy", sh.Data.Entries[0].Stage)
	}

	// Completed -> achievement.
	rec = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/me/books/%s/progress", ubID), map[string]any{"current_page": 10, "total_pages": 10}, cookie)
	if rec.Code != 200 {
		t.Fatalf("progress: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/api/v1/me/shelf", nil, cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &sh)
	if !hasStage(sh, "completed") {
		t.Fatalf("completed shelf = %+v", sh.Data.Entries)
	}

	// Settings: invalid thresholds rejected.
	rec = doJSON(t, h, "PUT", "/api/v1/me/shelf/settings", map[string]any{"dust_days": 60, "cobweb_days": 30, "worm_days": 90}, cookie)
	if rec.Code != 422 {
		t.Fatalf("bad thresholds: %d %s, want 422", rec.Code, rec.Body.String())
	}
	// Disable -> fresh again.
	rec = doJSON(t, h, "PUT", "/api/v1/me/shelf/settings", map[string]any{"enabled": false, "dust_days": 5, "cobweb_days": 10, "worm_days": 20}, cookie)
	if rec.Code != 200 {
		t.Fatalf("update settings: %d %s", rec.Code, rec.Body.String())
	}
	// A second active book with old activity must be fresh while disabled:
	var b2 struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{"title": "Second Shelf Book", "writers": []map[string]any{{"id": wResp.Data.ID}}}, cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &b2)
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{"book_id": b2.Data.ID, "reading_status": "currently_reading"}, cookie)
	var ub2 struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if rec.Code != 201 {
		t.Fatalf("add second: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ub2)
	if _, err := db.Exec(`UPDATE user_books SET updated_at=datetime('now','-100 days') WHERE id=?`, ub2.Data.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO reading_sessions(id, user_book_id, started_at, pages_read) VALUES(?, ?, datetime('now','-100 days'), 0)`, "old2", ub2.Data.ID); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, h, "GET", "/api/v1/me/shelf", nil, cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &sh)
	// The 100-day book should be fresh while disabled.
	found := false
	for _, e := range sh.Data.Entries {
		if e.Stage == "fresh" {
			found = true
		}
	}
	if !found {
		t.Fatalf("disabled shelf should show fresh objects: %+v", sh.Data.Entries)
	}
}

func hasStage(sh struct {
	Data struct {
		Entries []struct {
			Stage        string `json:"stage"`
			DaysInactive int    `json:"days_inactive"`
		} `json:"entries"`
	} `json:"data"`
}, want string) bool {
	for _, e := range sh.Data.Entries {
		if e.Stage == want {
			return true
		}
	}
	return false
}
