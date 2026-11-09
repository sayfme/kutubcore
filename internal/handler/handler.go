// Package handler contains HTTP handlers. Handlers depend on the
// service layer, never on domain internals or SQL directly
// (Blueprint §13 architectural rules).
package handler

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"kutubcore/internal/middleware"
	"kutubcore/internal/repository"
	"kutubcore/internal/service"
	"kutubcore/web"
)

// Handler holds shared dependencies for HTTP handlers.
type Handler struct {
	DB     *sql.DB
	Logger *slog.Logger

	Auth    *service.AuthService
	Catalog *service.CatalogService

	Personal    repository.PersonalRepository
	PersonalSvc *service.PersonalService

	Account *service.AccountService

	Dirs   repository.DirectoryRepository
	DirSvc *service.DirectoryService

	Acquisitions   repository.AcquisitionRepository
	AcquisitionSvc *service.AcquisitionService

	Files   repository.FileRepository
	FileSvc *service.FileService
	FileDir string

	Books   repository.BookRepository
	Writers repository.WriterRepository
	Ref     repository.ReferenceRepository

	SessionTTL   time.Duration
	CookieSecure bool
}

func (h *Handler) sessionMaxAge() int {
	if h.SessionTTL <= 0 {
		return int((30 * 24 * time.Hour) / time.Second)
	}
	return int(h.SessionTTL / time.Second)
}

