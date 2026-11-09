// KutubCore shell: auth + catalog + library. PWA offline reads are
// handled by /sw.js (see web/static/sw.js). No framework, no bundler.

// PWA service worker: app shell cache-first, public catalog
// network-first with cache fallback. Personal /me/* is never cached.
if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {});
  });
}

// Online/offline indicator (Blueprint §10: connection status).
const onlineEl = () => document.querySelector("#online-status");
function setOnline() {
  const el = onlineEl();
  if (el) el.textContent = navigator.onLine ? "Online" : "Offline — showing cached catalog";
}
window.addEventListener("online", setOnline);
window.addEventListener("offline", setOnline);
setOnline();
const $ = (s) => document.querySelector(s);
const who = $("#who");

async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json" },
    credentials: "same-origin",
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json = null;
  try { json = text ? JSON.parse(text) : null; } catch { json = null; }
  return { res, json };
}

async function refreshWho() {
  const { res, json } = await api("GET", "/api/v1/auth/me");
  if (res.ok && json?.data) {
    const u = json.data;
    const org = u.roles?.includes("organizer") ? " [Organizer]" : "";
    who.textContent = `Signed in as ${u.display_name || u.email || u.id}${org}`;
    $("#logout-btn").hidden = false;
    $("#organizer-btn").hidden = !!u.roles?.includes("organizer");
    $("#create-section").dataset.organizer = u.roles?.includes("organizer") ? "1" : "";
  } else {
    who.textContent = "Browsing as guest. Sign in to manage your library.";
    $("#logout-btn").hidden = true;
    $("#organizer-btn").hidden = true;
  }
}

async function listBooks(q = "") {
  const path = q ? `/api/v1/books?q=${encodeURIComponent(q)}` : "/api/v1/books";
  const { json } = await api("GET", path);
  const ul = $("#books");
  ul.innerHTML = "";
  for (const b of json?.data ?? []) {
    const li = document.createElement("li");
    const writers = (b.writers ?? []).map((w) => w.name).join(", ");
    li.textContent = b.title + (writers ? ` — ${writers}` : "");
    ul.appendChild(li);
  }
  if (!(json?.data ?? []).length) ul.innerHTML = "<li>No books yet.</li>";
}

$("#search-form").addEventListener("submit", (e) => {
  e.preventDefault();
  listBooks(new FormData(e.target).get("q") || "");
});

$("#auth-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const identifier = (fd.get("identifier") || "").trim();
  const payload = identifier.includes("@")
    ? { email: identifier, identifier, password: fd.get("password") }
    : { mobile: identifier, identifier, password: fd.get("password") };
  const { res, json } = await api("POST", "/api/v1/auth/login", payload);
  $("#auth-err").textContent = res.ok ? "" : (json?.error?.message || "Sign-in failed");
  await refreshWho();
});

$("#register-btn").addEventListener("click", async () => {
  const fd = new FormData($("#auth-form"));
  const identifier = (fd.get("identifier") || "").trim();
  const payload = {
    display_name: fd.get("display_name") || identifier,
    email: identifier.includes("@") ? identifier : undefined,
    mobile: identifier.includes("@") ? undefined : identifier,
    identifier,
    password: fd.get("password"),
  };
  const { res, json } = await api("POST", "/api/v1/auth/register", payload);
  $("#auth-err").textContent = res.ok ? "" : (json?.error?.message || "Register failed");
  await refreshWho();
});

$("#organizer-btn").addEventListener("click", async () => {
  await api("POST", "/api/v1/users/me/roles/organizer", {});
  await refreshWho();
});

$("#logout-btn").addEventListener("click", async () => {
  await api("POST", "/api/v1/auth/logout", {});
  await refreshWho();
});

$("#writer-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const { res, json } = await api("POST", "/api/v1/writers", { full_name: fd.get("full_name") });
  $("#writer-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) e.target.reset();
});

$("#book-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const writerId = (fd.get("writer_id") || "").trim();
  const payload = { title: fd.get("title") };
  if (writerId) payload.writers = [{ id: writerId, role: "author" }];
  const { res, json } = await api("POST", "/api/v1/books", payload);
  $("#book-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) { e.target.reset(); listBooks(); }
});

// ── My Collection (Milestone C) ────────────────────────────────────────────
async function refreshLibrary() {
  const { res, json } = await api("GET", "/api/v1/me/dashboard");
  if (res.ok && json?.data) {
    const d = json.data;
    $("#dash").textContent =
      `Library: ${d.total_books} · Owned: ${d.physical_owned} · Downloaded: ${d.digital_downloaded} · ` +
      `Reading: ${d.currently_reading} · Done: ${d.completed} · Want: ${d.want_to_read} · Fav: ${d.favorites}`;
  }
  const lib = await api("GET", "/api/v1/me/books");
  const ul = $("#my-books");
  ul.innerHTML = "";
  for (const b of lib.json?.data ?? []) {
    const li = document.createElement("li");
    li.textContent = `${b.book_title} · ${b.physical_status}/${b.digital_status} · ${b.reading_status}${b.is_favorite ? " ★" : ""}`;
    ul.appendChild(li);
  }
  if (!(lib.json?.data ?? []).length) ul.innerHTML = "<li>Your library is empty. Add a catalog book by ID.</li>";
}

$("#add-book-form")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const payload = {
    book_id: fd.get("book_id"),
    physical_status: fd.get("physical_status"),
    reading_status: fd.get("reading_status"),
    is_favorite: fd.get("is_favorite") === "on",
  };
  const { res, json } = await api("POST", "/api/v1/me/books", payload);
  $("#add-book-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) { e.target.reset(); refreshLibrary(); }
});

async function refreshDirectories() {
  const shops = await api("GET", "/api/v1/bookshops");
  const sources = await api("GET", "/api/v1/sources");
  $("#shops").innerHTML = (shops.json?.data ?? []).map((s) => `<li>${s.name} (${s.shop_type})</li>`).join("") || "<li>No bookshops yet.</li>";
  $("#sources").innerHTML = (sources.json?.data ?? []).map((s) => `<li>${s.name} (${s.source_type})</li>`).join("") || "<li>No sources yet.</li>";
}

$("#shop-form")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const { res, json } = await api("POST", "/api/v1/bookshops", { name: fd.get("name"), shop_type: fd.get("shop_type") });
  $("#shop-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) { e.target.reset(); refreshDirectories(); }
});

$("#source-form")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const { res, json } = await api("POST", "/api/v1/sources", { name: fd.get("name"), source_type: fd.get("source_type") });
  $("#source-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) { e.target.reset(); refreshDirectories(); }
});

$("#acq-form")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const total = parseFloat(fd.get("total_cost"));
  const payload = {
    bookshop_id: fd.get("bookshop_id") || undefined,
    acquisition_date: fd.get("acquisition_date") || undefined,
    total_cost: Number.isFinite(total) ? total : undefined,
    currency_code: (fd.get("currency_code") || "").toUpperCase() || undefined,
    items: [{ book_id: fd.get("book_id"), format: "paperback", quantity: 1 }],
  };
  const { res, json } = await api("POST", "/api/v1/me/acquisitions", payload);
  $("#acq-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) { e.target.reset(); refreshLibrary(); }
});

$("#upload-form")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const bookId = fd.get("book_id");
  const res = await fetch(`/api/v1/books/${bookId}/files`, {
    method: "POST",
    credentials: "same-origin",
    body: fd,
  });
  const json = await res.json().catch(() => null);
  $("#upload-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) e.target.reset();
});

refreshWho().then(() => { refreshLibrary(); refreshDirectories(); });
listBooks();
