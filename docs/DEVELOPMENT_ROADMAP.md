# DEVELOPMENT_ROADMAP — KutubCore

| Milestone | Scope | Status |
|---|---|---|
| A — Foundation | Go skeleton, config, SQLite, migrations, health, PWA shell | ✅ |
| B — Catalog | Users/roles/auth, books/editions/writers, audit, duplicates | ✅ |
| C — Reader | Personal library, statuses, progress/sessions, journal, dashboard | ✅ |
| D — Directories+Files | Bookshops/sources, acquisitions/copies (rules 1–2), uploads/downloads | ✅ |
| E — Offline+Polish | FTS5 search, SW offline reads, backup/restore, seed, docs | ✅ |
| F — Hardening | Password reset, email verification, HTTPS/secure cookies, HSTS, E2E suite, CI | ✅ |
| G — LAN | LAN deployment guide, mDNS note, LAN IP logging + HTTP warning | ✅ (this milestone) |
| H — Cloud | Postgres adapter, Neon, sync queue + conflict UI | later |
| I — Analytics+Fun | Analytics (pages/month), reading goals, Bookshelf Life stages | ✅ (Milestone G) |
| J — Extras | Recommendations, import/export (CSV/JSON) | later |

MVP acceptance (Blueprint §18): all 13 criteria are met in A–E except
"authors/tests for backup/restore" (E delivers CLI backup/restore + tests)
and "offline test suite" (SW covered by route/unit tests; full E2E suite is
Milestone F).
