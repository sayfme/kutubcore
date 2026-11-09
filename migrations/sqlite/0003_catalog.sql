-- KutubCore shared catalog (Milestone B, Blueprint §6).
-- Books (work) / Editions (published version) / BookWriters stay separate
-- (defining rule, Blueprint §20). Soft delete via deleted_at (§7).

-- ── languages ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS languages (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    code        TEXT NOT NULL UNIQUE,
    native_name TEXT,
    is_active   INTEGER NOT NULL DEFAULT 1
);

INSERT OR IGNORE INTO languages(id, name, code, native_name, is_active) VALUES
    ('00000000-0000-0000-0000-00000000ba01', 'Bengali', 'bn', 'বাংলা', 1),
    ('00000000-0000-0000-0000-00000000e001', 'English', 'en', 'English', 1),
    ('00000000-0000-0000-0000-00000000a001', 'Arabic', 'ar', 'العربية', 1),
    ('00000000-0000-0000-0000-00000000h001', 'Hindi', 'hi', 'हिन्दी', 1),
    ('00000000-0000-0000-0000-00000000u001', 'Urdu', 'ur', 'اردو', 1),
    ('00000000-0000-0000-0000-00000000f001', 'French', 'fr', 'Français', 1),
    ('00000000-0000-0000-0000-00000000j001', 'Japanese', 'ja', '日本語', 1),
    ('00000000-0000-0000-0000-00000000g001', 'German', 'de', 'Deutsch', 1);

-- ── genres (hierarchical; UI is flat in B, tree comes later) ──────────────
CREATE TABLE IF NOT EXISTS genres (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    parent_id   TEXT REFERENCES genres(id) ON DELETE SET NULL,
    description TEXT,
    is_active   INTEGER NOT NULL DEFAULT 1
);

INSERT OR IGNORE INTO genres(id, name, slug, is_active) VALUES
    ('00000000-0000-0000-0000-0000000fc001', 'Fiction', 'fiction', 1),
    ('00000000-0000-0000-0000-0000000fc002', 'Non-fiction', 'non-fiction', 1),
    ('00000000-0000-0000-0000-0000000fc003', 'Novel', 'novel', 1),
    ('00000000-0000-0000-0000-0000000fc004', 'Poetry', 'poetry', 1),
    ('00000000-0000-0000-0000-0000000fc005', 'Short story', 'short-story', 1),
    ('00000000-0000-0000-0000-0000000fc006', 'History', 'history', 1),
    ('00000000-0000-0000-0000-0000000fc007', 'Biography', 'biography', 1),
    ('00000000-0000-0000-0000-0000000fc008', 'Science', 'science', 1);

-- ── books (logical work) ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS books (
    id             TEXT PRIMARY KEY,
    title          TEXT NOT NULL,
    subtitle       TEXT,
    description    TEXT,
    original_title TEXT,
    cover_image_id TEXT,
    created_by     TEXT REFERENCES users(id) ON DELETE SET NULL,
    version        INTEGER NOT NULL DEFAULT 1,
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    deleted_at     TEXT
);
CREATE INDEX IF NOT EXISTS idx_books_title ON books(title);
CREATE INDEX IF NOT EXISTS idx_books_deleted ON books(deleted_at);
CREATE INDEX IF NOT EXISTS idx_books_created_by ON books(created_by);

-- ── editions (published version; edition-specific fields ONLY here) ────────
CREATE TABLE IF NOT EXISTS editions (
    id               TEXT PRIMARY KEY,
    book_id          TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    language_id      TEXT REFERENCES languages(id) ON DELETE SET NULL,
    publisher        TEXT,
    edition_number   TEXT,
    publication_year INTEGER,
    format           TEXT NOT NULL,
    page_count       INTEGER,
    version          INTEGER NOT NULL DEFAULT 1,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CHECK (page_count IS NULL OR page_count > 0),
    CHECK (publication_year IS NULL OR (publication_year >= 1000 AND publication_year <= 2200)),
    CHECK (format IN ('paperback','hardcover','ebook','pdf','epub','audiobook','other'))
);
CREATE INDEX IF NOT EXISTS idx_editions_book ON editions(book_id);
CREATE INDEX IF NOT EXISTS idx_editions_language ON editions(language_id);
CREATE INDEX IF NOT EXISTS idx_editions_year ON editions(publication_year);

-- ── identifiers (ISBN etc. — on editions, never on books) ──────────────────
CREATE TABLE IF NOT EXISTS book_identifiers (
    id               TEXT PRIMARY KEY,
    edition_id       TEXT NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    identifier_type  TEXT NOT NULL,
    identifier_value TEXT NOT NULL,
    UNIQUE (identifier_type, identifier_value)
);
CREATE INDEX IF NOT EXISTS idx_identifiers_edition ON book_identifiers(edition_id);

-- ── writers ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS writers (
    id          TEXT PRIMARY KEY,
    full_name   TEXT NOT NULL,
    biography   TEXT,
    birth_date  TEXT,
    death_date  TEXT,
    website     TEXT,
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    deleted_at  TEXT
);
CREATE INDEX IF NOT EXISTS idx_writers_name ON writers(full_name);
CREATE INDEX IF NOT EXISTS idx_writers_deleted ON writers(deleted_at);

CREATE TABLE IF NOT EXISTS book_writers (
    book_id          TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    writer_id        TEXT NOT NULL REFERENCES writers(id) ON DELETE CASCADE,
    contribution_role TEXT NOT NULL DEFAULT 'author',
    display_order    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (book_id, writer_id, contribution_role)
);
CREATE INDEX IF NOT EXISTS idx_book_writers_writer ON book_writers(writer_id);

-- ── book <-> genre ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS book_genres (
    book_id  TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    genre_id TEXT NOT NULL REFERENCES genres(id) ON DELETE CASCADE,
    PRIMARY KEY (book_id, genre_id)
);

-- ── audit log (catalog changes; §4/§14) ────────────────────────────────────
CREATE TABLE IF NOT EXISTS audit_logs (
    id            TEXT PRIMARY KEY,
    actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    entity_type   TEXT NOT NULL,
    entity_id     TEXT NOT NULL,
    action        TEXT NOT NULL,
    before_json   TEXT,
    after_json    TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);

UPDATE app_meta SET value = '3' WHERE key = 'schema_version';
