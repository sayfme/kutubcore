# SECURITY — KutubCore (Blueprint §14, implemented A–E)

| Area | Implementation |
|---|---|
| Passwords | Argon2id (m=64 MiB, t=1, p=4, 32-byte key), min 8 chars |
| Sessions | Server-side, token SHA-256 hashed at rest; `HttpOnly; SameSite=Lax` cookie; 30-day TTL; revoke on logout |
| AuthZ | Role (reader/organizer) + record ownership; identity from session, never client `user_id` |
| SQL | Parameterized queries only |
| Uploads | 50 MiB books / 5 MiB cover cap, format allowlists, size/MIME validated, SHA-256 checksums |
| Paths | Generated file names; uploaded names never used as paths; traversal rejected on serve |
| API | Rate limit 30/min on login/register (in-memory); 1 MiB JSON body cap |
| Privacy | Private notes/progress/reviews only under `/me/*`; catalog endpoints never join personal tables; service worker never caches `/me/*`, `/auth/*`, `/users/*` |
| Backups | Manifest + SHA-256 per file + `integrity_check`; restore keeps a safety copy |

Milestone F additions: password reset (single-use 1 h, no enumeration,
sessions invalidated), email verification (single-use 72 h, never gates
login), HSTS/nosniff/frame headers, direct TLS via `TLS_CERT_FILE`/
`TLS_KEY_FILE`, or `COOKIE_SECURE=true` behind a TLS proxy. No SMTP in v1:
reset/verify links go to the server log for local/dev only.

Still deferred: CSRF tokens for cookie auth (SameSite=Lax mitigates in v1),
per-user rate-limit store for multi-host cloud.
