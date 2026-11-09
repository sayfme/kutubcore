# CHANGELOG — KutubCore

## Unreleased — Milestone E (Offline + polish)
- FTS5 full-text search on books (`books_fts` + triggers, prefix-AND queries,
  LIKE fallback) replacing plain `%LIKE%` on the catalog endpoint.
- PWA service worker (`GET /sw.js`, root scope): shell/static cache-first,
  public catalog network-first with cache fallback, personal data never
  cached; online/offline indicator in the shell.
- Embedded static assets (`kutubcore/web`) so `/static/*` and `/sw.js` work
  regardless of process working directory (fixes `go test` CWD 404s).
- Backup/restore CLI (`kutubcore backup` / `restore`) + `scripts/backup.sh` /
  `scripts/restore.sh`: VACUUM INTO snapshot, file tree copy, SHA-256
  manifest, integrity verify + safety copy before restore.
- Demo seed (`scripts/seed.sh`): organizer, writers, books + edition,
  bookshop, acquisition, PDF upload, library entry.
- Docs pack split from the blueprint (`ARCHITECTURE`, `ERD`,
  `DATABASE_SCHEMA`, `API_SPECIFICATION`, `UI_SPECIFICATION`, `SECURITY`,
  `SYNC_DESIGN`, `DEVELOPMENT_ROADMAP`, `CONTRIBUTING`, `CHANGELOG`).

## Milestone D
- Bookshops + sources with links; acquisitions/items/copies with owned-file
  enforcement (rules 1–2); PDF/EPUB/cover uploads + downloads.

## Milestone C
- Personal library, statuses, progress/sessions, notes/reviews, dashboard.

## Milestone B
- Auth (Argon2id + sessions), roles, catalog CRUD, audit, duplicates.

## Milestone A
- Foundation: config, SQLite, migrations, health, PWA shell.
