package handler_test

import (
	"encoding/json"
	"testing"

	"kutubcore/internal/testutil"
)

func TestDirectoryFlow_BookshopSourceLinks(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Do", "email": "do@example.com", "password": "secret123",
	}, "")
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	if rec.Code != 200 {
		t.Fatalf("organizer: %d %s", rec.Code, rec.Body.String())
	}

	// Guest (non-organizer) cannot create.
	rec = doJSON(t, h, "POST", "/api/v1/bookshops", map[string]any{"name": "X"}, "")
	if rec.Code != 401 && rec.Code != 403 {
		t.Fatalf("guest create bookshop: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, "POST", "/api/v1/bookshops", map[string]any{
		"name": "Dhaka Book House", "shop_type": "both", "city": "Dhaka", "country": "BD",
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("create bookshop: %d %s", rec.Code, rec.Body.String())
	}
	var shop struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &shop)

	rec = doJSON(t, h, "POST", "/api/v1/bookshops/"+shop.Data.ID+"/links", map[string]any{
		"label": "Website", "url": "https://example.com/shop",
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("add link: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/api/v1/bookshops/"+shop.Data.ID, nil, "")
	if rec.Code != 200 {
		t.Fatalf("get bookshop: %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Data struct {
			Links []struct {
				URL string `json:"url"`
			} `json:"links"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Data.Links) != 1 || got.Data.Links[0].URL != "https://example.com/shop" {
		t.Fatalf("links = %+v", got.Data.Links)
	}

	rec = doJSON(t, h, "POST", "/api/v1/sources", map[string]any{
		"name": "University Library", "source_type": "library",
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("create source: %d %s", rec.Code, rec.Body.String())
	}
	var src struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &src)
	rec = doJSON(t, h, "POST", "/api/v1/sources/"+src.Data.ID+"/links", map[string]any{
		"label": "Catalog", "url": "https://example.com/catalog",
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("add source link: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/api/v1/sources?q=University", nil, "")
	if rec.Code != 200 {
		t.Fatalf("list sources: %d %s", rec.Code, rec.Body.String())
	}
}
