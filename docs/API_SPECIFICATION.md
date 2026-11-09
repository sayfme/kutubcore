# API_SPECIFICATION — KutubCore `/api/v1`

Envelope (Blueprint §9):

```json
// success
{ "success": true, "data": …, "meta": … }
// error
{ "success": false, "error": { "code": "…", "message": "…" } }
```

Errors: `VALIDATION_ERROR` (422), `UNAUTHENTICATED` (401),
`FORBIDDEN` (403), `NOT_FOUND` (404), `DUPLICATE_POSSIBLE` (409),
`RATE_LIMIT` (429), `DB_UNAVAILABLE`/`INTERNAL` (503/500).
Pagination: `?limit=20&page=1` + `meta{limit,page,total}`.
Search: `?q=` uses FTS5 prefix-AND (prefix on every token);
filters: `?language=&genre=&writer_id=&physical_status=&digital_status=&reading_status=&favorite=`.
Session cookie: `kutubcore_session` (`HttpOnly; SameSite=Lax`).

## Auth & users

| Method | Endpoint | Access |
|---|---|---|
| POST | `/auth/register` `{display_name, email?, mobile?, password}` | public (rate-limited) |
| POST | `/auth/login` `{email?\|mobile?\|identifier, password}` | public (rate-limited) |
| POST | `/auth/logout` | any (clears cookie) |
| GET | `/auth/me` | auth |
| GET | `/users/me`, `/users/me/roles` | auth |
| POST | `/users/me/roles/organizer` | auth (self-activate) |

## Catalog (public reads, organizer writes)

`GET /books`, `GET /books/{id}` (incl. editions), `GET|POST /books/{id}/editions`,
`GET|POST|PATCH|DELETE /writers[/{id}]`, `GET /languages|/genres`,
`GET|POST|PATCH|DELETE /bookshops[/{id}]` + `/bookshops/{id}/links`,
`GET|POST|PATCH|DELETE /sources[/{id}]` + `/sources/{id}/links`.
Duplicate books without `?force=true` → `409 DUPLICATE_POSSIBLE`.

## Files

`GET /books/{id}/files` (public metadata),
`POST /books/{id}/files` (multipart PDF/EPUB or JSON `{external_url}`),
`POST /books/{id}/cover` (multipart image),
`GET /files/{id}/download` (attachment, or redirect for external URLs).

## Personal (`/api/v1/me/*`, always owner-checked)

`GET /dashboard`, `GET|POST /books`, `GET|PATCH|DELETE /books/{id}`,
`GET|PUT /books/{id}/progress`, `GET|POST /books/{id}/sessions`,
`GET|POST /books/{id}/notes + PATCH|DELETE …/notes/{noteId}`,
`GET|PUT|DELETE /books/{id}/review`,
`GET|POST|DELETE /acquisitions[/{id}]`, `GET /copies?book_id=`.

Collection rules: `owned` requires a copy (record acquisition first);
`downloaded` requires an available file (upload first) → `422` otherwise.

## Health & PWA

`GET /healthz`, `GET /api/v1/health`, `GET /` (shell), `GET /sw.js`, `GET /static/*`.
