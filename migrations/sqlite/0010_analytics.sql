-- KutubCore analytics + goals + Bookshelf Life (Milestone G, Blueprint §3
-- module 13 + §12). All local-only; guard: cosmetic features never change
-- book or reading data.

CREATE TABLE IF NOT EXISTS reading_goals (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period_month   TEXT NOT NULL, -- 'YYYY-MM' in UTC
    books_target   INTEGER NOT NULL,
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (user_id, period_month),
    CHECK (books_target >= 1)
);
CREATE INDEX IF NOT EXISTS idx_goals_user ON reading_goals(user_id);
CREATE INDEX IF NOT EXISTS idx_goals_period ON reading_goals(period_month);

-- Per-user Bookshelf Life cosmetic settings (Blueprint §12).
CREATE TABLE IF NOT EXISTS shelf_settings (
    user_id      TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled      INTEGER NOT NULL DEFAULT 1,
    dust_days    INTEGER NOT NULL DEFAULT 30,
    cobweb_days  INTEGER NOT NULL DEFAULT 60,
    worm_days    INTEGER NOT NULL DEFAULT 90,
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CHECK (dust_days >= 1),
    CHECK (cobweb_days > dust_days),
    CHECK (worm_days > cobweb_days)
);

UPDATE app_meta SET value = '10' WHERE key = 'schema_version';
