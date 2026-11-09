-- KutubCore personal library + reading tracker + journal
-- (Milestone C, Blueprint §3 modules 9-11, §5, §6, §7).
-- user_books keeps physical/digital/reading independent:
--   owned != downloaded != started reading (§5, rule 6).

CREATE TABLE IF NOT EXISTS user_books (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id          TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    physical_status  TEXT NOT NULL DEFAULT 'not_collected',
    digital_status   TEXT NOT NULL DEFAULT 'not_available',
    reading_status   TEXT NOT NULL DEFAULT 'want_to_read',
    is_favorite      INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (user_id, book_id),
    CHECK (physical_status IN ('not_collected','want_to_buy','owned')),
    CHECK (digital_status IN ('not_available','want_to_download','downloaded')),
    CHECK (reading_status IN ('want_to_read','currently_reading','on_hold','completed','abandoned'))
);
CREATE INDEX IF NOT EXISTS idx_user_books_user ON user_books(user_id);
CREATE INDEX IF NOT EXISTS idx_user_books_user_book ON user_books(user_id, book_id);
CREATE INDEX IF NOT EXISTS idx_user_books_reading ON user_books(user_id, reading_status);
CREATE INDEX IF NOT EXISTS idx_user_books_physical ON user_books(user_id, physical_status);
CREATE INDEX IF NOT EXISTS idx_user_books_digital ON user_books(user_id, digital_status);
CREATE INDEX IF NOT EXISTS idx_user_books_favorite ON user_books(user_id, is_favorite);
CREATE INDEX IF NOT EXISTS idx_user_books_created ON user_books(user_id, created_at);

CREATE TABLE IF NOT EXISTS reading_progress (
    id               TEXT PRIMARY KEY,
    user_book_id     TEXT NOT NULL UNIQUE REFERENCES user_books(id) ON DELETE CASCADE,
    current_page     INTEGER,
    total_pages      INTEGER,
    progress_percent REAL NOT NULL DEFAULT 0,
    started_at       TEXT,
    completed_at     TEXT,
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CHECK (current_page IS NULL OR current_page >= 0),
    CHECK (total_pages IS NULL OR total_pages > 0),
    CHECK (progress_percent >= 0 AND progress_percent <= 100),
    CHECK (current_page IS NULL OR total_pages IS NULL OR current_page <= total_pages)
);

CREATE TABLE IF NOT EXISTS reading_sessions (
    id            TEXT PRIMARY KEY,
    user_book_id  TEXT NOT NULL REFERENCES user_books(id) ON DELETE CASCADE,
    started_at    TEXT NOT NULL,
    ended_at      TEXT,
    pages_read    INTEGER,
    reading_note  TEXT,
    CHECK (pages_read IS NULL OR pages_read >= 0),
    CHECK (ended_at IS NULL OR started_at IS NULL OR ended_at >= started_at)
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_book ON reading_sessions(user_book_id);
CREATE INDEX IF NOT EXISTS idx_sessions_started ON reading_sessions(started_at);

CREATE TABLE IF NOT EXISTS notes (
    id            TEXT PRIMARY KEY,
    user_book_id  TEXT NOT NULL REFERENCES user_books(id) ON DELETE CASCADE,
    title         TEXT,
    content       TEXT NOT NULL,
    page_number   INTEGER,
    visibility    TEXT NOT NULL DEFAULT 'private',
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CHECK (visibility IN ('private','public')),
    CHECK (page_number IS NULL OR page_number >= 0)
);
CREATE INDEX IF NOT EXISTS idx_notes_user_book ON notes(user_book_id);

CREATE TABLE IF NOT EXISTS reviews (
    id            TEXT PRIMARY KEY,
    user_book_id  TEXT NOT NULL UNIQUE REFERENCES user_books(id) ON DELETE CASCADE,
    rating        INTEGER,
    reaction      TEXT,
    review_text   TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CHECK (rating IS NULL OR (rating >= 1 AND rating <= 5)),
    CHECK (reaction IS NULL OR reaction IN ('favorite','average','meh','legend'))
);

UPDATE app_meta SET value = '4' WHERE key = 'schema_version';
