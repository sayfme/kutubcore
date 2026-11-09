# UI_SPECIFICATION — KutubCore (Blueprint §11)

Single PWA shell at `/` (vanilla JS, semantic HTML, one file per asset —
`web/static/{css/app.css,js/app.js,sw.js}` + `web/static/manifest.webmanifest`,
embedded into the binary so `go run` works without a static server).

## Screens (single scroll, sections)

1. Header: title, signed-in identity, online/offline status (`#online-status`).
2. Sign in: register / login / become-organizer / sign-out.
3. Search books: `q` input → FTS search results.
4. My Collection: add-by-ID (physical/reading/favorite), dashboard line, list,
   record acquisition.
5. Bookshops & Sources: create + list.
6. Digital Files: upload (PDF/EPUB) by book ID.
7. Catalog (Organizer): create writer / book.

## Design

Traditional library tone, clean reading-focused layout, light/dark via
`prefers-color-scheme` (CSS variables, focus-visible outlines, keyboard nav).
Destructive actions need confirmation (not yet wired in the shell; API
supports soft-delete).

## Offline (§10)

- Service worker precaches shell + static; network-first with cache fallback
  for public catalog GETs (`/books`, `/writers`, `/languages`, `/genres`,
  `/bookshops`, `/sources`), so the catalog keeps working offline.
- `/me/*`, `/auth/*`, `/users/*` are NEVER cached (private data).
- Online/offline status indicator in the header.
