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
<html lang="en" data-bs-theme="auto">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#1f2a44">
<title>KutubCore — Your Library, Beautifully Shelved</title>
<meta name="description" content="KutubCore — an offline-first library for your books.">
<link rel="manifest" href="/static/manifest.webmanifest">
<link rel="stylesheet" href="/static/css/bootstrap.min.css">
<link rel="stylesheet" href="/static/css/app.css">
</head>
<body>
<header class="container py-4">
  <div class="d-flex align-items-center gap-3 flex-wrap">
    <div class="logo-wrap" style="width:56px;height:56px" aria-hidden="true"><img class="ic" src="/static/icons/logo.svg" alt="" style="width:34px;height:34px"></div>
    <div class="brand">
      <h1 class="mb-0 display-6 fw-bold">KutubCore</h1>
      <p class="tagline mb-0 text-body-secondary">Your library, beautifully shelved — offline-first.</p>
    </div>
    <div class="ms-auto d-flex gap-2">
      <span class="badge text-bg-success" id="pill-online"><span class="dot" aria-hidden="true"></span> <span id="online-status" role="status">Online</span></span>
      <span class="badge text-bg-danger d-none" id="pill-offline"><span class="dot" aria-hidden="true"></span> Offline — cached catalog</span>
    </div>
  </div>
  <div class="spine-strip" aria-hidden="true"></div>
</header>

<nav class="navbar navbar-expand bg-body-tertiary sticky-top border-bottom" aria-label="Sections">
  <div class="container" style="gap:.65rem">
    <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navCollapse" aria-controls="navCollapse" aria-expanded="false" aria-label="Toggle navigation"><span class="navbar-toggler-icon"></span></button>
    <div class="collapse navbar-collapse" id="navCollapse">
      <ul class="navbar-nav">
        <li class="nav-item"><a class="nav-link" href="#catalog"><img class="ic" src="/static/icons/book.svg" alt="">Catalog</a></li>
        <li class="nav-item"><a class="nav-link" href="#library"><img class="ic" src="/static/icons/library.svg" alt="">My Collection</a></li>
        <li class="nav-item"><a class="nav-link" href="#analytics"><img class="ic" src="/static/icons/target.svg" alt="">Reading</a></li>
        <li class="nav-item"><a class="nav-link" href="#directories" id="nav-directories"><img class="ic" src="/static/icons/store.svg" alt="">Bookshops</a></li>
        <li class="nav-item"><a class="nav-link" href="#signin"><img class="ic" src="/static/icons/signin.svg" alt="">Sign in</a></li>
      </ul>
      <span class="d-flex align-items-center gap-2 ms-auto text-body-secondary"><span class="avatar" id="avatar" aria-hidden="true">?</span> <span id="who">Loading…</span></span>
    </div>
  </div>
</nav>

