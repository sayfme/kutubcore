// Package handler contains HTTP handlers. Handlers depend on the
// service layer, never on domain internals or SQL directly
// (Blueprint §13 architectural rules).
package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Handler holds shared dependencies for HTTP handlers.
type Handler struct {
	DB     *sql.DB
	Logger *slog.Logger
}

// Routes builds the HTTP router using only the stdlib (no framework
// dependency in foundation). Add /api/v1 routes here as milestones land.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.Health)
	mux.HandleFunc("GET /api/v1/health", h.Health)

	// Serve the PWA shell + static assets.
	// Templates are served from web/templates; static from web/static.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("GET /", h.Shell)

	return mux
}

// Health reports process + database liveness (Blueprint §18:
// "no critical data loss during shutdown/restart" — used by scripts).
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if h.DB != nil {
		if err := h.DB.PingContext(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"success": false,
				"error":   map[string]string{"code": "DB_UNAVAILABLE", "message": "Database is unavailable."},
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"status":    "ok",
			"time":      time.Now().UTC().Format(time.RFC3339),
			"timestamp": time.Now().UTC().Unix(),
		},
		"meta": nil,
	})
}

// Shell serves the PWA index. Kept intentionally tiny in foundation;
// the real UI arrives in Milestone 2+.
func (h *Handler) Shell(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(shellHTML))
}

const shellHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#1f2a44">
<title>KutubCore</title>
<link rel="manifest" href="/static/manifest.webmanifest">
<link rel="stylesheet" href="/static/css/app.css">
</head>
<body>
<main class="shell">
  <h1>KutubCore</h1>
  <p>Digital Book Collection Management System</p>
  <p>Foundation (Milestone 1) is running. Catalog comes in Milestone 2.</p>
  <p><a href="/api/v1/health">API health</a></p>
</main>
<script src="/static/js/app.js"></script>
</body>
</html>`

// writeJSON writes a Blueprint §9 envelope: {success, data, meta} / {success, error{code,message}}.
func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