// Routes builds the HTTP router using only the stdlib (no framework
// dependency in foundation). Auth/catalog wiring is in Milestone B.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.Health)
	mux.HandleFunc("GET /api/v1/health", h.Health)

	// ── auth ──────────────────────────────────────────────────────────
	limiter := middleware.NewRateLimiter(30, time.Minute)
	authLimited := func(hf http.HandlerFunc) http.Handler {
		return middleware.RateLimitAuth(limiter)(http.HandlerFunc(hf))
	}
	mux.Handle("POST /api/v1/auth/register", authLimited(h.Register))
	mux.Handle("POST /api/v1/auth/login", authLimited(h.Login))
	mux.Handle("POST /api/v1/auth/logout", http.HandlerFunc(h.Logout))
	mux.Handle("GET /api/v1/auth/me", http.HandlerFunc(h.Me))
	mux.Handle("POST /api/v1/auth/forgot-password", authLimited(h.ForgotPassword))
	mux.Handle("POST /api/v1/auth/reset-password", authLimited(h.ResetPassword))
	mux.Handle("POST /api/v1/auth/verify-email", authLimited(h.VerifyEmail))
	mux.Handle("POST /api/v1/auth/resend-verification", http.HandlerFunc(h.ResendVerification))

	// ── users ─────────────────────────────────────────────────────────
	mux.Handle("GET /api/v1/users/me", http.HandlerFunc(h.MeProfile))
	mux.Handle("GET /api/v1/users/me/roles", http.HandlerFunc(h.MeRoles))
	mux.Handle("POST /api/v1/users/me/roles/organizer", http.HandlerFunc(h.ActivateOrganizer))

	// ── catalog (public reads, organizer writes) ──────────────────────
	mux.Handle("GET /api/v1/books", http.HandlerFunc(h.ListBooks))
	mux.Handle("GET /api/v1/books/{id}", http.HandlerFunc(h.GetBook))
	mux.Handle("GET /api/v1/books/{id}/editions", http.HandlerFunc(h.ListEditions))
	mux.Handle("GET /api/v1/writers", http.HandlerFunc(h.ListWriters))
	mux.Handle("GET /api/v1/writers/{id}", http.HandlerFunc(h.GetWriter))
	mux.Handle("GET /api/v1/languages", http.HandlerFunc(h.ListLanguages))
	mux.Handle("GET /api/v1/genres", http.HandlerFunc(h.ListGenres))

	organizer := func(hf http.HandlerFunc) http.Handler {
		return middleware.RequireOrganizer(http.HandlerFunc(hf))
	}
	// RequireOrganizer runs after SessionAuth (wired in app.New / test helper).
	mux.Handle("POST /api/v1/books", organizer(h.CreateBook))
	mux.Handle("PATCH /api/v1/books/{id}", organizer(h.UpdateBook))
	mux.Handle("DELETE /api/v1/books/{id}", organizer(h.DeleteBook))
	mux.Handle("POST /api/v1/books/{id}/editions", organizer(h.CreateEdition))
	mux.Handle("POST /api/v1/writers", organizer(h.CreateWriter))
	mux.Handle("PATCH /api/v1/writers/{id}", organizer(h.UpdateWriter))
	mux.Handle("DELETE /api/v1/writers/{id}", organizer(h.DeleteWriter))

	// ── directories (bookshops + sources) ───────────────────────────────
	mux.Handle("GET /api/v1/bookshops", http.HandlerFunc(h.ListBookshops))
	mux.Handle("GET /api/v1/bookshops/{id}", http.HandlerFunc(h.GetBookshop))
	mux.Handle("POST /api/v1/bookshops", organizer(h.CreateBookshop))
	mux.Handle("PATCH /api/v1/bookshops/{id}", organizer(h.UpdateBookshop))
	mux.Handle("DELETE /api/v1/bookshops/{id}", organizer(h.DeleteBookshop))
	mux.Handle("POST /api/v1/bookshops/{id}/links", organizer(h.AddBookshopLink))
	mux.Handle("DELETE /api/v1/bookshops/{id}/links/{linkId}", organizer(h.DeleteBookshopLink))
	mux.Handle("GET /api/v1/sources", http.HandlerFunc(h.ListSources))
	mux.Handle("GET /api/v1/sources/{id}", http.HandlerFunc(h.GetSource))
	mux.Handle("POST /api/v1/sources", organizer(h.CreateSource))
	mux.Handle("PATCH /api/v1/sources/{id}", organizer(h.UpdateSource))
	mux.Handle("DELETE /api/v1/sources/{id}", organizer(h.DeleteSource))
	mux.Handle("POST /api/v1/sources/{id}/links", organizer(h.AddSourceLink))
	mux.Handle("DELETE /api/v1/sources/{id}/links/{linkId}", organizer(h.DeleteSourceLink))

	// ── files (public metadata, organizer upload, authorized download) ───
	mux.Handle("GET /api/v1/books/{id}/files", http.HandlerFunc(h.ListFiles))
	mux.Handle("GET /api/v1/files/{id}/download", http.HandlerFunc(h.DownloadFile))
	mux.Handle("POST /api/v1/books/{id}/files", organizer(h.UploadFile))
	mux.Handle("POST /api/v1/books/{id}/cover", organizer(h.UploadCover))

	// ── personal library (authenticated, owner-checked) ────────────────
	authenticated := func(hf http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(hf))
	}
	mux.Handle("GET /api/v1/me/dashboard", authenticated(h.Dashboard))
	mux.Handle("GET /api/v1/me/acquisitions", authenticated(h.ListAcquisitions))
	mux.Handle("POST /api/v1/me/acquisitions", authenticated(h.CreateAcquisition))
	mux.Handle("GET /api/v1/me/acquisitions/{id}", authenticated(h.GetAcquisition))
	mux.Handle("DELETE /api/v1/me/acquisitions/{id}", authenticated(h.DeleteAcquisition))
	mux.Handle("GET /api/v1/me/copies", authenticated(h.ListCopies))
	mux.Handle("GET /api/v1/me/books", authenticated(h.ListMyBooks))
	mux.Handle("POST /api/v1/me/books", authenticated(h.AddMyBook))
	mux.Handle("GET /api/v1/me/books/{id}", authenticated(h.GetMyBook))
	mux.Handle("PATCH /api/v1/me/books/{id}", authenticated(h.UpdateMyBook))
	mux.Handle("DELETE /api/v1/me/books/{id}", authenticated(h.DeleteMyBook))
	mux.Handle("GET /api/v1/me/books/{id}/progress", authenticated(h.GetProgress))
	mux.Handle("PUT /api/v1/me/books/{id}/progress", authenticated(h.UpdateProgress))
	mux.Handle("GET /api/v1/me/books/{id}/sessions", authenticated(h.ListSessions))
	mux.Handle("POST /api/v1/me/books/{id}/sessions", authenticated(h.CreateSession))
	mux.Handle("GET /api/v1/me/books/{id}/notes", authenticated(h.ListNotes))
	mux.Handle("POST /api/v1/me/books/{id}/notes", authenticated(h.CreateNote))
	mux.Handle("PATCH /api/v1/me/books/{id}/notes/{noteId}", authenticated(h.UpdateNote))
	mux.Handle("DELETE /api/v1/me/books/{id}/notes/{noteId}", authenticated(h.DeleteNote))
	mux.Handle("GET /api/v1/me/books/{id}/review", authenticated(h.GetReview))
	mux.Handle("PUT /api/v1/me/books/{id}/review", authenticated(h.UpsertReview))
	mux.Handle("DELETE /api/v1/me/books/{id}/review", authenticated(h.DeleteReview))

	// Serve the PWA shell + static assets (embedded, CWD-independent).
	// /sw.js is served at root with Service-Worker-Allowed so the worker
	// has root scope (a /static/ worker would be confined to /static/).
	mux.Handle("GET /sw.js", http.HandlerFunc(h.ServiceWorker))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS()))))
	mux.HandleFunc("GET /", h.Shell)

	return mux
}

// staticFS returns the embedded static file system rooted at static/.
func staticFS() fs.FS {
	sub, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		return web.StaticFS
	}
	return sub
}

// ServiceWorker serves the service worker at /sw.js with root scope,
// no-cache (updates immediately), and a JS content type.
func (h *Handler) ServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Service-Worker-Allowed", "/")
	w.Header().Set("Cache-Control", "no-cache")
	data, err := fs.ReadFile(web.StaticFS, "static/sw.js")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Service worker unavailable.")
		return
	}
	_, _ = w.Write(data)
}