<main class="container py-4" style="max-width:960px">
  <section class="card shadow-sm mb-4" id="catalog" aria-labelledby="catalog-h">
    <div class="card-body">
      <h2 class="card-title h5" id="catalog-h"><img class="ic" src="/static/icons/search.svg" alt="">Catalog</h2>
      <form id="search-form" role="search" class="row g-2 align-items-center">
        <div class="col-12 col-md-9"><input class="form-control" name="q" placeholder="Title, original title, writer…" autocomplete="off" aria-label="Search"></div>
        <div class="col-12 col-md-3"><button type="submit" class="btn btn-primary w-100"><img class="ic" src="/static/icons/search.svg" alt="">Search</button></div>
      </form>
      <ul class="row g-3 mt-1 list-unstyled" id="books"></ul>
    </div>
  </section>

  <section class="card shadow-sm mb-4" id="signin" aria-labelledby="signin-h">
    <div class="card-body">
      <h2 class="card-title h5" id="signin-h"><img class="ic" src="/static/icons/signin.svg" alt="">Sign in</h2>
      <form id="auth-form">
        <div class="mb-3"><label class="form-label">Display name <input class="form-control" name="display_name" autocomplete="name" placeholder="Your name (for registering)"></label></div>
        <div class="mb-3"><label class="form-label">Email or identifier <input class="form-control" name="identifier" autocomplete="username" placeholder="you@example.com or mobile"></label></div>
        <input type="hidden" name="email">
        <input type="hidden" name="mobile">
        <div class="mb-3"><label class="form-label">Password <input class="form-control" type="password" name="password" required autocomplete="current-password" placeholder="Minimum 8 characters"></label></div>
        <div class="d-flex flex-wrap gap-2">
        <button type="submit" class="btn btn-primary w-100 w-md-auto"><img class="ic" src="/static/icons/signin.svg" alt="">Sign in</button>
        <button type="button" id="register-btn" class="btn btn-outline-secondary w-100 w-md-auto"><img class="ic" src="/static/icons/plus.svg" alt="">Register</button>
      </div>
      </form>
      <div id="account-view" style="display:none">
        <p class="mb-2">You are signed in. Your library, acquisitions, and reading stats are below.</p>
        <p class="mt-3 mb-0 d-flex flex-wrap gap-2"><button type="button" class="btn btn-outline-secondary w-100 w-md-auto" id="organizer-btn" hidden><img class="ic" src="/static/icons/star.svg" alt="">Become an Organizer</button>
           <button type="button" class="btn btn-outline-secondary w-100 w-md-auto" id="logout-btn" hidden>Sign out</button></p>
      </div>
      <p class="err text-danger" id="auth-err" role="alert"></p>
      <p class="text-body-secondary small mb-0" id="signin-note">Browsing as a guest is fine — sign in to keep your own shelves, notes, and goals.</p>
    </div>
  </section>

  <section class="card shadow-sm mb-4" id="library" aria-labelledby="library-h" style="display:none">
    <div class="card-body">
      <h2 class="card-title h5" id="library-h"><img class="ic" src="/static/icons/library.svg" alt="">My Collection</h2>
      <form id="add-book-form">
        <div class="mb-3"><label class="form-label">Catalog book ID <input class="form-control" name="book_id" required placeholder="Paste a book ID from the catalog"></label></div>
        <div class="row g-2">
          <div class="col-12 col-md-6"><label class="form-label">Physical
            <select class="form-select" name="physical_status">
              <option value="not_collected">Not collected</option>
              <option value="want_to_buy">Want to buy</option>
              <option value="owned">Owned</option>
            </select></label></div>
          <div class="col-12 col-md-6"><label class="form-label">Reading
            <select class="form-select" name="reading_status">
              <option value="want_to_read">Want to read</option>
              <option value="currently_reading">Currently reading</option>
              <option value="on_hold">On hold</option>
              <option value="completed">Completed</option>
              <option value="abandoned">Abandoned</option>
            </select></label></div>
        </div>
        <div class="form-check mb-3 mt-3"><input class="form-check-input" type="checkbox" name="is_favorite" id="fav-c"><label class="form-check-label" for="fav-c">Favorite</label></div>
        <button type="submit" class="btn btn-primary w-100 w-sm-auto"><img class="ic" src="/static/icons/plus.svg" alt="">Add to my library</button>
      </form>
      <p class="err text-danger" id="add-book-err" role="alert"></p>
      <p id="dash"></p>
      <ul class="rows list-group list-group-flush" id="my-books"></ul>
    </div>
  </section>

  <section class="card shadow-sm mb-4" id="acquisitions" aria-labelledby="acq-h" style="display:none">
    <div class="card-body">
      <h2 class="card-title h5" id="acq-h"><img class="ic" src="/static/icons/store.svg" alt="">Acquisitions</h2>
      <p class="text-body-secondary small">Record a purchase or gift — each unit becomes a copy on your shelf. This unlocks “Owned” for that book.</p>
      <form id="acq-form">
        <div class="row g-2">
          <div class="col-12 col-md-6"><label class="form-label">Book ID <input class="form-control" name="book_id" required></label></div>
          <div class="col-12 col-md-6"><label class="form-label">Bookshop ID <input class="form-control" name="bookshop_id" placeholder="Optional"></label></div>
          <div class="col-12 col-md-6"><label class="form-label">Date <input class="form-control" name="acquisition_date" type="date"></label></div>
          <div class="col-12 col-md-4"><label class="form-label">Total cost <input class="form-control" name="total_cost" type="number" min="0" step="0.01" placeholder="Optional"></label></div>
          <div class="col-12 col-md-2"><label class="form-label">Currency <input class="form-control" name="currency_code" maxlength="3" placeholder="USD"></label></div>
        </div>
        <button type="submit" class="btn btn-primary mt-2 w-100 w-sm-auto"><img class="ic" src="/static/icons/check.svg" alt="">Record acquisition</button>
      </form>
      <p class="err text-danger" id="acq-err" role="alert"></p>
    </div>
  </section>

  <section class="card shadow-sm mb-4" id="directories" aria-labelledby="directories-h" style="display:none">
    <div class="card-body">
      <h2 class="card-title h5" id="directories-h"><img class="ic" src="/static/icons/store.svg" alt="">Bookshops &amp; Sources</h2>
      <p class="text-body-secondary small">Organizer only — sign in and become an organizer to add shops and sources.</p>
      <div class="row g-3">
        <div class="col-12 col-md-6">
          <form id="shop-form">
            <div class="mb-3"><label class="form-label">Name <input class="form-control" name="name" required placeholder="Dhaka Book House"></label></div>
            <div class="mb-3"><label class="form-label">Type <select class="form-select" name="shop_type">
              <option value="both">Both</option>
              <option value="physical">Physical</option>
              <option value="online">Online</option>
            </select></label></div>
            <button type="submit" class="btn btn-primary w-100 w-sm-auto"><img class="ic" src="/static/icons/plus.svg" alt="">Add bookshop</button>
          </form>
          <p class="err text-danger" id="shop-err" role="alert"></p>
          <ul class="rows list-group list-group-flush" id="shops"></ul>
        </div>
        <div class="col-12 col-md-6">
          <form id="source-form">
            <div class="mb-3"><label class="form-label">Name <input class="form-control" name="name" required placeholder="University Library"></label></div>
            <div class="mb-3"><label class="form-label">Type <select class="form-select" name="source_type">
              <option value="other">Other</option>
              <option value="bookshop">Bookshop</option>
              <option value="library">Library</option>
              <option value="gift">Gift</option>
              <option value="friend">Friend</option>
              <option value="publisher">Publisher</option>
              <option value="online_repository">Online repository</option>
            </select></label></div>
            <button type="submit" class="btn btn-primary w-100 w-sm-auto"><img class="ic" src="/static/icons/plus.svg" alt="">Add source</button>
          </form>
          <p class="err text-danger" id="source-err" role="alert"></p>
          <ul class="rows list-group list-group-flush" id="sources"></ul>
        </div>
      </div>
    </div>
  </section>

  <section class="card shadow-sm mb-4" id="analytics" aria-labelledby="analytics-h" style="display:none">
    <div class="card-body">
      <h2 class="card-title h5" id="analytics-h"><img class="ic" src="/static/icons/target.svg" alt="">Analytics &amp; Goals</h2>
      <form id="goal-form" class="row g-2 align-items-center">
        <div class="col-12 col-md-8"><label class="form-label">Books this month (target)
          <input class="form-control" name="books_target" type="number" min="1" max="100" required placeholder="2"></label></div>
        <div class="col-12 col-md-4"><button type="submit" class="btn btn-primary w-100"><img class="ic" src="/static/icons/check.svg" alt="">Set goal</button></div>
      </form>
      <p class="err text-danger" id="goal-err" role="alert"></p>
      <p id="goal-status"></p>
      <h3 class="h6 mt-3"><img class="ic" src="/static/icons/book.svg" alt="">Pages by month</h3>
      <div id="analytics"></div>
      <h3 class="h6 mt-4"><img class="ic" src="/static/icons/library.svg" alt="">Bookshelf Life</h3>
      <form id="shelf-settings-form">
        <div class="form-check mb-3"><input class="form-check-input" type="checkbox" name="enabled" id="shelf-on" checked><label class="form-check-label" for="shelf-on">Enable shelf life</label></div>
        <div class="row g-2">
          <div class="col-12 col-md-4"><label class="form-label">Dust after (days) <input class="form-control" name="dust_days" type="number" min="1" value="30"></label></div>
          <div class="col-12 col-md-4"><label class="form-label">Cobwebs after (days) <input class="form-control" name="cobweb_days" type="number" min="2" value="60"></label></div>
          <div class="col-12 col-md-4"><label class="form-label">Bookworms after (days) <input class="form-control" name="worm_days" type="number" min="3" value="90"></label></div>
        </div>
        <button type="submit" class="btn btn-primary mt-2 w-100 w-sm-auto"><img class="ic" src="/static/icons/check.svg" alt="">Save thresholds</button>
      </form>
      <p class="err text-danger" id="shelf-err" role="alert"></p>
      <ul class="rows list-group list-group-flush" id="shelf"></ul>
    </div>
  </section>

  <section class="card shadow-sm mb-4" id="files" aria-labelledby="files-h" style="display:none">
    <div class="card-body">
      <h2 class="card-title h5" id="files-h"><img class="ic" src="/static/icons/file.svg" alt="">Digital Files</h2>
      <form id="upload-form">
        <div class="mb-3"><label class="form-label">Book ID <input class="form-control" name="book_id" required placeholder="Catalog book ID"></label></div>
        <div class="mb-3"><label class="form-label">PDF/EPUB file <input class="form-control" name="file" type="file" accept=".pdf,.epub" required></label></div>
        <button type="submit" class="btn btn-primary w-100 w-sm-auto"><img class="ic" src="/static/icons/plus.svg" alt="">Upload file</button>
      </form>
      <p class="err text-danger" id="upload-err" role="alert"></p>
    </div>
  </section>

  <section class="card shadow-sm mb-4" id="create-section" aria-labelledby="catalog-edit-h" style="display:none">
    <div class="card-body">
      <h2 class="card-title h5" id="catalog-edit-h"><img class="ic" src="/static/icons/pen.svg" alt="">Catalog (Organizer)</h2>
      <div class="row g-3">
        <div class="col-12 col-md-6">
          <form id="writer-form">
            <h3 class="h6">New writer</h3>
            <div class="mb-3"><label class="form-label">Full name <input class="form-control" name="full_name" required placeholder="Author name"></label></div>
            <button type="submit" class="btn btn-primary w-100 w-sm-auto"><img class="ic" src="/static/icons/plus.svg" alt="">Add writer</button>
          </form>
          <p class="err text-danger" id="writer-err" role="alert"></p>
        </div>
        <div class="col-12 col-md-6">
          <form id="book-form">
            <h3 class="h6">New book</h3>
            <div class="mb-3"><label class="form-label">Title <input class="form-control" name="title" required placeholder="Book title"></label></div>
            <div class="mb-3"><label class="form-label">Writer ID <input class="form-control" name="writer_id" placeholder="Optional"></label></div>
            <button type="submit" class="btn btn-primary w-100 w-sm-auto"><img class="ic" src="/static/icons/plus.svg" alt="">Add book</button>
          </form>
          <p class="err text-danger" id="book-err" role="alert"></p>
        </div>
      </div>
      <details class="mt-3"><summary>API health</summary><p><a href="/api/v1/health">GET /api/v1/health</a></p></details>
    </div>
  </section>

  <p class="foot text-center text-body-secondary small py-4">KutubCore — offline-first library · <a href="/api/v1/health">API health</a> · <a href="/sw.js">Service worker</a></p>
</main>
<script src="/static/js/bootstrap.bundle.min.js"></script>
<script src="/static/js/app.js"></script>
</body>
</html>`
