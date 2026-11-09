# KutubCore

Digital Book Collection Management System (DBCMS).
Offline-first · Personal + Community · Go · SQLite → PostgreSQL.

**Status:** Milestone 1 — Foundation (in progress).
Full design: `KutubCore_Complete_Project_Blueprint.md` (§1-§20).

## What this is (so far)

- Go modular monolith, stdlib `net/http` only — no framework dependency yet.
- SQLite (pure-Go driver, no CGO) with WAL, `busy_timeout`, and
  `PRAGMA foreign_keys = ON` on every connection.
- Embedded, ordered, idempotent SQL migrations (`migrations/sqlite/*.sql`).
- Structured logging (`log/slog`) + request IDs + graceful shutdown.
- PWA shell at `/` with manifest and static assets.

Not yet: auth, catalog CRUD, personal library, search, sync, file uploads.
Those are Milestones 2-6 per the blueprint.

## Run locally

```sh
# Requires Go 1.24+
cp .env.example .env   # optional; defaults work without it

go run ./cmd/kutubcore
# Listening on :8080, DB at data/kutubcore.db (auto-created)

curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/health
```

Open http://localhost:8080 for the shell.

## Test

```sh
go test ./...
go vet ./...
gofmt -l .
```

## Project layout (Blueprint §13)

```
cmd/kutubcore/          main entrypoint
internal/
  app/                  wires deps (DB + logger -> HTTP handler)
  config/               env-based config, no secrets in code
  database/
    migrate.go          embedded migration runner (idempotent)
    sqlite/             SQLite adapter (Open, pragmas)
    postgres/           future adapter placeholder (Milestone 5)
  domain/{user,book,writer,genre,language,bookshop,collection,reading,review,sync}/
  repository/ service/ handler/ middleware/ validator/ storage/
web/static/{css,js,icons,images}/  web/templates/  web manifest
migrations/sqlite/      ordered *.sql, applied on startup
migrations/postgres/    future (Milestone 5)
data/                   gitignored: db, files/, backups/, logs/
docs/ scripts/ tests/
```

## Locked decisions (Milestone 1)

| Concern | Decision | Why |
|---|---|---|
| Backend | Go, modular monolith | Blueprint §20 |
| HTTP | stdlib `net/http` (ServeMux, `GET /healthz`) | No dependency until routing needs it |
| SQLite driver | `modernc.org/sqlite` (pure Go, no CGO) | Standalone binary works anywhere; LAN/standalone friendlier |
| SQLite pragmas | WAL, busy_timeout 5s, foreign_keys ON, single writer (`MaxOpenConns=1`) | Avoid "database is locked" on LAN host |
| Migrations | Custom runner + `schema_migrations` table, one tx per file | Zero extra dep; portable SQL for Postgres later |
| Config | Env vars (`ADDR`, `DB_PATH`, `LOG_LEVEL`) | Secrets never in code (§14) |
| Logging | `log/slog` text handler | Structured, stdlib, no lock-in |
| API envelope | `{success, data, meta}` / `{success, error{code,message}}` | Blueprint §9 |

Everything else (router lib, validator, Argon2id, FTS5, PWA caching
strategy) is intentionally not locked until the milestone that needs it.

## Defining rule

> Catalog identity, Edition identity, and Personal Library identity
> must remain separate. (Blueprint §20)

## Next step

Milestone 2 — Catalog: users/roles tables + auth, then books/editions/writers.
Say `go B` to continue.
