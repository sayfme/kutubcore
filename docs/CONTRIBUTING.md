# CONTRIBUTING — KutubCore

## Workflow

1. Branch from `main`, keep the change small and inside one milestone.
2. Schema changes: add the next `migrations/sqlite/NNNN_*.sql` (ordered,
   idempotent, one transaction). Never edit an applied migration.
3. Add/extend tests with real migrations (in-memory SQLite via
   `internal/testutil`), not mocks.
4. Update `docs/` + `README.md` + `CHANGELOG.md` with the change.

## Checks (must pass before merging)

```sh
gofmt -l .
go vet ./...
go test ./...
```

## Conventions

- Go modular monolith (§13): handlers thin, services hold rules, repos hold SQL.
- Portable SQL (varchar+check, UUID text, UTC RFC3339 timestamps).
- Single-writer SQLite (`MaxOpenConns=1`): never run a nested query while
  rows are still open (collect + close first).
- API envelope `{success, data, meta}` / `{success, error{code,message}}`;
  personal routes always owner-checked; updates bump `version`.

## Secrets & data

Never commit `.env`, `data/*.db`, uploaded PDFs/EPUBs, or backups.
`data/` is gitignored; backups in `data/backups/` stay local.
