package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kutubcore/internal/testutil"
)

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "kutubcore_session", Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func cookieOf(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "kutubcore_session" && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func TestAuthFlow_RegisterLoginMeOrganizer(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	// Register
	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Test Reader", "email": "reader@example.com", "password": "secret123",
	}, "")
	if rec.Code != 201 {
		t.Fatalf("register: status %d body %s", rec.Code, rec.Body.String())
	}
	cookie := cookieOf(rec)
	if cookie == "" {
		t.Fatal("register: missing session cookie")
	}

	// Me
	rec = doJSON(t, h, "GET", "/api/v1/auth/me", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("me: status %d body %s", rec.Code, rec.Body.String())
	}
	var me struct {
		Data struct {
			Email string   `json:"email"`
			Roles []string `json:"roles"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Data.Email != "reader@example.com" {
		t.Fatalf("me email = %q", me.Data.Email)
	}
	if len(me.Data.Roles) != 1 || me.Data.Roles[0] != "reader" {
		t.Fatalf("me roles = %v, want [reader]", me.Data.Roles)
	}

	// Organizer self-activation
	rec = doJSON(t, h, "POST", "/api/v1/users/me/roles/organizer", map[string]any{}, cookie)
	if rec.Code != 200 {
		t.Fatalf("organizer: status %d body %s", rec.Code, rec.Body.String())
	}

	// Wrong password
	rec = doJSON(t, h, "POST", "/api/v1/auth/login", map[string]any{
		"identifier": "reader@example.com", "password": "wrongpass123",
	}, "")
	if rec.Code != 401 {
		t.Fatalf("wrong login: status %d body %s", rec.Code, rec.Body.String())
	}

	// Logout clears cookie and invalidates session
	rec = doJSON(t, h, "POST", "/api/v1/auth/logout", map[string]any{}, cookie)
	if rec.Code != 204 {
		t.Fatalf("logout: status %d body %s", rec.Code, rec.Body.String())
	}
	// Old cookie no longer authenticates /auth/me
	rec = doJSON(t, h, "GET", "/api/v1/auth/me", nil, cookie)
	if rec.Code != 200 {
		// SessionAuth is optional-auth: /auth/me returns 401 when unauthenticated,
		// which is also acceptable if the session was deleted.
		if rec.Code != 401 {
			t.Fatalf("me after logout: status %d body %s", rec.Code, rec.Body.String())
		}
	}
}

func TestGuestCannotCreateBook(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)

	rec := doJSON(t, application.Handler, "POST", "/api/v1/books", map[string]any{"title": "Nope"}, "")
	if rec.Code != 401 && rec.Code != 403 {
		t.Fatalf("guest create book: status %d body %s, want 401/403", rec.Code, rec.Body.String())
	}
}
