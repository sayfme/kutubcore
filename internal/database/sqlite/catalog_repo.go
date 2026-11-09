package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"kutubcore/internal/domain/book"
	"kutubcore/internal/domain/writer"
	"kutubcore/internal/repository"
)

var _ repository.BookRepository = (*CatalogRepo)(nil)

// CatalogRepo is the SQLite catalog repository (books, editions, writers,
// languages, genres, audit).
type CatalogRepo struct{ DB *sql.DB }

func NewCatalogRepo(db *sql.DB) *CatalogRepo { return &CatalogRepo{DB: db} }

// ── helpers ─────────────────────────────────────────────────────────────────

func offsetOf(p repository.Page) int {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	return (p.Page - 1) * p.Limit
}

// ftsMatch converts user input into a safe FTS5 prefix-AND query:
// each whitespace-separated term becomes a quoted "term"* token, so
// no FTS operator can be injected. Returns "" if no usable terms.
func ftsMatch(q string) string {
	terms := strings.Fields(q)
	var parts []string
	for _, t := range terms {
		t = strings.ReplaceAll(t, `"`, `""`)
		if t == "" {
			continue
		}
		parts = append(parts, `"`+t+`"*`)
	}
	return strings.Join(parts, " ")
}

func sortClause(s string) string {
	// Whitelist only — never interpolate user input.
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "title", "title_asc", "title_desc":
		if strings.HasSuffix(strings.ToLower(s), "desc") {
			return "title DESC"
		}
		return "title ASC"
	case "updated_at", "-updated_at", "updated_at_desc":
		return "updated_at DESC"
	case "created_at", "-created_at":
		return "created_at DESC"
	default:
		return "created_at DESC"
	}
}

// ── books ───────────────────────────────────────────────────────────────────

