-- KutubCore FTS5 search (Milestone E, Blueprint §9/§20: SQLite FTS5 initially).
-- books_fts indexes title/subtitle/description/original_title/writers and is
-- maintained by triggers so it never goes stale. book_id is UNINDEXED (map
-- back to the UUID primary key). Tokenizer: unicode61 + diacritic folding, so
-- Bengali and other languages tokenize as Unicode words.

CREATE VIRTUAL TABLE IF NOT EXISTS books_fts USING fts5(
    book_id UNINDEXED,
    title,
    subtitle,
    description,
    original_title,
    writers,
    tokenize = 'unicode61 remove_diacritics 1'
);

-- Backfill existing (non-deleted) books.
INSERT INTO books_fts(rowid, book_id, title, subtitle, description, original_title, writers)
    SELECT b.rowid, b.id, b.title, COALESCE(b.subtitle, ''), COALESCE(b.description, ''),
           COALESCE(b.original_title, ''), ''
    FROM books b WHERE b.deleted_at IS NULL;

-- Populate writers column from book_writers.
UPDATE books_fts SET writers = COALESCE((
    SELECT group_concat(w.full_name, ' ') FROM book_writers bw
    JOIN writers w ON w.id = bw.writer_id
    WHERE bw.book_id = books_fts.book_id AND w.deleted_at IS NULL
), '');

-- ── triggers: books ───────────────────────────────────────────────────────
CREATE TRIGGER IF NOT EXISTS trg_books_fts_ai AFTER INSERT ON books BEGIN
    INSERT INTO books_fts(rowid, book_id, title, subtitle, description, original_title, writers)
    VALUES (new.rowid, new.id, new.title, COALESCE(new.subtitle, ''), COALESCE(new.description, ''),
            COALESCE(new.original_title, ''), '');
END;

CREATE TRIGGER IF NOT EXISTS trg_books_fts_ad AFTER DELETE ON books BEGIN
    DELETE FROM books_fts WHERE book_id = old.id;
END;

-- Soft delete also removes from search (deleted_at set / cleared).
CREATE TRIGGER IF NOT EXISTS trg_books_fts_au AFTER UPDATE OF deleted_at ON books BEGIN
    DELETE FROM books_fts WHERE book_id = old.id AND new.deleted_at IS NOT NULL;
    INSERT INTO books_fts(rowid, book_id, title, subtitle, description, original_title, writers)
    SELECT new.rowid, new.id, new.title, COALESCE(new.subtitle, ''), COALESCE(new.description, ''),
           COALESCE(new.original_title, ''), ''
    WHERE new.deleted_at IS NULL;
END;

CREATE TRIGGER IF NOT EXISTS trg_books_fts_au2 AFTER UPDATE OF title, subtitle, description, original_title ON books BEGIN
    UPDATE books_fts SET title = new.title, subtitle = COALESCE(new.subtitle, ''),
        description = COALESCE(new.description, ''), original_title = COALESCE(new.original_title, '')
        WHERE book_id = new.id AND new.deleted_at IS NULL;
END;

-- ── triggers: writers column (book_writers changes) ───────────────────────
CREATE TRIGGER IF NOT EXISTS trg_bw_fts_ai AFTER INSERT ON book_writers BEGIN
    UPDATE books_fts SET writers = COALESCE((
        SELECT group_concat(w.full_name, ' ') FROM book_writers bw
        JOIN writers w ON w.id = bw.writer_id
        WHERE bw.book_id = new.book_id AND w.deleted_at IS NULL
    ), '') WHERE book_id = new.book_id;
END;

CREATE TRIGGER IF NOT EXISTS trg_bw_fts_ad AFTER DELETE ON book_writers BEGIN
    UPDATE books_fts SET writers = COALESCE((
        SELECT group_concat(w.full_name, ' ') FROM book_writers bw
        JOIN writers w ON w.id = bw.writer_id
        WHERE bw.book_id = old.book_id AND w.deleted_at IS NULL
    ), '') WHERE book_id = old.book_id;
END;

UPDATE app_meta SET value = '8' WHERE key = 'schema_version';
