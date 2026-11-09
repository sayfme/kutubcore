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
  if (el) el.textContent = navigator.onLine ? "Online" : "Offline — cached catalog";
  document.body.classList.toggle("offline", !navigator.onLine);
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

// Deterministic spine colour from the title (stable per book).
const SPINES = ["var(--spine-1)", "var(--spine-2)", "var(--spine-3)", "var(--spine-4)", "var(--spine-5)"];
function spineFor(title) {
  let h = 0;
  for (const c of title) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return SPINES[h % SPINES.length];
}
function initials(title) {
  return (title || "?").trim().split(/\s+/).map((w) => w[0]).slice(0, 2).join("").toUpperCase() || "?";
}

function setVisible(id, show) {
  const el = document.querySelector(id);
  if (el) el.style.display = show ? "" : "none";
}

async function refreshWho() {
  const { res, json } = await api("GET", "/api/v1/auth/me");
  const navDir = document.querySelector("#nav-directories");
  if (res.ok && json?.data) {
    const u = json.data;
    const isOrg = !!u.roles?.includes("organizer");
    const org = isOrg ? " • Organizer" : "";
    who.textContent = `Signed in as ${u.display_name || u.email || u.id}${org}`;
    $("#avatar").textContent = initials(u.display_name || u.email || "?");
    // Dashboard view: replace the sign-in forms with the account view.
    $("#auth-form").style.display = "none";
    $("#account-view").style.display = "";
    $("#signin-h").innerHTML = '<img class="ic" src="/static/icons/signin.svg" alt="">Your account';
    $("#signin-note").style.display = "none";
    $("#logout-btn").hidden = false;
    $("#organizer-btn").hidden = isOrg;
    // Signed-in readers get their private sections (library, reading).
    setVisible("#library", true);
    setVisible("#acquisitions", true);
    setVisible("#analytics", true);
    // Blueprint §11: organizer workspace shows only for organizers.
    for (const id of ["#directories", "#files", "#create-section"]) setVisible(id, isOrg);
    if (navDir) navDir.style.display = isOrg ? "" : "none";
  } else {
    who.textContent = "Browsing as guest. Sign in to manage your library.";
    $("#avatar").textContent = "?";
    $("#auth-form").style.display = "";
    $("#account-view").style.display = "none";
    $("#signin-h").innerHTML = '<img class="ic" src="/static/icons/signin.svg" alt="">Sign in';
    $("#signin-note").style.display = "";
    $("#logout-btn").hidden = true;
    $("#organizer-btn").hidden = true;
    // Personal sections need a login — hide them from guests entirely.
    for (const id of ["#library", "#acquisitions", "#analytics", "#directories", "#files", "#create-section"]) setVisible(id, false);
    if (navDir) navDir.style.display = "none";
  }
}

