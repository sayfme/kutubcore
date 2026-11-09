package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"kutubcore/internal/domain/collection"
	"kutubcore/internal/domain/reading"
	"kutubcore/internal/domain/review"
	"kutubcore/internal/repository"
)

var _ repository.PersonalRepository = (*PersonalRepo)(nil)

// PersonalRepo is the SQLite PersonalRepository.
type PersonalRepo struct{ DB *sql.DB }

func NewPersonalRepo(db *sql.DB) *PersonalRepo { return &PersonalRepo{DB: db} }

// ── helpers ─────────────────────────────────────────────────────────────────

// scanUserBooks collects rows (no nested queries while open).
func scanUserBooks(rows *sql.Rows) ([]collection.UserBook, error) {
	var out []collection.UserBook
	for rows.Next() {
		var (
			ub  collection.UserBook
			fav int
		)
		if err := rows.Scan(&ub.ID, &ub.UserID, &ub.BookID, &ub.BookTitle, &ub.PhysicalStatus, &ub.DigitalStatus, &ub.ReadingStatus, &fav, &ub.CreatedAt, &ub.UpdatedAt); err != nil {
			return nil, err
		}
		ub.IsFavorite = fav == 1
		out = append(out, ub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return out, nil
}

const userBookSelect = `SELECT ub.id, ub.user_id, ub.book_id, b.title, ub.physical_status, ub.digital_status, ub.reading_status, ub.is_favorite, ub.created_at, ub.updated_at
	FROM user_books ub JOIN books b ON b.id=ub.book_id AND b.deleted_at IS NULL`

// ── user books ──────────────────────────────────────────────────────────────

func (r *PersonalRepo) ListUserBooks(ctx context.Context, userID string, f repository.PersonalFilter, p repository.Page) ([]collection.UserBook, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	where := []string{"ub.user_id=?"}
	args := []any{userID}
	if f.Query != "" {
		where = append(where, "b.title LIKE ?")
		args = append(args, "%"+f.Query+"%")
	}
	if f.PhysicalStatus != "" {
		where = append(where, "ub.physical_status=?")
		args = append(args, f.PhysicalStatus)
	}
	if f.DigitalStatus != "" {
		where = append(where, "ub.digital_status=?")
		args = append(args, f.DigitalStatus)
	}
	if f.ReadingStatus != "" {
		where = append(where, "ub.reading_status=?")
		args = append(args, f.ReadingStatus)
	}
	if f.Favorite != nil {
		v := 0
		if *f.Favorite {
			v = 1
		}
		where = append(where, "ub.is_favorite=?")
		args = append(args, v)
	}
	whereSQL := joinAnd(where)

	var total int
	if err := r.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM books b JOIN user_books ub ON ub.book_id=b.id WHERE "+whereSQL, args...).Scan(&total); err != nil {
		// Fallback: count via user_books only (join may hide soft-deleted books).
		if err2 := r.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_books ub WHERE "+whereSQL, args...).Scan(&total); err2 != nil {
			return nil, 0, err
		}
	}
	listArgs := append(append([]any{}, args...), p.Limit, (p.Page-1)*p.Limit)
	rows, err := r.DB.QueryContext(ctx, userBookSelect+" WHERE "+whereSQL+" ORDER BY ub.created_at DESC LIMIT ? OFFSET ?", listArgs...)
	if err != nil {
		return nil, 0, err
	}
	out, err := scanUserBooks(rows)
	if err != nil {
		return nil, 0, err
	}
	// Attach progress/review per row (after rows closed).
	for i := range out {
		if err := r.attachProgressReview(ctx, &out[i]); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *PersonalRepo) GetUserBook(ctx context.Context, userID, id string) (*collection.UserBook, error) {
	var (
		ub  collection.UserBook
		fav int
	)
	err := r.DB.QueryRowContext(ctx, userBookSelect+" WHERE ub.id=? AND ub.user_id=?", id, userID).
		Scan(&ub.ID, &ub.UserID, &ub.BookID, &ub.BookTitle, &ub.PhysicalStatus, &ub.DigitalStatus, &ub.ReadingStatus, &fav, &ub.CreatedAt, &ub.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	ub.IsFavorite = fav == 1
	if err := r.attachProgressReview(ctx, &ub); err != nil {
		return nil, err
	}
	return &ub, nil
}

func (r *PersonalRepo) attachProgressReview(ctx context.Context, ub *collection.UserBook) error {
	p, err := r.GetProgress(ctx, ub.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if p != nil {
		ub.Progress = &collection.ProgressData{
			CurrentPage: p.CurrentPage, TotalPages: p.TotalPages,
			ProgressPercent: p.ProgressPercent, StartedAt: p.StartedAt, CompletedAt: p.CompletedAt,
		}
	}
	rv, err := r.GetReview(ctx, ub.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if rv != nil {
		ub.Review = &collection.ReviewData{Rating: rv.Rating, Reaction: rv.Reaction, ReviewText: rv.ReviewText}
	}
	return nil
}

func (r *PersonalRepo) CreateUserBook(ctx context.Context, ub *collection.UserBook) error {
	now := nowUTC()
	if ub.PhysicalStatus == "" {
		ub.PhysicalStatus = "not_collected"
	}
	if ub.DigitalStatus == "" {
		ub.DigitalStatus = "not_available"
	}
	if ub.ReadingStatus == "" {
		ub.ReadingStatus = "want_to_read"
	}
	fav := 0
	if ub.IsFavorite {
		fav = 1
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO user_books(id, user_id, book_id, physical_status, digital_status, reading_status, is_favorite, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		ub.ID, ub.UserID, ub.BookID, ub.PhysicalStatus, ub.DigitalStatus, ub.ReadingStatus, fav, now, now)
	if err != nil {
		return err
	}
	ub.CreatedAt, ub.UpdatedAt = now, now
	return nil
}

func (r *PersonalRepo) UpdateUserBook(ctx context.Context, ub *collection.UserBook) error {
	fav := 0
	if ub.IsFavorite {
		fav = 1
	}
	res, err := r.DB.ExecContext(ctx,
		`UPDATE user_books SET physical_status=?, digital_status=?, reading_status=?, is_favorite=?, updated_at=?
		 WHERE id=? AND user_id=?`,
		ub.PhysicalStatus, ub.DigitalStatus, ub.ReadingStatus, fav, nowUTC(), ub.ID, ub.UserID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *PersonalRepo) DeleteUserBook(ctx context.Context, userID, id string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM user_books WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ── progress ────────────────────────────────────────────────────────────────

func (r *PersonalRepo) GetProgress(ctx context.Context, userBookID string) (*reading.Progress, error) {
	var (
		p                  reading.Progress
		curr, total        sql.NullInt64
		started, completed sql.NullString
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, user_book_id, current_page, total_pages, progress_percent, started_at, completed_at, updated_at FROM reading_progress WHERE user_book_id=?`, userBookID).
		Scan(&p.ID, &p.UserBookID, &curr, &total, &p.ProgressPercent, &started, &completed, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if curr.Valid {
		p.CurrentPage = int(curr.Int64)
	}
	if total.Valid {
		p.TotalPages = int(total.Int64)
	}
	p.StartedAt, p.CompletedAt = started.String, completed.String
	return &p, nil
}

func (r *PersonalRepo) UpsertProgress(ctx context.Context, p *reading.Progress) error {
	now := nowUTC()
	p.UpdatedAt = now
	if p.ID == "" {
		p.ID = mustUUID()
	}
	// Upsert on unique user_book_id.
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO reading_progress(id, user_book_id, current_page, total_pages, progress_percent, started_at, completed_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)
		 ON CONFLICT(user_book_id) DO UPDATE SET
		   current_page=excluded.current_page, total_pages=excluded.total_pages,
		   progress_percent=excluded.progress_percent, started_at=excluded.started_at,
		   completed_at=excluded.completed_at, updated_at=excluded.updated_at`,
		p.ID, p.UserBookID, nInt(p.CurrentPage), nInt(p.TotalPages), p.ProgressPercent, nStr(p.StartedAt), nStr(p.CompletedAt), now)
	return err
}

// ── sessions ────────────────────────────────────────────────────────────────

func (r *PersonalRepo) ListSessions(ctx context.Context, userBookID string) ([]reading.Session, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, user_book_id, started_at, ended_at, pages_read, reading_note FROM reading_sessions WHERE user_book_id=? ORDER BY started_at DESC`, userBookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []reading.Session
	for rows.Next() {
		var (
			s           reading.Session
			ended, note sql.NullString
			pages       sql.NullInt64
		)
		if err := rows.Scan(&s.ID, &s.UserBookID, &s.StartedAt, &ended, &pages, &note); err != nil {
			return nil, err
		}
		s.EndedAt, s.Note = ended.String, note.String
		if pages.Valid {
			s.PagesRead = int(pages.Int64)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *PersonalRepo) CreateSession(ctx context.Context, s *reading.Session) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO reading_sessions(id, user_book_id, started_at, ended_at, pages_read, reading_note) VALUES(?,?,?,?,?,?)`,
		s.ID, s.UserBookID, s.StartedAt, nStr(s.EndedAt), nInt(s.PagesRead), nStr(s.Note))
	return err
}

// ── notes ───────────────────────────────────────────────────────────────────

func (r *PersonalRepo) ListNotes(ctx context.Context, userBookID string) ([]review.Note, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, user_book_id, title, content, page_number, visibility, created_at, updated_at FROM notes WHERE user_book_id=? ORDER BY created_at DESC`, userBookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []review.Note
	for rows.Next() {
		var (
			n     review.Note
			title sql.NullString
			page  sql.NullInt64
		)
		if err := rows.Scan(&n.ID, &n.UserBookID, &title, &n.Content, &page, &n.Visibility, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.Title = title.String
		if page.Valid {
			n.PageNumber = int(page.Int64)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *PersonalRepo) GetNote(ctx context.Context, userBookID, noteID string) (*review.Note, error) {
	var (
		n     review.Note
		title sql.NullString
		page  sql.NullInt64
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, user_book_id, title, content, page_number, visibility, created_at, updated_at FROM notes WHERE id=? AND user_book_id=?`, noteID, userBookID).
		Scan(&n.ID, &n.UserBookID, &title, &n.Content, &page, &n.Visibility, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	n.Title = title.String
	if page.Valid {
		n.PageNumber = int(page.Int64)
	}
	return &n, nil
}

func (r *PersonalRepo) CreateNote(ctx context.Context, n *review.Note) error {
	now := nowUTC()
	if n.Visibility == "" {
		n.Visibility = "private"
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO notes(id, user_book_id, title, content, page_number, visibility, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		n.ID, n.UserBookID, nStr(n.Title), n.Content, nInt(n.PageNumber), n.Visibility, now, now)
	if err != nil {
		return err
	}
	n.CreatedAt, n.UpdatedAt = now, now
	return nil
}

func (r *PersonalRepo) UpdateNote(ctx context.Context, n *review.Note) error {
	res, err := r.DB.ExecContext(ctx,
		`UPDATE notes SET title=?, content=?, page_number=?, visibility=?, updated_at=? WHERE id=? AND user_book_id=?`,
		nStr(n.Title), n.Content, nInt(n.PageNumber), n.Visibility, nowUTC(), n.ID, n.UserBookID)
	if err != nil {
		return err
	}
	x, _ := res.RowsAffected()
	if x == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *PersonalRepo) DeleteNote(ctx context.Context, userBookID, noteID string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM notes WHERE id=? AND user_book_id=?`, noteID, userBookID)
	if err != nil {
		return err
	}
	x, _ := res.RowsAffected()
	if x == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ── reviews ─────────────────────────────────────────────────────────────────

func (r *PersonalRepo) GetReview(ctx context.Context, userBookID string) (*review.Review, error) {
	var (
		rv             review.Review
		rating         sql.NullInt64
		reaction, text sql.NullString
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, user_book_id, rating, reaction, review_text, created_at, updated_at FROM reviews WHERE user_book_id=?`, userBookID).
		Scan(&rv.ID, &rv.UserBookID, &rating, &reaction, &text, &rv.CreatedAt, &rv.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if rating.Valid {
		rv.Rating = int(rating.Int64)
	}
	rv.Reaction, rv.ReviewText = reaction.String, text.String
	return &rv, nil
}

func (r *PersonalRepo) UpsertReview(ctx context.Context, rv *review.Review) error {
	now := nowUTC()
	if rv.ID == "" {
		rv.ID = mustUUID()
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO reviews(id, user_book_id, rating, reaction, review_text, created_at, updated_at) VALUES(?,?,?,?,?,?,?)
		 ON CONFLICT(user_book_id) DO UPDATE SET rating=excluded.rating, reaction=excluded.reaction,
		   review_text=excluded.review_text, updated_at=excluded.updated_at`,
		rv.ID, rv.UserBookID, nInt(rv.Rating), nStr(rv.Reaction), nStr(rv.ReviewText), now, now)
	return err
}

func (r *PersonalRepo) DeleteReview(ctx context.Context, userBookID string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM reviews WHERE user_book_id=?`, userBookID)
	return err
}

// ── dashboard ───────────────────────────────────────────────────────────────

func (r *PersonalRepo) Dashboard(ctx context.Context, userID string) (*repository.Dashboard, error) {
	d := &repository.Dashboard{}
	q := func(dest *int, query string, args ...any) error {
		return r.DB.QueryRowContext(ctx, query, args...).Scan(dest)
	}
	if err := q(&d.TotalBooks, `SELECT COUNT(*) FROM user_books WHERE user_id=?`, userID); err != nil {
		return nil, err
	}
	if err := q(&d.PhysicalOwned, `SELECT COUNT(*) FROM user_books WHERE user_id=? AND physical_status='owned'`, userID); err != nil {
		return nil, err
	}
	if err := q(&d.DigitalDownloaded, `SELECT COUNT(*) FROM user_books WHERE user_id=? AND digital_status='downloaded'`, userID); err != nil {
		return nil, err
	}
	if err := q(&d.CurrentlyReading, `SELECT COUNT(*) FROM user_books WHERE user_id=? AND reading_status='currently_reading'`, userID); err != nil {
		return nil, err
	}
	if err := q(&d.Completed, `SELECT COUNT(*) FROM user_books WHERE user_id=? AND reading_status='completed'`, userID); err != nil {
		return nil, err
	}
	if err := q(&d.WantToRead, `SELECT COUNT(*) FROM user_books WHERE user_id=? AND reading_status='want_to_read'`, userID); err != nil {
		return nil, err
	}
	if err := q(&d.Favorites, `SELECT COUNT(*) FROM user_books WHERE user_id=? AND is_favorite=1`, userID); err != nil {
		return nil, err
	}

	// Recent sessions for this user.
	rows, err := r.DB.QueryContext(ctx,
		`SELECT rs.id, rs.user_book_id, rs.started_at, rs.ended_at, rs.pages_read, rs.reading_note
		 FROM reading_sessions rs JOIN user_books ub ON ub.id=rs.user_book_id
		 WHERE ub.user_id=? ORDER BY rs.started_at DESC LIMIT 5`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			s           reading.Session
			ended, note sql.NullString
			pages       sql.NullInt64
		)
		if err := rows.Scan(&s.ID, &s.UserBookID, &s.StartedAt, &ended, &pages, &note); err != nil {
			rows.Close()
			return nil, err
		}
		s.EndedAt, s.Note = ended.String, note.String
		if pages.Valid {
			s.PagesRead = int(pages.Int64)
		}
		d.RecentSessions = append(d.RecentSessions, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Recently added books.
	rrows, err := r.DB.QueryContext(ctx, userBookSelect+` WHERE ub.user_id=? ORDER BY ub.created_at DESC LIMIT 5`, userID)
	if err != nil {
		return nil, err
	}
	recent, err := scanUserBooks(rrows)
	if err != nil {
		return nil, err
	}
	for i := range recent {
		if err := r.attachProgressReview(ctx, &recent[i]); err != nil {
			return nil, err
		}
	}
	d.RecentlyAdded = recent
	return d, nil
}

// ── misc ────────────────────────────────────────────────────────────────────

func joinAnd(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += " AND " + p
	}
	return out
}

func nInt(n int) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(n), Valid: true}
}

func nStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
