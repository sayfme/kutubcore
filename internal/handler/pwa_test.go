package handler_test

import (
	"strings"
	"testing"

	"kutubcore/internal/testutil"
)

// PWA: /sw.js at root with root-scope header; shell registers the worker.
func TestPWA_ServiceWorker(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "GET", "/sw.js", nil, "")
	if rec.Code != 200 {
		t.Fatalf("GET /sw.js: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Service-Worker-Allowed") != "/" {
		t.Fatalf("Service-Worker-Allowed = %q, want /", rec.Header().Get("Service-Worker-Allowed"))
	}
	if rec.Header().Get("Content-Type") != "application/javascript" {
		t.Fatalf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "kutubcore-v1") {
		t.Fatal("sw.js should declare a cache version")
	}
	if strings.Contains(body, "/me/") && !strings.Contains(body, "isSensitive") {
		t.Fatal("sw.js must guard personal paths")
	}

	// Shell points at manifest + registers worker via app.js.
	rec = doJSON(t, h, "GET", "/", nil, "")
	if rec.Code != 200 {
		t.Fatalf("GET /: %d", rec.Code)
	}
	html := rec.Body.String()
	if !strings.Contains(html, "manifest.webmanifest") || !strings.Contains(html, "online-status") {
		t.Fatal("shell should link manifest + have online status")
	}
	rec = doJSON(t, h, "GET", "/static/sw.js", nil, "")
	if rec.Code != 200 {
		t.Fatalf("GET /static/sw.js: %d", rec.Code)
	}
	rec = doJSON(t, h, "GET", "/static/js/app.js", nil, "")
	if rec.Code != 200 {
		t.Fatalf("GET /static/js/app.js: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "serviceWorker") {
		t.Fatal("app.js should register the service worker")
	}
}
