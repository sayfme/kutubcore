# KutubCore

Digital Book Collection Management System (DBCMS).
Offline-first · Personal + Community · Go · SQLite → PostgreSQL.

**Status:** Milestone F + LAN working (hardened + LAN-ready).
Full design: `KutubCore_Complete_Project_Blueprint.md` (§1-§20).

## What works now

- **Auth:** register / login / logout / me (email or mobile, Argon2id password hashing,
  server-side sessions + `HttpOnly; SameSite=Lax` cookie, 30-day TTL, rate limit 30/min
  on login/register). Every user starts as Reader; self-activate Organizer via
  `POST /api/v1/users/me/roles/organizer` (Blueprint §4).
- **Directories:** bookshops (`/bookshops` + links) and collection sources
  (`/sources` + links), public reads, organizer writes with audit.
- **Acquisitions (`/me/acquisitions`, `/me/copies`):** one acquisition with many
  items, each unit creates a `book_copies` row in one transaction; acquisition
  history never rewrites collection status by itself (rule 7).
- **Files:** upload PDF/EPUB or cover image (Organizer), safenamed on disk under
  `FILE_DIR` with SHA-256 checksum/size/MIME validation + traversal protection,
  or record an external URL (`IsAvailable=false` until downloaded, rule 10),
  download via `GET /files/{id}/download` (attachment or redirect).
- **Catalog:** Books + Editions + Identifiers + Writers + Languages + Genres.
  Book (work) / Edition (published version) stay separate (Blueprint §20).
  Optimistic concurrency via `version`, soft-delete via `deleted_at`, audit logs on
  every catalog change, duplicate detection on book create (`?force=true` to override).
