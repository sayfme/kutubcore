/* KutubCore service worker (Milestone E, Blueprint §10).
 * Strategies:
 *  - App shell + static assets: cache-first (precached at install).
 *  - Public catalog API GETs: network-first, cache fallback (offline reads).
 *  - NEVER cache: /api/v1/me/*, /api/v1/auth/*, /api/v1/users/* personal data,
 *    and all non-GET requests (Blueprint §10: keep sensitive data out of caches).
 */
const CACHE = "kutubcore-v1";
const SHELL = [
  "/",
  "/static/css/app.css",
  "/static/js/app.js",
  "/static/manifest.webmanifest",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(SHELL)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

function isPublicCatalogGet(url) {
  if (url.pathname === "/api/v1/books" ||
      url.pathname.startsWith("/api/v1/books/") ||
      url.pathname === "/api/v1/writers" ||
      url.pathname.startsWith("/api/v1/writers/") ||
      url.pathname === "/api/v1/languages" ||
      url.pathname === "/api/v1/genres" ||
      url.pathname === "/api/v1/bookshops" ||
      url.pathname.startsWith("/api/v1/bookshops/") ||
      url.pathname === "/api/v1/sources" ||
      url.pathname.startsWith("/api/v1/sources/")) {
    return true;
  }
  return url.pathname.startsWith("/static/") || url.pathname === "/";
}

function isSensitive(url) {
  return url.pathname.startsWith("/api/v1/me/") ||
         url.pathname.startsWith("/api/v1/auth/") ||
         url.pathname.startsWith("/api/v1/users/");
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") return; // never intercept writes
  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return; // same-origin only
  if (isSensitive(url)) return; // personal data: always network, never cached

  // Static + shell: cache-first.
  if (url.pathname.startsWith("/static/") || url.pathname === "/") {
    event.respondWith(
      caches.match(request).then((cached) => {
        if (cached) return cached;
        return fetch(request).then((res) => {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(request, copy));
          return res;
        });
      })
    );
    return;
  }

  // Public catalog API: network-first with cache fallback (offline reads).
  if (isPublicCatalogGet(url)) {
    event.respondWith(
      fetch(request).then((res) => {
        if (res.ok) {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(request, copy));
        }
        return res;
      }).catch(() => caches.match(request))
    );
  }
});
