# SYNC_DESIGN — KutubCore (Blueprint §10; cloud phase, not yet built)

Goal: bidirectional sync between a local/hybrid client and the cloud
PostgreSQL API, after the local MVP is stable.

## Current state (A–E)

- Schema is sync-ready: UUID PKs, `created_at`, `updated_at`, `deleted_at`
  tombstones on catalog tables, `version` for optimistic concurrency.
- Local writes already work offline (SQLite is the source of truth locally).
- No sync queue/tables yet — they arrive with the Postgres adapter.

## Planned design (unchanged from blueprint §10)

- Device identity: stable `device_id` (server-generated UUID per client).
- `sync_queue` (outbox): each syncable write enqueues an event in the same DB
  transaction; push/pull APIs (`POST /sync/push`, `GET /sync/pull` with
  cursor, `GET /sync/status`) reconcile with optimistic version checks.
- Conflicts: catalog = explicit resolution; progress = reconcile latest valid;
  notes/reviews = revision history, never silent overwrite; acquisitions =
  append-only; favorites = last-write-wins acceptable.
- PWA caching stays as in `UI_SPECIFICATION.md`: shell cache-first, public
  catalog network-first, personal data never cached.
