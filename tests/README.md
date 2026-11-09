# Tests

- Unit/integration: `internal/**` (in-memory SQLite with real migrations,
  `httptest` through the handler stack).
- End-to-end: `tests/e2e_test.go` — real HTTP sockets, full happy path
  (register → catalog → acquisition → library → tracker → dashboard → PWA),
  plus file-DB checks.
- Coverage of Blueprint §16 targets: status transitions, genre hierarchy
  (flat in v1), permission checks, progress math, duplicate detection,
  FK enforcement, authorization, file upload/download. Sync/E2E offline
  suites arrive with the cloud phase.