// Health reports process + database liveness.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if h.DB != nil {
		if err := h.DB.PingContext(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database is unavailable.")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}, nil)
}

// Shell serves the PWA index.
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
  <header>
    <h1>KutubCore</h1>
    <p id="who">Loading…</p>
    <p id="online-status" role="status">Online</p>
  </header>

  <section>
    <h2>Sign in</h2>
    <form id="auth-form">
      <label>Display name <input name="display_name" autocomplete="name"></label>
      <label>Email or identifier <input name="identifier" autocomplete="username"></label>
      <input type="hidden" name="email">
      <input type="hidden" name="mobile">
      <label>Password <input type="password" name="password" required autocomplete="current-password"></label>
      <button type="submit" data-mode="login">Sign in</button>
      <button type="button" id="register-btn">Register</button>
    </form>
    <p><button type="button" id="organizer-btn" hidden>Become Organizer</button>
       <button type="button" id="logout-btn" hidden>Sign out</button></p>
    <p class="err" id="auth-err"></p>
  </section>

  <section>
    <h2>Search books</h2>
    <form id="search-form">
      <input name="q" placeholder="Title, original title, writer…">
      <button type="submit">Search</button>
    </form>
    <ul id="books"></ul>
  </section>

  <section id="library-section">
    <h2>My Collection (Reader)</h2>
    <form id="add-book-form">
      <label>Book ID <input name="book_id" required placeholder="catalog book id"></label>
      <label>Physical
        <select name="physical_status">
          <option value="not_collected">not_collected</option>
          <option value="want_to_buy">want_to_buy</option>
          <option value="owned">owned</option>
        </select></label>
      <label>Reading
        <select name="reading_status">
          <option value="want_to_read">want_to_read</option>
          <option value="currently_reading">currently_reading</option>
          <option value="on_hold">on_hold</option>
          <option value="completed">completed</option>
          <option value="abandoned">abandoned</option>
        </select></label>
      <label><input type="checkbox" name="is_favorite"> Favorite</label>
      <button type="submit">Add to my library</button>
    </form>
    <p class="err" id="add-book-err"></p>
    <p id="dash"></p>
    <ul id="my-books"></ul>
    <h3>Record acquisition (creates copies)</h3>
    <form id="acq-form">
      <label>Book ID <input name="book_id" required></label>
      <label>Bookshop ID <input name="bookshop_id" placeholder="optional"></label>
      <label>Date <input name="acquisition_date" type="date"></label>
      <label>Total cost <input name="total_cost" type="number" min="0" step="0.01" placeholder="optional"></label>
      <label>Currency <input name="currency_code" maxlength="3" placeholder="USD"></label>
      <button type="submit">Record acquisition</button>
    </form>
    <p class="err" id="acq-err"></p>
  </section>

  <section id="directories-section">
    <h2>Bookshops &amp; Sources (Organizer)</h2>
    <form id="shop-form">
      <label>Name <input name="name" required></label>
      <label>Type <select name="shop_type">
        <option value="both">both</option>
        <option value="physical">physical</option>
        <option value="online">online</option>
      </select></label>
      <button type="submit">Add bookshop</button>
    </form>
    <p class="err" id="shop-err"></p>
    <ul id="shops"></ul>
    <form id="source-form">
      <label>Name <input name="name" required></label>
      <label>Type <select name="source_type">
        <option value="other">other</option>
        <option value="bookshop">bookshop</option>
        <option value="library">library</option>
        <option value="gift">gift</option>
        <option value="friend">friend</option>
        <option value="publisher">publisher</option>
        <option value="online_repository">online_repository</option>
      </select></label>
      <button type="submit">Add source</button>
    </form>
    <p class="err" id="source-err"></p>
    <ul id="sources"></ul>
  </section>

  <section id="files-section">
    <h2>Digital Files (Organizer)</h2>
    <form id="upload-form">
      <label>Book ID <input name="book_id" required></label>
      <label>PDF/EPUB file <input name="file" type="file" accept=".pdf,.epub" required></label>
      <button type="submit">Upload file</button>
    </form>
    <p class="err" id="upload-err"></p>
  </section>

  <section id="create-section">
    <h2>Catalog (Organizer)</h2>
    <form id="writer-form">
      <h3>New writer</h3>
      <label>Full name <input name="full_name" required></label>
      <button type="submit">Add writer</button>
    </form>
    <p class="err" id="writer-err"></p>
    <form id="book-form">
      <h3>New book</h3>
      <label>Title <input name="title" required></label>
      <label>Writer ID <input name="writer_id" placeholder="optional"></label>
      <button type="submit">Add book</button>
    </form>
    <p class="err" id="book-err"></p>
    <details><summary>API health</summary><p><a href="/api/v1/health">GET /api/v1/health</a></p></details>
  </section>
</main>
<script src="/static/js/app.js"></script>
</body>
</html>`
