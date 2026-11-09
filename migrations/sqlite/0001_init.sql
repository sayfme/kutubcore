-- KutubCore foundation migration (Milestone 1).
-- Domain tables (users, books, editions, ...) arrive in Milestone 2+.
-- Keep this file idempotent: the migrator records it in schema_migrations
-- and never runs it twice.

CREATE TABLE IF NOT EXISTS app_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO app_meta(key, value) VALUES ('schema_version', '1');
INSERT OR IGNORE INTO app_meta(key, value) VALUES ('product', 'KutubCore');
