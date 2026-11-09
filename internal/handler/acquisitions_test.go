package handler_test

import (
	"encoding/json"
	"testing"

	"kutubcore/internal/testutil"
)

func TestAcquisitionFlow_CopiesAndOwnedRule(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "AC", "email": "acq@example.com", "password": "secret123",
	}, "")
	cookie := cookieOf(rec)
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	rec = doJSON(t, h, "POST", "/api/v1/writers", map[string]any{"full_name": "Acq Author"}, cookie)
	var wResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wResp)
	rec = doJSON(t, h, "POST", "/api/v1/books", map[string]any{"title": "Acq Book", "writers": []map[string]any{{"id": wResp.Data.ID}}}, cookie)
	var bResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bResp)
	bookID := bResp.Data.ID

	// Bookshop + source for acquisition refs.
	rec = doJSON(t, h, "POST", "/api/v1/bookshops", map[string]any{"name": "Shop 1", "shop_type": "physical"}, cookie)
	var shop struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &shop)
	rec = doJSON(t, h, "POST", "/api/v1/sources", map[string]any{"name": "Gift", "source_type": "gift"}, cookie)
	var src struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &src)

	// Acquisition with quantity 2 -> 2 copies.
	rec = doJSON(t, h, "POST", "/api/v1/me/acquisitions", map[string]any{
		"bookshop_id": shop.Data.ID, "source_id": src.Data.ID,
		"acquisition_date": "2026-01-15", "total_cost": 25.5, "currency_code": "USD",
		"items": []map[string]any{{"book_id": bookID, "format": "paperback", "quantity": 2, "unit_price": 12.75}},
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("acquisition: %d %s", rec.Code, rec.Body.String())
	}
	var acq struct {
		Data struct {
			Items []struct {
				Copies []struct {
					ID string `json:"id"`
				} `json:"copies"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &acq)
	if len(acq.Data.Items) != 1 || len(acq.Data.Items[0].Copies) != 2 {
		t.Fatalf("copies = %+v, want 2", acq.Data.Items)
	}

	// Copies inventory endpoint.
	rec = doJSON(t, h, "GET", "/api/v1/me/copies?book_id="+bookID, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("copies: %d %s", rec.Code, rec.Body.String())
	}
	var copies struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &copies)
	if len(copies.Data) != 2 {
		t.Fatalf("copies count = %d, want 2", len(copies.Data))
	}

	// Second user sees no copies.
	rec = doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Other2", "email": "other2@example.com", "password": "secret123",
	}, "")
	other := cookieOf(rec)
	rec = doJSON(t, h, "GET", "/api/v1/me/copies", nil, other)
	var otherCopies struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &otherCopies)
	if len(otherCopies.Data) != 0 {
		t.Fatalf("other copies = %d, want 0", len(otherCopies.Data))
	}

	// Owned now allowed (copies exist).
	rec = doJSON(t, h, "POST", "/api/v1/me/books", map[string]any{"book_id": bookID, "physical_status": "owned", "reading_status": "currently_reading"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("add owned: %d %s", rec.Code, rec.Body.String())
	}
}
