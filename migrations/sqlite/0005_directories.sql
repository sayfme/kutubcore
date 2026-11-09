-- KutubCore bookshops + collection sources (Milestone D, Blueprint §3
-- modules 6-7, §6). Catalog-level (public), organized by Organizers,
-- written with audit + soft delete like books/writers.

CREATE TABLE IF NOT EXISTS bookshops (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    shop_type   TEXT NOT NULL DEFAULT 'both',
    address     TEXT,
    city        TEXT,
    country     TEXT,
    phone       TEXT,
    email       TEXT,
    website     TEXT,
    notes       TEXT,
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    deleted_at  TEXT,
    CHECK (shop_type IN ('physical','online','both'))
);
CREATE INDEX IF NOT EXISTS idx_bookshops_name ON bookshops(name);
CREATE INDEX IF NOT EXISTS idx_bookshops_deleted ON bookshops(deleted_at);

CREATE TABLE IF NOT EXISTS bookshop_links (
    id          TEXT PRIMARY KEY,
    bookshop_id TEXT NOT NULL REFERENCES bookshops(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    url         TEXT NOT NULL,
    is_primary  INTEGER NOT NULL DEFAULT 0,
    UNIQUE (bookshop_id, url)
);
CREATE INDEX IF NOT EXISTS idx_bookshop_links_shop ON bookshop_links(bookshop_id);

CREATE TABLE IF NOT EXISTS collection_sources (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    source_type TEXT NOT NULL,
    description TEXT,
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    deleted_at  TEXT,
    CHECK (source_type IN ('bookshop','library','gift','friend','publisher','online_repository','other'))
);
CREATE INDEX IF NOT EXISTS idx_sources_name ON collection_sources(name);
CREATE INDEX IF NOT EXISTS idx_sources_type ON collection_sources(source_type);
CREATE INDEX IF NOT EXISTS idx_sources_deleted ON collection_sources(deleted_at);

CREATE TABLE IF NOT EXISTS source_links (
    id        TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES collection_sources(id) ON DELETE CASCADE,
    label     TEXT NOT NULL,
    url       TEXT NOT NULL,
    UNIQUE (source_id, url)
);
CREATE INDEX IF NOT EXISTS idx_source_links_source ON source_links(source_id);

UPDATE app_meta SET value = '5' WHERE key = 'schema_version';
