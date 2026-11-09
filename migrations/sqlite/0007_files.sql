-- KutubCore digital files + locations (Milestone D, Blueprint §3 module 12,
-- §6, §8). Metadata in DB, bytes on filesystem; cover_image_id reuses the
-- book_files row for the cover (no separate covers table).
-- A saved URL does NOT mean downloaded/available (rule 10): is_available
-- distinguishes downloaded vs external link vs unavailable.

CREATE TABLE IF NOT EXISTS book_files (
    id               TEXT PRIMARY KEY,
    book_id          TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    edition_id       TEXT REFERENCES editions(id) ON DELETE SET NULL,
    file_name        TEXT NOT NULL,
    file_format      TEXT NOT NULL,
    file_size        INTEGER,
    checksum_sha256  TEXT,
    mime_type        TEXT,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CHECK (file_format IN ('paperback','hardcover','ebook','pdf','epub','audiobook','other','cover')),
    CHECK (file_size IS NULL OR file_size >= 0)
);
CREATE INDEX IF NOT EXISTS idx_book_files_book ON book_files(book_id);
CREATE INDEX IF NOT EXISTS idx_book_files_edition ON book_files(edition_id);
CREATE INDEX IF NOT EXISTS idx_book_files_checksum ON book_files(checksum_sha256);

CREATE TABLE IF NOT EXISTS file_locations (
    id            TEXT PRIMARY KEY,
    book_file_id  TEXT NOT NULL REFERENCES book_files(id) ON DELETE CASCADE,
    location_type TEXT NOT NULL,
    storage_path  TEXT,
    external_url  TEXT,
    is_available  INTEGER NOT NULL DEFAULT 1,
    CHECK (location_type IN ('local','external_url','network')),
    CHECK (is_available IN (0,1))
);
CREATE INDEX IF NOT EXISTS idx_file_locations_file ON file_locations(book_file_id);

UPDATE app_meta SET value = '7' WHERE key = 'schema_version';
