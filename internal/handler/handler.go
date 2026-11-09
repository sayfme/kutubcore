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

	Analytics    repository.AnalyticsRepository
	AnalyticsSvc *service.AnalyticsService

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
	mux.Handle("GET /api/v1/me/analytics", authenticated(h.GetAnalytics))
	mux.Handle("GET /api/v1/me/goals", authenticated(h.GetGoal))
	mux.Handle("GET /api/v1/me/goals/all", authenticated(h.ListGoals))
	mux.Handle("PUT /api/v1/me/goals", authenticated(h.SetGoal))
	mux.Handle("GET /api/v1/me/shelf", authenticated(h.Shelf))
	mux.Handle("PUT /api/v1/me/shelf/settings", authenticated(h.UpdateShelfSettings))
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
<title>KutubCore — Your Library, Beautifully Shelved</title>
<meta name="description" content="KutubCore — an offline-first library for your books.">
<link rel="manifest" href="/static/manifest.webmanifest">
<link rel="stylesheet" href="/static/css/app.css">
</head>
<body>
<div class="hero">
  <div class="hero-head">
    <div class="logo-wrap"><img src="/static/icons/logo.svg" alt="" aria-hidden="true"></div>
    <div class="brand">
      <h1>KutubCore</h1>
      <p class="tagline">Your library, beautifully shelved — offline-first.</p>
    </div>
    <div class="hero-meta">
      <span class="pill" id="pill-online"><span class="dot" aria-hidden="true"></span> <span id="online-status" role="status">Online</span></span>
      <span class="pill" id="pill-offline"><span class="dot" aria-hidden="true"></span> Offline — cached catalog</span>
    </div>
  </div>
  <div class="spine-strip" aria-hidden="true"></div>
</div>

<nav class="nav" aria-label="Sections">
  <div class="nav-inner">
    <a href="#catalog"><img class="ic" src="/static/icons/book.svg" alt="">Catalog</a>
    <a href="#library"><img class="ic" src="/static/icons/library.svg" alt="">My Collection</a>
    <a href="#analytics"><img class="ic" src="/static/icons/target.svg" alt="">Reading</a>
    <a href="#directories" id="nav-directories"><img class="ic" src="/static/icons/store.svg" alt="">Bookshops</a>
    <a href="#signin"><img class="ic" src="/static/icons/signin.svg" alt="">Sign in</a>
    <span class="who"><span class="avatar" id="avatar" aria-hidden="true">?</span> <span id="who">Loading…</span></span>
  </div>
</nav>

