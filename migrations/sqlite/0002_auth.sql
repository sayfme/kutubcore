-- KutubCore identity + sessions (Milestone B, Blueprint §6).
-- Roles are records linked to users, not separate account types (§4).

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL,
    email         TEXT,
    mobile        TEXT,
    password_hash TEXT,
    is_active     INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Partial unique indexes: only when identifier present (Blueprint §7).
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email
    ON users(email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_mobile
    ON users(mobile) WHERE mobile IS NOT NULL;

CREATE TABLE IF NOT EXISTS roles (
    id    TEXT PRIMARY KEY,
    code  TEXT NOT NULL UNIQUE,
    name  TEXT NOT NULL
);

INSERT OR IGNORE INTO roles(id, code, name) VALUES
    ('00000000-0000-0000-0000-000000000001', 'reader', 'Reader'),
    ('00000000-0000-0000-0000-000000000002', 'organizer', 'Organizer');

CREATE TABLE IF NOT EXISTS user_roles (
    user_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id  TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    user_agent  TEXT,
    ip          TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

UPDATE app_meta SET value = '2' WHERE key = 'schema_version';
