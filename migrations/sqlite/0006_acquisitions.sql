-- KutubCore acquisitions + copies (Milestone D, Blueprint §3 module 8, §6, §7).
-- Acquisition history is INDEPENDENT from current collection status (rule 7):
-- recording a purchase never rewrites user_books by itself.
-- One acquisition may contain many items; each unit becomes a book_copies row
-- (rule 1: owned requires >=1 copy).

CREATE TABLE IF NOT EXISTS acquisitions (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id         TEXT REFERENCES collection_sources(id) ON DELETE SET NULL,
    bookshop_id       TEXT REFERENCES bookshops(id) ON DELETE SET NULL,
    acquisition_date  TEXT,
    total_cost        REAL,
    currency_code     TEXT,
    notes             TEXT,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CHECK (total_cost IS NULL OR total_cost >= 0),
    CHECK (currency_code IS NULL OR length(currency_code) = 3)
);
CREATE INDEX IF NOT EXISTS idx_acquisitions_user ON acquisitions(user_id);
CREATE INDEX IF NOT EXISTS idx_acquisitions_user_date ON acquisitions(user_id, acquisition_date);

CREATE TABLE IF NOT EXISTS acquisition_items (
    id           TEXT PRIMARY KEY,
    acquisition_id TEXT NOT NULL REFERENCES acquisitions(id) ON DELETE CASCADE,
    book_id      TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    edition_id   TEXT REFERENCES editions(id) ON DELETE SET NULL,
    quantity     INTEGER NOT NULL DEFAULT 1,
    unit_price   REAL,
    format       TEXT NOT NULL,
    CHECK (quantity >= 1),
    CHECK (unit_price IS NULL OR unit_price >= 0),
    CHECK (format IN ('paperback','hardcover','ebook','pdf','epub','audiobook','other'))
);
CREATE INDEX IF NOT EXISTS idx_acq_items_acq ON acquisition_items(acquisition_id);
CREATE INDEX IF NOT EXISTS idx_acq_items_book ON acquisition_items(book_id);

CREATE TABLE IF NOT EXISTS book_copies (
    id                  TEXT PRIMARY KEY,
    acquisition_item_id TEXT NOT NULL REFERENCES acquisition_items(id) ON DELETE CASCADE,
    edition_id          TEXT REFERENCES editions(id) ON DELETE SET NULL,
    format              TEXT NOT NULL,
    condition           TEXT,
    storage_location    TEXT,
    acquired_at         TEXT,
    notes               TEXT,
    CHECK (format IN ('paperback','hardcover','ebook','pdf','epub','audiobook','other')),
    CHECK (condition IS NULL OR condition IN ('new','good','fair','damaged'))
);
CREATE INDEX IF NOT EXISTS idx_copies_item ON book_copies(acquisition_item_id);
CREATE INDEX IF NOT EXISTS idx_copies_edition ON book_copies(edition_id);

UPDATE app_meta SET value = '6' WHERE key = 'schema_version';