- **Personal library (`/api/v1/me/*`, owner-checked):** add books to your library,
  track physical (`not_collected|want_to_buy|owned`), digital
  (`not_available|want_to_download|downloaded`), and reading
  (`want_to_read|currently_reading|on_hold|completed|abandoned`) independently (§5).
  **Rule 1 enforced:** `owned` requires ≥1 copy (record an acquisition first).
  **Rule 2 enforced:** `downloaded` requires ≥1 available file (upload first).
  Reading progress (current/total page, auto percent, started/completed timestamps),
  reading sessions, private notes, 1:1 reviews (star 1-5 + reaction `favorite|average|meh|legend`
  kept separate, §5), favorites, and `GET /me/dashboard` (totals, recent sessions, recently added).
  Cross-user access returns 404 (never leaks another user's library).
- **API:** REST JSON under `/api/v1` (§9 envelope, pagination `?limit=&page=`, filters
  `?q=&physical_status=&reading_status=&favorite=`). Public reads for guests;
  organizer-only writes; private data never on public endpoints.
- **Foundation:** Go modular monolith, stdlib `net/http`, SQLite (WAL, busy_timeout,
  `foreign_keys=ON`, single writer), embedded ordered migrations, structured logging,
  request IDs, graceful shutdown, embedded PWA shell.
- **FTS5 search:** `q` on the catalog is now full-text (prefix-AND over
  title/subtitle/description/original_title/writers, tokenizer
  `unicode61 remove_diacritics 1`), trigger-maintained, with safe quoting
  (operator injection impossible) and LIKE fallback on empty queries.
- **Offline PWA:** `GET /sw.js` (root scope) precaches shell+static
  (cache-first), serves public catalog GETs network-first with cache fallback
  (`/books`, `/writers`, `/languages`, `/genres`, `/bookshops`, `/sources`),
  never caches personal `/me/*|/auth/*|/users/*` data; online/offline badge.
- **Backup/restore:** `kutubcore backup` snapshots DB via `VACUUM INTO` +
  file tree into `data/backups/<ts>/` with a SHA-256 `manifest.json`;
  `kutubcore restore <dir>` verifies checksums + `integrity_check` and keeps
  a safety copy of the live DB. Wrappers: `scripts/backup.sh`, `scripts/restore.sh`.
- **Seed:** `scripts/seed.sh` (needs server on `$ADDR`, default :8080) creates
  a demo organizer, writers, books + edition, bookshop, acquisition, PDF
  upload, and library entry.
- **Docs pack:** `docs/{ARCHITECTURE,ERD,DATABASE_SCHEMA,API_SPECIFICATION,UI_SPECIFICATION,SECURITY,SYNC_DESIGN,DEVELOPMENT_ROADMAP,CONTRIBUTING,CHANGELOG}.md`
  split from the blueprint.
- **Password reset + email verification (F):** `POST /auth/forgot-password`
  (always 200, no enumeration) → `POST /auth/reset-password` (single-use,
  1 h, invalidates sessions); `POST /auth/verify-email` + resend
  (72 h, optional — never gates login). No SMTP in v1: reset/verify links
  go to the server log for local/dev (wire SMTP before exposing).
- **HTTPS behind proxy + security headers (F):** direct TLS via
  `TLS_CERT_FILE`/`TLS_KEY_FILE`, or terminate TLS in nginx/caddy with
  `COOKIE_SECURE=true`; HSTS + `nosniff` + `X-Frame-Options` on all responses.
- **LAN:** bind `0.0.0.0:8080`, server logs a `lan_url` + LAN IP, plain-HTTP
  warning if insecure on LAN; `scripts/lan.sh` prints IP + mDNS hint +
  firewall hint; guide in `docs/LAN_DEPLOYMENT.md` (mDNS via Avahi,
  ufw LAN-only, proxy HTTPS recipe).
- **E2E + CI (F):** `tests/e2e_test.go` (real HTTP sockets, full happy path);
  `scripts/ci.sh` (gofmt/vet/test/build) run by `.github/workflows/ci.yml`
  and `.gitea/workflows/ci.yaml`.

Not yet (Cloud/Enhancements, on purpose): Postgres adapter + sync, gamification,
analytics/recommendations. The local MVP + LAN are complete.

## Run locally

```sh
# Requires Go 1.26+ (go.mod floor; tested on 1.27)
cp .env.example .env   # optional; defaults work without it

go run ./cmd/kutubcore
# Listening on :8080, DB at data/kutubcore.db (auto-created, migrations applied)

curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/books
```

Try it end-to-end:

```sh
# register (auto-login, sets cookie) + become organizer
curl -c cj.txt -H 'Content-Type: application/json' \
  -d '{"display_name":"You","email":"you@example.com","password":"secret123"}' \
  http://localhost:8080/api/v1/auth/register
curl -b cj.txt -H 'Content-Type: application/json' -d '{}' \
  http://localhost:8080/api/v1/users/me/roles/organizer

# create writer + book + edition
curl -b cj.txt -H 'Content-Type: application/json' \
  -d '{"full_name":"Rabindranath Tagore"}' http://localhost:8080/api/v1/writers
curl -b cj.txt -H 'Content-Type: application/json' \
  -d '{"title":"Gitanjali","writers":[{"id":"<writer-id>"}]}' http://localhost:8080/api/v1/books
curl http://localhost:8080/api/v1/books?q=gitanjali
```

Open http://localhost:8080 for the shell.

## Test

```sh
go test ./...
go vet ./...
gofmt -l .
```

Tests use in-memory SQLite with real migrations (no test doubles for the DB).

## Project layout (Blueprint §13)

```
cmd/kutubcore/          main entrypoint
internal/
  app/                  wires deps (DB + logger -> HTTP handler)
  config/               env-based config (ADDR, DB_PATH, SESSION_TTL_HOURS, COOKIE_SECURE)
  database/
    migrate.go          embedded migration runner (idempotent, one tx per file)
    sqlite/             SQLite adapter: Open + user/session/catalog repos
    postgres/           future adapter placeholder (Milestone 5)
  domain/user,book,writer,collection,reading,review,bookshop,source,acquisition,files/
  repository/           repo interfaces (User, Session, Book, Personal, Directories, Acquisition, Files, Audit)
  service/              business rules: Auth, Catalog, Personal, Directories, Acquisition, Files
  handler/              HTTP handlers (auth, users, books, writers, reference, personal, directories, acquisitions, files)
  middleware/           RequestID, SessionAuth/RequireAuth/RequireOrganizer, rate limit
  storage/              filesystem storage (safe names, traversal guard)
  testutil/             in-memory app builder for handler tests
web/static/{css,js}/   PWA shell (css, js, manifest)
migrations/sqlite/      0001..0007 (init, auth, catalog, personal, directories, acquisitions, files)
data/                   gitignored: db, files/, backups/, logs/
```

## Locked decisions (updated Milestone F)

| Concern | Decision | Why |
|---|---|---|
| Backend | Go, modular monolith | Blueprint §20 |
| HTTP | stdlib `net/http` (ServeMux, path params) | No framework dep yet |
| SQLite driver | `modernc.org/sqlite` (pure Go, no CGO) | Standalone binary works anywhere |
| SQLite pragmas | WAL, busy_timeout 5s, foreign_keys ON, single writer (`MaxOpenConns=1`) | Avoid "database is locked" on LAN host |
| Migrations | Custom runner + `schema_migrations`, one tx per file | Zero extra dep; portable SQL for Postgres later |
| Passwords | Argon2id (m=64 MiB, t=1, p=4, 32-byte key) + min 8 chars | Blueprint §14 |
| Sessions | Server-side, token hashed (SHA-256), HttpOnly SameSite=Lax cookie, 30d | No JWT needed for standalone/LAN; revoke on logout |
| Rate limit | 30/min/IP on register+login (in-memory fixed window) | Brute-force slowdown (§14); move to shared store in cloud |
| IDs | UUIDv4 strings, generated server-side | Blueprint §6 |
| Concurrency | Optimistic `version` on books/editions/writers | §9 optimistic concurrency |
| Errors | Envelope error codes (NOT_FOUND, VALIDATION_ERROR, DUPLICATE_POSSIBLE, etc.) | §9 |
| Go floor | 1.26+ (current go.mod) | `golang.org/x/crypto` floor |

## Defining rule

> Catalog identity, Edition identity, and Personal Library identity
> must remain separate. (Blueprint §20)

## Next step

Cloud + Enhancements are deliberately deferred. The offline MVP + LAN are
complete. Say `go cloud` for Milestone H (Postgres + sync), or `go polish`
for gamification/analytics/import-export.