async function listBooks(q = "") {
  const path = q ? `/api/v1/books?q=${encodeURIComponent(q)}` : "/api/v1/books";
  const { json } = await api("GET", path);
  const ul = $("#books");
  ul.innerHTML = "";
  for (const b of json?.data ?? []) {
    const li = document.createElement("li");
    li.className = "col-12 col-md-6 col-lg-4";
    li.style.listStyle = "none";
    const inner = document.createElement("div");
    inner.className = "card h-100 shadow-sm";
    const body = document.createElement("div");
    body.className = "card-body d-flex gap-3";
    const cover = document.createElement("div");
    cover.className = "cover";
    cover.style.background = spineFor(b.title);
    cover.textContent = initials(b.title);
    cover.setAttribute("aria-hidden", "true");
    const box = document.createElement("div");
    const title = document.createElement("h4");
    title.textContent = b.title;
    const by = document.createElement("p");
    by.className = "by";
    by.textContent = (b.writers ?? []).map((w) => w.name).join(", ") || "";
    const blurb = document.createElement("p");
    blurb.className = "blurb";
    blurb.textContent = (b.description ?? "").slice(0, 140);
    box.append(title, by);
    if (blurb.textContent) box.append(blurb);
    if ((b.genres ?? []).length) {
      const chips = document.createElement("div");
      chips.className = "chips";
      for (const g of b.genres.slice(0, 3)) {
        const c = document.createElement("span");
        c.className = "chip";
        c.textContent = g.name;
        chips.append(c);
      }
      box.append(chips);
    }
    const addBtn = document.createElement("button");
    addBtn.type = "button";
    addBtn.className = "btn-ghost";
    addBtn.textContent = "Add ID";
    addBtn.title = "Copy this book's ID into the 'Add to my library' form";
    addBtn.addEventListener("click", () => {
      const input = document.querySelector("#add-book-form [name=book_id]");
      if (input) { input.value = b.id; input.focus(); }
      document.querySelector("#library")?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
    box.append(addBtn);
    body.append(cover, box);
    inner.append(body);
    li.append(inner);
    ul.appendChild(li);
  }
  if (!(json?.data ?? []).length) ul.innerHTML = '<li class="col-12">No books yet. Organizers can add the first ones.</li>';
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
  delete payload.identifier; // strict server decoding rejects unknown fields
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
    const head = document.createElement("strong");
    head.textContent = b.book_title;
    const badges = document.createElement("span");
    badges.className = "badges";
    for (const s of [b.physical_status, b.digital_status, b.reading_status]) {
      const bd = document.createElement("span");
      bd.className = "badge badge-soft";
      bd.textContent = s.replaceAll("_", " ");
      badges.append(bd);
    }
    const fav = document.createElement("span");
    fav.className = "badge badge-fav";
    fav.textContent = "★ favorite";
    badges.append(fav);
    li.append(head, badges);
    if (b.progress && b.progress.total_pages) {
      const p = document.createElement("div");
      p.style.color = "var(--muted)";
      p.style.fontSize = ".85rem";
      p.textContent = `Page ${b.progress.current_page} of ${b.progress.total_pages} (${Math.round(b.progress.progress_percent)}%)`;
      li.append(p);
    }
    if (b.review?.review_text) {
      const q = document.createElement("div");
      q.style.fontSize = ".85rem";
      q.style.color = "var(--muted)";
      q.textContent = `“${b.review.review_text}”`;
      li.append(q);
    }
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

// ── Analytics & Goals & Bookshelf (Milestone G) ──────────────────────────
// Cosmetic only: stages never change book or reading data (Blueprint §12).
async function refreshAnalytics() {
  const a = await api("GET", "/api/v1/me/analytics?months=6");
  const el = document.querySelector("#analytics");
  const months = a.json?.data?.months ?? [];
  if (months.length) {
    const max = Math.max(...months.map((m) => m.pages_read), 1);
    el.innerHTML = months.map((m) => {
      const pct = Math.round((m.pages_read / max) * 100);
      return `<p><strong>${m.month}</strong> — ${m.pages_read} pages, ${m.sessions} sessions, ${m.books_done} done
        <span class="bar"><span style="width:${pct}%"></span></span></p>`;
    }).join("");
  } else {
    el.innerHTML = "<p>No reading data yet.</p>";
  }

  const g = await api("GET", "/api/v1/me/goals");
  const gd = g.json?.data;
  if (gd?.period_month) {
    document.querySelector("#goal-status").textContent =
      `${gd.period_month}: ${gd.books_done ?? 0} / ${gd.books_target} books done`;
  }

  const shelf = await api("GET", "/api/v1/me/shelf");
  const entries = shelf.json?.data?.entries ?? [];
  const ul = document.querySelector("#shelf");
  ul.innerHTML = entries.map((e) => {
    const icon = e.stage === "completed" ? "⭐"
      : e.stage === "heavy" ? "🪱"
      : e.stage === "bookworm" ? "🐛"
      : e.stage === "cobweb" ? "🕸️"
      : e.stage === "dust" ? "🧹"
      : "📚";
    return `<li>${icon} ${e.book_title} — ${e.label}${e.days_inactive ? ` (${e.days_inactive}d)` : ""}</li>`;
  }).join("") || "<li>Empty shelf.</li>";
}

document.querySelector("#goal-form")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const { res, json } = await api("PUT", "/api/v1/me/goals", { books_target: parseInt(fd.get("books_target"), 10) });
  document.querySelector("#goal-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) refreshAnalytics();
});

document.querySelector("#shelf-settings-form")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  const { res, json } = await api("PUT", "/api/v1/me/shelf/settings", {
    enabled: fd.get("enabled") === "on",
    dust_days: parseInt(fd.get("dust_days"), 10),
    cobweb_days: parseInt(fd.get("cobweb_days"), 10),
    worm_days: parseInt(fd.get("worm_days"), 10),
  });
  document.querySelector("#shelf-err").textContent = res.ok ? "" : (json?.error?.message || "Failed");
  if (res.ok) refreshAnalytics();
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

refreshWho().then(() => { refreshLibrary(); refreshDirectories(); refreshAnalytics(); });
listBooks();