func (r *CatalogRepo) ListBooks(ctx context.Context, f repository.BookFilter, p repository.Page) ([]book.Book, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	// Base query. Language/genre/writer filters join through editions/genres.
	where := []string{"b.deleted_at IS NULL"}
	args := []any{}
	joins := ""

	if q := strings.TrimSpace(f.Query); q != "" {
		// Primary: FTS5 prefix-AND match (title/subtitle/desc/original/writers).
		// Fallback: legacy LIKE if the query yields no safe FTS terms.
		if match := ftsMatch(q); match != "" {
			joins += " JOIN books_fts fts ON fts.book_id = b.id "
			where = append(where, "books_fts MATCH ?")
			args = append(args, match)
		} else {
			where = append(where, `(b.title LIKE ? OR b.original_title LIKE ? OR EXISTS(SELECT 1 FROM book_writers bw JOIN writers w ON w.id=bw.writer_id WHERE bw.book_id=b.id AND (w.full_name LIKE ? OR w.deleted_at IS NOT NULL AND 0)))`)
			like := "%" + q + "%"
			args = append(args, like, like, like)
		}
	}
	if f.Language != "" {
		joins += ` JOIN editions e ON e.book_id=b.id
		 JOIN languages l ON l.id=e.language_id AND l.code=?`
		args = append(args, strings.ToLower(strings.TrimSpace(f.Language)))
	}
	if f.Genre != "" {
		joins += ` JOIN book_genres bg ON bg.book_id=b.id
		 JOIN genres g ON g.id=bg.genre_id AND g.slug=?`
		args = append(args, strings.ToLower(strings.TrimSpace(f.Genre)))
	}
	if f.WriterID != "" {
		joins += ` JOIN book_writers bw2 ON bw2.book_id=b.id AND bw2.writer_id=?`
		args = append(args, f.WriterID)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := fmt.Sprintf(`SELECT COUNT(DISTINCT b.id) FROM books b%s WHERE %s`, joins, whereSQL)
	if err := r.DB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSQL := fmt.Sprintf(
		`SELECT b.id, b.title, b.subtitle, b.description, b.original_title, b.created_by, b.version, b.created_at, b.updated_at
		 FROM books b%s WHERE %s ORDER BY %s LIMIT ? OFFSET ?`, joins, whereSQL, sortClause(f.Sort))
	listArgs := append(append([]any{}, args...), p.Limit, offsetOf(p))
	rows, err := r.DB.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}

	// IMPORTANT: collect first, close rows, THEN run nested queries.
	// SQLite uses a single writer connection (MaxOpenConns=1); querying
	// while rows are open deadlocks.
	var out []book.Book
	for rows.Next() {
		var (
			b                               book.Book
			subtitle, desc, orig, createdBy sql.NullString
		)
		if err := rows.Scan(&b.ID, &b.Title, &subtitle, &desc, &orig, &createdBy, &b.Version, &b.CreatedAt, &b.UpdatedAt); err != nil {
			rows.Close()
			return nil, 0, err
		}
		b.Subtitle = subtitle.String
		b.Description = desc.String
		b.OriginalTitle = orig.String
		b.CreatedBy = createdBy.String
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	rows.Close()

	for i := range out {
		if err := r.attachBookDetails(ctx, &out[i]); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *CatalogRepo) attachBookDetails(ctx context.Context, b *book.Book) error {
	// Close each rows set before opening the next (single-conn SQLite).
	// Writers.
	wRows, err := r.DB.QueryContext(ctx,
		`SELECT w.id, w.full_name, bw.contribution_role, bw.display_order
		 FROM book_writers bw JOIN writers w ON w.id=bw.writer_id
		 WHERE bw.book_id=? AND w.deleted_at IS NULL ORDER BY bw.display_order, w.full_name`, b.ID)
	if err != nil {
		return err
	}
	for wRows.Next() {
		var wr book.WriterRef
		if err := wRows.Scan(&wr.ID, &wr.Name, &wr.Role, &wr.Order); err != nil {
			wRows.Close()
			return err
		}
		b.Writers = append(b.Writers, wr)
	}
	if err := wRows.Err(); err != nil {
		wRows.Close()
		return err
	}
	wRows.Close()

	// Genres.
	gRows, err := r.DB.QueryContext(ctx,
		`SELECT g.id, g.name, g.slug FROM book_genres bg JOIN genres g ON g.id=bg.genre_id
		 WHERE bg.book_id=? ORDER BY g.name`, b.ID)
	if err != nil {
		return err
	}
	for gRows.Next() {
		var g book.GenreRef
		if err := gRows.Scan(&g.ID, &g.Name, &g.Slug); err != nil {
			gRows.Close()
			return err
		}
		b.Genres = append(b.Genres, g)
	}
	if err := gRows.Err(); err != nil {
		gRows.Close()
		return err
	}
	gRows.Close()
	return nil
}

func (r *CatalogRepo) GetBook(ctx context.Context, id string) (*book.Book, error) {
	var (
		b                               book.Book
		subtitle, desc, orig, createdBy sql.NullString
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, title, subtitle, description, original_title, created_by, version, created_at, updated_at
		 FROM books WHERE id=? AND deleted_at IS NULL`, id).
		Scan(&b.ID, &b.Title, &subtitle, &desc, &orig, &createdBy, &b.Version, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Subtitle = subtitle.String
	b.Description = desc.String
	b.OriginalTitle = orig.String
	b.CreatedBy = createdBy.String
	if err := r.attachBookDetails(ctx, &b); err != nil {
		return nil, err
	}
	editions, err := r.ListEditions(ctx, id)
	if err != nil {
		return nil, err
	}
	b.Editions = editions
	return &b, nil
}

func (r *CatalogRepo) CreateBook(ctx context.Context, b *book.Book) error {
	now := nowUTC()
	if b.Version < 1 {
		b.Version = 1
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO books(id, title, subtitle, description, original_title, created_by, version, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		b.ID, strings.TrimSpace(b.Title), nullStr(b.Subtitle), nullStr(b.Description), nullStr(b.OriginalTitle), nullStr(b.CreatedBy), b.Version, now, now)
	if err != nil {
		return err
	}
	return r.syncBookLinks(ctx, b)
}

func (r *CatalogRepo) UpdateBook(ctx context.Context, b *book.Book) error {
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx,
		`UPDATE books SET title=?, subtitle=?, description=?, original_title=?, version=version+1, updated_at=? WHERE id=? AND deleted_at IS NULL`,
		strings.TrimSpace(b.Title), nullStr(b.Subtitle), nullStr(b.Description), nullStr(b.OriginalTitle), now, b.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	b.Version++
	b.UpdatedAt = now
	return r.syncBookLinks(ctx, b)
}

func (r *CatalogRepo) SoftDeleteBook(ctx context.Context, id string) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE books SET deleted_at=?, version=version+1 WHERE id=? AND deleted_at IS NULL`, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *CatalogRepo) RestoreBook(ctx context.Context, id string) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE books SET deleted_at=NULL WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *CatalogRepo) syncBookLinks(ctx context.Context, b *book.Book) error {
	// Only reconcile links when the payload explicitly provided them
	// (non-nil). A PATCH without writers/genres must not delete links.
	if b.Writers != nil {
		if _, err := r.DB.ExecContext(ctx, `DELETE FROM book_writers WHERE book_id=?`, b.ID); err != nil {
			return err
		}
		for _, wr := range b.Writers {
			role := wr.Role
			if role == "" {
				role = "author"
			}
			if _, err := r.DB.ExecContext(ctx,
				`INSERT OR IGNORE INTO book_writers(book_id, writer_id, contribution_role, display_order) VALUES(?,?,?,?)`,
				b.ID, wr.ID, role, wr.Order); err != nil {
				return err
			}
		}
	}
	if b.Genres != nil {
		if _, err := r.DB.ExecContext(ctx, `DELETE FROM book_genres WHERE book_id=?`, b.ID); err != nil {
			return err
		}
		for _, g := range b.Genres {
			if _, err := r.DB.ExecContext(ctx, `INSERT OR IGNORE INTO book_genres(book_id, genre_id) VALUES(?,?)`, b.ID, g.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *CatalogRepo) ListEditions(ctx context.Context, bookID string) ([]book.Edition, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT e.id, e.book_id, e.language_id, l.code, e.publisher, e.edition_number, e.publication_year, e.format, e.page_count, e.version, e.created_at, e.updated_at
		 FROM editions e LEFT JOIN languages l ON l.id=e.language_id WHERE e.book_id=? ORDER BY e.publication_year, e.created_at`, bookID)
	if err != nil {
		return nil, err
	}
	var out []book.Edition
	for rows.Next() {
		var (
			e                            book.Edition
			langID, langCode, pub, edNum sql.NullString
			year, pages                  sql.NullInt64
		)
		if err := rows.Scan(&e.ID, &e.BookID, &langID, &langCode, &pub, &edNum, &year, &e.Format, &pages, &e.Version, &e.CreatedAt, &e.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		e.LanguageID = langID.String
		e.LanguageCode = langCode.String
		e.Publisher = pub.String
		e.EditionNumber = edNum.String
		if year.Valid {
			e.PublicationYear = int(year.Int64)
		}
		if pages.Valid {
			e.PageCount = int(pages.Int64)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	// Identifiers after closing (single-conn SQLite, no nested queries).
	for i := range out {
		ids, err := r.editionIdentifiers(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Identifiers = ids
	}
	return out, nil
}

func (r *CatalogRepo) editionIdentifiers(ctx context.Context, editionID string) ([]book.Identifier, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT identifier_type, identifier_value FROM book_identifiers WHERE edition_id=? ORDER BY identifier_type`, editionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []book.Identifier
	for rows.Next() {
		var id book.Identifier
		if err := rows.Scan(&id.Type, &id.Value); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *CatalogRepo) CreateEdition(ctx context.Context, e *book.Edition) error {
	now := nowUTC()
	if e.Version < 1 {
		e.Version = 1
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO editions(id, book_id, language_id, publisher, edition_number, publication_year, format, page_count, version, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.BookID, nullStr(e.LanguageID), nullStr(e.Publisher), nullStr(e.EditionNumber), nullInt(e.PublicationYear), e.Format, nullInt(e.PageCount), e.Version, now, now)
	if err != nil {
		return err
	}
	for _, idn := range e.Identifiers {
		if idn.Type == "" || idn.Value == "" {
			continue
		}
		// Validate identifier type loosely (isbn10/isbn13 + future).
		if _, err := r.DB.ExecContext(ctx,
			`INSERT OR IGNORE INTO book_identifiers(id, edition_id, identifier_type, identifier_value) VALUES(?,?,?,?)`,
			mustUUID(), e.ID, strings.ToLower(strings.TrimSpace(idn.Type)), strings.TrimSpace(idn.Value)); err != nil {
			// id column is TEXT PK without default — use generated UUID.
			return err
		}
	}
	return nil
}

func (r *CatalogRepo) FindDuplicate(ctx context.Context, title string, writerIDs []string, excludeID string) (*book.Book, error) {
	// Normalized exact title match (case-insensitive). Collect first, then
	// run the writer check after closing (single-conn SQLite deadlock fix).
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, title FROM books WHERE LOWER(TRIM(title)) = LOWER(TRIM(?)) AND deleted_at IS NULL AND id != ?`, title, excludeID)
	if err != nil {
		return nil, err
	}
	var candidates []book.Book
	for rows.Next() {
		var b book.Book
		if err := rows.Scan(&b.ID, &b.Title); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for _, b := range candidates {
		if len(writerIDs) == 0 {
			return &b, nil // same title alone is enough to warn
		}
		var shared int
		args := []any{b.ID}
		placeholders := make([]string, len(writerIDs))
		for i, wid := range writerIDs {
			placeholders[i] = "?"
			args = append(args, wid)
		}
		q := fmt.Sprintf(`SELECT COUNT(*) FROM book_writers WHERE book_id=? AND writer_id IN (%s)`, strings.Join(placeholders, ","))
		if err := r.DB.QueryRowContext(ctx, q, args...).Scan(&shared); err != nil {
			return nil, err
		}
		if shared > 0 {
			return &b, nil
		}
	}
	return nil, nil
}

// ── writers ─────────────────────────────────────────────────────────────────

func (r *CatalogRepo) ListWriters(ctx context.Context, query string, p repository.Page) ([]writer.Writer, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	args := []any{}
	where := "deleted_at IS NULL"
	if q := strings.TrimSpace(query); q != "" {
		where += " AND full_name LIKE ?"
		args = append(args, "%"+q+"%")
	}
	var total int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM writers WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), p.Limit, offsetOf(p))
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, full_name, biography, birth_date, death_date, website, version, created_at, updated_at FROM writers WHERE `+where+` ORDER BY full_name LIMIT ? OFFSET ?`,
		listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []writer.Writer
	for rows.Next() {
		var (
			w                       writer.Writer
			bio, birth, death, site sql.NullString
		)
		if err := rows.Scan(&w.ID, &w.FullName, &bio, &birth, &death, &site, &w.Version, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, 0, err
		}
		w.Biography, w.BirthDate, w.DeathDate, w.Website = bio.String, birth.String, death.String, site.String
		out = append(out, w)
	}
	return out, total, rows.Err()
}

func (r *CatalogRepo) GetWriter(ctx context.Context, id string) (*writer.Writer, error) {
	var (
		w                       writer.Writer
		bio, birth, death, site sql.NullString
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, full_name, biography, birth_date, death_date, website, version, created_at, updated_at FROM writers WHERE id=? AND deleted_at IS NULL`, id).
		Scan(&w.ID, &w.FullName, &bio, &birth, &death, &site, &w.Version, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w.Biography, w.BirthDate, w.DeathDate, w.Website = bio.String, birth.String, death.String, site.String
	return &w, nil
}

func (r *CatalogRepo) CreateWriter(ctx context.Context, w *writer.Writer) error {
	now := nowUTC()
	if w.Version < 1 {
		w.Version = 1
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO writers(id, full_name, biography, birth_date, death_date, website, version, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		w.ID, strings.TrimSpace(w.FullName), nullStr(w.Biography), nullStr(w.BirthDate), nullStr(w.DeathDate), nullStr(w.Website), w.Version, now, now)
	return err
}

func (r *CatalogRepo) UpdateWriter(ctx context.Context, w *writer.Writer) error {
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx,
		`UPDATE writers SET full_name=?, biography=?, birth_date=?, death_date=?, website=?, version=version+1, updated_at=? WHERE id=? AND deleted_at IS NULL`,
		strings.TrimSpace(w.FullName), nullStr(w.Biography), nullStr(w.BirthDate), nullStr(w.DeathDate), nullStr(w.Website), now, w.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	w.Version++
	w.UpdatedAt = now
	return nil
}

func (r *CatalogRepo) SoftDeleteWriter(ctx context.Context, id string) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE writers SET deleted_at=?, version=version+1 WHERE id=? AND deleted_at IS NULL`, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ── reference ───────────────────────────────────────────────────────────────

func (r *CatalogRepo) ListLanguages(ctx context.Context) ([]repository.Language, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, code, native_name, is_active FROM languages WHERE is_active=1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repository.Language
	for rows.Next() {
		var (
			l      repository.Language
			native sql.NullString
			active int
		)
		if err := rows.Scan(&l.ID, &l.Name, &l.Code, &native, &active); err != nil {
			return nil, err
		}
		l.Native = native.String
		l.Active = active == 1
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *CatalogRepo) ListGenres(ctx context.Context) ([]repository.Genre, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, slug, description, is_active FROM genres WHERE is_active=1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repository.Genre
	for rows.Next() {
		var (
			g      repository.Genre
			desc   sql.NullString
			active int
		)
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug, &desc, &active); err != nil {
			return nil, err
		}
		g.Description = desc.String
		g.Active = active == 1
		out = append(out, g)
	}
	return out, rows.Err()
}

// ── audit ───────────────────────────────────────────────────────────────────

func (r *CatalogRepo) Log(ctx context.Context, actorUserID, entityType, entityID, action, beforeJSON, afterJSON string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO audit_logs(id, actor_user_id, entity_type, entity_id, action, before_json, after_json, created_at) VALUES(?,?,?,?,?,?,?,?)`,
		mustUUID(), nullStr(actorUserID), entityType, entityID, action, nullStr(beforeJSON), nullStr(afterJSON), nowUTC())
	return err
}

// ── misc ────────────────────────────────────────────────────────────────────

func nullStr(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(n int) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(n), Valid: true}
}

func mustUUID() string {
	// Use package-level helper to avoid import cycle noise.
	return uuidGen()
}
