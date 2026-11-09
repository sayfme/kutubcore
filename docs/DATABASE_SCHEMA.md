# DATABASE_SCHEMA — KutubCore (SQLite, migrations 0001–0008)

Conventions: UUID text PKs, UTC timestamps as RFC3339 text, soft-delete via
`deleted_at`, portable `VARCHAR+CHECK` instead of PG enums, partial unique
indexes for optional identifiers, FKs enforced on every connection.

## Tables by domain

| Domain | Tables (migration) |
|---|---|
| System | `app_meta`, `schema_migrations` (0001) |
| Identity | `users`, `roles`, `user_roles`, `sessions` (0002) |
| Catalog | `books`, `editions`, `book_identifiers`, `writers`, `book_writers`, `languages`, `genres`, `book_genres`, `books_fts` (0003, 0008) |
| Personal | `user_books`, `reading_progress`, `reading_sessions`, `notes`, `reviews` (0004) |
| Directories | `bookshops`, `bookshop_links`, `collection_sources`, `source_links` (0005) |
| Acquisitions | `acquisitions`, `acquisition_items`, `book_copies` (0006) |
| Files | `book_files`, `file_locations` (0007) |
| Audit | `audit_logs` (0003) |

See `ERD.md` for the relationship diagram and
`KutubCore_Complete_Project_Blueprint.md` §6 for the full column list.

## Key rules

- `users.email` / `users.mobile`: partial unique (only when present).
- `user_books`: unique `(user_id, book_id)`; CHECKs on the three statuses.
- Collection rules (§7): `owned` ⇒ ≥1 copy; `downloaded` ⇒ ≥1 available file.
- `books_fts`: FTS5 virtual table (`book_id` UNINDEXED + 5 text columns)
  maintained by triggers on books/book_writers; backfilled in 0008.
- Indexes: catalog (title, deleted, book→editions), personal (user+statuses),
  acquisitions (user+date), sessions (user_book), audit (entity, actor).