<main class="shell">
  <section class="card" id="signin" aria-labelledby="signin-h">
    <h2 id="signin-h"><img class="ic" src="/static/icons/signin.svg" alt="">Sign in</h2>
    <form id="auth-form">
      <label>Display name <input name="display_name" autocomplete="name" placeholder="Your name (for registering)"></label>
      <label>Email or identifier <input name="identifier" autocomplete="username" placeholder="you@example.com or mobile"></label>
      <input type="hidden" name="email">
      <input type="hidden" name="mobile">
      <label>Password <input type="password" name="password" required autocomplete="current-password" placeholder="Minimum 8 characters"></label>
      <button type="submit"><img class="ic" src="/static/icons/signin.svg" alt="">Sign in</button>
      <button type="button" id="register-btn" class="btn-ghost"><img class="ic" src="/static/icons/plus.svg" alt="">Register</button>
    </form>
    <p><button type="button" id="organizer-btn" hidden><img class="ic" src="/static/icons/star.svg" alt="">Become an Organizer</button>
       <button type="button" id="logout-btn" hidden>Sign out</button></p>
    <p class="err" id="auth-err" role="alert"></p>
    <p style="color:var(--muted);font-size:.9rem;margin:.2rem 0 0">Browsing as a guest is fine — sign in to keep your own shelves, notes, and goals.</p>
  </section>

  <section class="card" id="catalog" aria-labelledby="catalog-h">
    <h2 id="catalog-h"><img class="ic" src="/static/icons/search.svg" alt="">Catalog</h2>
    <form id="search-form" role="search">
      <label>Search <input name="q" placeholder="Title, original title, writer…" autocomplete="off"></label>
      <button type="submit"><img class="ic" src="/static/icons/search.svg" alt="">Search</button>
    </form>
    <ul class="books" id="books"></ul>
  </section>

  <section class="card" id="library" aria-labelledby="library-h">
    <h2 id="library-h"><img class="ic" src="/static/icons/library.svg" alt="">My Collection</h2>
    <form id="add-book-form">
      <label>Catalog book ID <input name="book_id" required placeholder="Paste a book ID from the catalog"></label>
      <div class="grid-2">
        <label>Physical
          <select name="physical_status">
            <option value="not_collected">Not collected</option>
            <option value="want_to_buy">Want to buy</option>
            <option value="owned">Owned</option>
          </select></label>
        <label>Reading
          <select name="reading_status">
            <option value="want_to_read">Want to read</option>
            <option value="currently_reading">Currently reading</option>
            <option value="on_hold">On hold</option>
            <option value="completed">Completed</option>
            <option value="abandoned">Abandoned</option>
          </select></label>
      </div>
      <label><input type="checkbox" name="is_favorite"> Favorite</label>
      <button type="submit"><img class="ic" src="/static/icons/plus.svg" alt="">Add to my library</button>
    </form>
    <p class="err" id="add-book-err" role="alert"></p>
    <p id="dash"></p>
    <ul class="rows" id="my-books"></ul>
    <h3><img class="ic" src="/static/icons/store.svg" alt="">Record acquisition (creates copies)</h3>
    <form id="acq-form">
      <div class="grid-2">
        <label>Book ID <input name="book_id" required></label>
        <label>Bookshop ID <input name="bookshop_id" placeholder="Optional"></label>
        <label>Date <input name="acquisition_date" type="date"></label>
        <label>Total cost <input name="total_cost" type="number" min="0" step="0.01" placeholder="Optional"></label>
      </div>
      <label>Currency <input name="currency_code" maxlength="3" placeholder="USD"></label>
      <button type="submit"><img class="ic" src="/static/icons/check.svg" alt="">Record acquisition</button>
    </form>
    <p class="err" id="acq-err" role="alert"></p>
  </section>

  <section class="card" id="directories" aria-labelledby="directories-h">
    <h2 id="directories-h"><img class="ic" src="/static/icons/store.svg" alt="">Bookshops &amp; Sources</h2>
    <p style="color:var(--muted);font-size:.9rem;margin:0 0 .5rem">Organizer only — sign in and become an organizer to add shops and sources.</p>
    <div class="grid-2">
      <div>
        <form id="shop-form">
          <label>Name <input name="name" required placeholder="Dhaka Book House"></label>
          <label>Type <select name="shop_type">
            <option value="both">Both</option>
            <option value="physical">Physical</option>
            <option value="online">Online</option>
          </select></label>
          <button type="submit"><img class="ic" src="/static/icons/plus.svg" alt="">Add bookshop</button>
        </form>
        <p class="err" id="shop-err" role="alert"></p>
        <ul class="rows" id="shops"></ul>
      </div>
      <div>
        <form id="source-form">
          <label>Name <input name="name" required placeholder="University Library"></label>
          <label>Type <select name="source_type">
            <option value="other">Other</option>
            <option value="bookshop">Bookshop</option>
            <option value="library">Library</option>
            <option value="gift">Gift</option>
            <option value="friend">Friend</option>
            <option value="publisher">Publisher</option>
            <option value="online_repository">Online repository</option>
          </select></label>
          <button type="submit"><img class="ic" src="/static/icons/plus.svg" alt="">Add source</button>
        </form>
        <p class="err" id="source-err" role="alert"></p>
        <ul class="rows" id="sources"></ul>
      </div>
    </div>
  </section>

  <section class="card" id="analytics" aria-labelledby="analytics-h">
    <h2 id="analytics-h"><img class="ic" src="/static/icons/target.svg" alt="">Analytics &amp; Goals</h2>
    <form id="goal-form">
      <label>Books this month (target)
        <input name="books_target" type="number" min="1" max="100" required placeholder="2"></label>
      <button type="submit"><img class="ic" src="/static/icons/check.svg" alt="">Set goal</button>
    </form>
    <p class="err" id="goal-err" role="alert"></p>
    <p id="goal-status"></p>
    <h3><img class="ic" src="/static/icons/book.svg" alt="">Pages by month</h3>
    <div id="analytics"></div>
    <h3><img class="ic" src="/static/icons/library.svg" alt="">Bookshelf Life</h3>
    <form id="shelf-settings-form">
      <label><input type="checkbox" name="enabled" checked> Enable shelf life</label>
      <div class="grid-2">
        <label>Dust after (days) <input name="dust_days" type="number" min="1" value="30"></label>
        <label>Cobwebs after (days) <input name="cobweb_days" type="number" min="2" value="60"></label>
        <label>Bookworms after (days) <input name="worm_days" type="number" min="3" value="90"></label>
      </div>
      <button type="submit"><img class="ic" src="/static/icons/check.svg" alt="">Save thresholds</button>
    </form>
    <p class="err" id="shelf-err" role="alert"></p>
    <ul class="rows" id="shelf"></ul>
  </section>

  <section class="card" id="files" aria-labelledby="files-h">
    <h2 id="files-h"><img class="ic" src="/static/icons/file.svg" alt="">Digital Files</h2>
    <form id="upload-form">
      <label>Book ID <input name="book_id" required placeholder="Catalog book ID"></label>
      <label>PDF/EPUB file <input name="file" type="file" accept=".pdf,.epub" required></label>
      <button type="submit"><img class="ic" src="/static/icons/plus.svg" alt="">Upload file</button>
    </form>
    <p class="err" id="upload-err" role="alert"></p>
  </section>

  <section class="card" id="create-section" aria-labelledby="catalog-edit-h">
    <h2 id="catalog-edit-h"><img class="ic" src="/static/icons/pen.svg" alt="">Catalog (Organizer)</h2>
    <div class="grid-2">
      <div>
        <form id="writer-form">
          <h3>New writer</h3>
          <label>Full name <input name="full_name" required placeholder="Author name"></label>
          <button type="submit"><img class="ic" src="/static/icons/plus.svg" alt="">Add writer</button>
        </form>
        <p class="err" id="writer-err" role="alert"></p>
      </div>
      <div>
        <form id="book-form">
          <h3>New book</h3>
          <label>Title <input name="title" required placeholder="Book title"></label>
          <label>Writer ID <input name="writer_id" placeholder="Optional"></label>
          <button type="submit"><img class="ic" src="/static/icons/plus.svg" alt="">Add book</button>
        </form>
        <p class="err" id="book-err" role="alert"></p>
      </div>
    </div>
    <details><summary>API health</summary><p><a href="/api/v1/health">GET /api/v1/health</a></p></details>
  </section>

  <p class="foot">KutubCore — offline-first library · <a href="/api/v1/health">API health</a> · <a href="/sw.js">Service worker</a></p>
</main>
<script src="/static/js/app.js"></script>
</body>
</html>`
