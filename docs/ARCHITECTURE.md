# ARCHITECTURE — KutubCore

Implementation snapshot for Milestones A–E. The authoritative design is
`KutubCore_Complete_Project_Blueprint.md`.

## System overview

KutubCore is a **modular monolith** in Go with an **offline-capable PWA**
frontend and a **SQLite** database (WAL). The same binary runs standalone
(localhost), on a LAN host, or pointed at a future PostgreSQL backend.

```
Browser / PWA (vanilla JS) ──HTTPS──> Go service (stdlib net/http)
                                          ├─ REST JSON /api/v1
                                          ├─ SQLite (modernc, pure Go)
                                          └─ data/files/ (PDF/EPUB/covers)
```

## Rules (§13)

- Domain packages never import HTTP handlers; services hold business rules;
  repositories isolate SQL; handlers only shape HTTP.
- SQLite and PostgreSQL code stays in `internal/database/{sqlite,postgres}`.
- Migrations (never ad-hoc DDL) run on startup from the embedded FS.
- Configuration comes from the environment only; secrets are never in code.

## Packages

| Package | Responsibility |
|---|---|
| `cmd/kutubcore` | Server + `backup`/`restore` subcommands |
| `internal/app` | Wires repos/services/handler + middleware |
| `internal/config` | Env config (ADDR, DB_PATH, FILE_DIR, sessions, cookies) |
| `internal/database` | Embedded migration runner (idempotent, one tx per file) |
| `internal/database/sqlite` | SQLite open (WAL, busy_timeout, FKs) + repos |
| `internal/domain/*` | user/id, book, writer, collection, reading, review, bookshop, source, acquisition, files |
| `internal/repository` | Repo interfaces (incl. Audit, Session, Personal, Directories, Acquisition, Files) |
| `internal/service` | Auth (Argon2id/sessions), Catalog, Personal, Directories, Acquisition, Files |
| `internal/handler` | HTTP handlers + embedded PWA shell |
| `internal/middleware` | RequestID, SessionAuth/RequireAuth/RequireOrganizer, rate limits |
| `internal/storage` | Filesystem store (safe names, traversal guard) |
| `internal/backup` | Backup/restore engine (VACUUM INTO + manifest) |
| `migrations/` | Ordered `*.sql` per DB (SQLite complete, Postgres pending) |
| `web/` | PWA shell + service worker (embedded into the binary) |

## Deployment (§2)

- Standalone: Go + SQLite on one machine; PWA reads via localhost.
- LAN: one host runs the service; other devices use its HTTP API.
  Never share a live SQLite file over NFS/SMB.
- Cloud (future): same code, Postgres adapter + durable file storage;
  sync via the queue server-side (see `SYNC_DESIGN.md`).
