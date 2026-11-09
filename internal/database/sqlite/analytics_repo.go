package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"kutubcore/internal/domain/analytics"
	"kutubcore/internal/domain/shelf"
	"kutubcore/internal/repository"
)

var _ repository.AnalyticsRepository = (*AnalyticsRepo)(nil)

// AnalyticsRepo is the SQLite AnalyticsRepository.
type AnalyticsRepo struct{ DB *sql.DB }

func NewAnalyticsRepo(db *sql.DB) *AnalyticsRepo { return &AnalyticsRepo{DB: db} }

func (r *AnalyticsRepo) Analytics(ctx context.Context, userID string, months int) (*analytics.Summary, error) {
	if months < 1 || months > 24 {
		months = 6
	}
	// Build month buckets in Go (UTC), oldest → newest.
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -months+1, 0)
	buckets := make([]analytics.MonthStat, 0, months)
	for i := 0; i < months; i++ {
		m := start.AddDate(0, i, 0)
		buckets = append(buckets, analytics.MonthStat{Month: m.Format("2006-01")})
	}
	byMonth := map[string]*analytics.MonthStat{}
	for i := range buckets {
		byMonth[buckets[i].Month] = &buckets[i]
	}

	// Pages + sessions per month (from reading_sessions of this user).
	rows, err := r.DB.QueryContext(ctx,
		`SELECT substr(rs.started_at, 1, 7) AS m, COALESCE(SUM(rs.pages_read), 0), COUNT(*)
		 FROM reading_sessions rs JOIN user_books ub ON ub.id = rs.user_book_id
		 WHERE ub.user_id=? AND rs.started_at >= ? GROUP BY m ORDER BY m`, userID, start.Format("2006-01-01"))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			m           string
			pages, sess int
		)
		if err := rows.Scan(&m, &pages, &sess); err != nil {
			rows.Close()
			return nil, err
		}
		if st, ok := byMonth[m]; ok {
			st.PagesRead = pages
			st.Sessions = sess
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Books done per month (from reading_progress completed_at).
	drows, err := r.DB.QueryContext(ctx,
		`SELECT substr(rp.completed_at, 1, 7) AS m, COUNT(*)
		 FROM reading_progress rp JOIN user_books ub ON ub.id = rp.user_book_id
		 WHERE ub.user_id=? AND rp.completed_at IS NOT NULL AND rp.completed_at >= ? GROUP BY m ORDER BY m`,
		userID, start.Format("2006-01-01"))
	if err != nil {
		return nil, err
	}
	for drows.Next() {
		var (
			m     string
			count int
		)
		if err := drows.Scan(&m, &count); err != nil {
			drows.Close()
			return nil, err
		}
		if st, ok := byMonth[m]; ok {
			st.BooksDone = count
		}
	}
	if err := drows.Err(); err != nil {
		drows.Close()
		return nil, err
	}
	drows.Close()

	sum := &analytics.Summary{Months: buckets}
	for i := range buckets {
		if buckets[i].Sessions > 0 {
			buckets[i].AvgSession = buckets[i].PagesRead / buckets[i].Sessions
		}
		sum.TotalPages += buckets[i].PagesRead
		sum.TotalSess += buckets[i].Sessions
		sum.TotalDone += buckets[i].BooksDone
	}
	return sum, nil
}

// ── goals ───────────────────────────────────────────────────────────────────

func (r *AnalyticsRepo) GetGoal(ctx context.Context, userID, periodMonth string) (*analytics.Goal, error) {
	var g analytics.Goal
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, user_id, period_month, books_target, created_at, updated_at FROM reading_goals WHERE user_id=? AND period_month=?`, userID, periodMonth).
		Scan(&g.ID, &g.UserID, &g.PeriodMonth, &g.BooksTarget, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Attach done-count for that month.
	var done int
	_ = r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM reading_progress rp JOIN user_books ub ON ub.id=rp.user_book_id WHERE ub.user_id=? AND substr(rp.completed_at,1,7)=?`,
		userID, periodMonth).Scan(&done)
	g.BooksDone = done
	return &g, nil
}

func (r *AnalyticsRepo) UpsertGoal(ctx context.Context, g *analytics.Goal) error {
	now := nowUTC()
	if g.ID == "" {
		g.ID = uuidGen()
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO reading_goals(id, user_id, period_month, books_target, created_at, updated_at) VALUES(?,?,?,?,?,?)
		 ON CONFLICT(user_id, period_month) DO UPDATE SET books_target=excluded.books_target, updated_at=excluded.updated_at`,
		g.ID, g.UserID, g.PeriodMonth, g.BooksTarget, now, now)
	return err
}

func (r *AnalyticsRepo) ListGoals(ctx context.Context, userID string) ([]analytics.Goal, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, user_id, period_month, books_target, created_at, updated_at FROM reading_goals WHERE user_id=? ORDER BY period_month DESC`, userID)
	if err != nil {
		return nil, err
	}
	var out []analytics.Goal
	for rows.Next() {
		var g analytics.Goal
		if err := rows.Scan(&g.ID, &g.UserID, &g.PeriodMonth, &g.BooksTarget, &g.CreatedAt, &g.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range out {
		done, err := r.doneForMonth(ctx, userID, out[i].PeriodMonth)
		if err != nil {
			return nil, err
		}
		out[i].BooksDone = done
	}
	return out, nil
}

func (r *AnalyticsRepo) doneForMonth(ctx context.Context, userID, periodMonth string) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM reading_progress rp JOIN user_books ub ON ub.id=rp.user_book_id WHERE ub.user_id=? AND substr(rp.completed_at,1,7)=?`,
		userID, periodMonth).Scan(&n)
	return n, err
}

// ── shelf ───────────────────────────────────────────────────────────────────

func (r *AnalyticsRepo) GetShelfSettings(ctx context.Context, userID string) (*shelf.Settings, error) {
	var (
		s       shelf.Settings
		enabled int
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT user_id, enabled, dust_days, cobweb_days, worm_days, updated_at FROM shelf_settings WHERE user_id=?`, userID).
		Scan(&s.UserID, &enabled, &s.DustDays, &s.CobwebDays, &s.WormDays, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		d := shelf.Defaults(userID)
		return &d, nil
	}
	if err != nil {
		return nil, err
	}
	s.Enabled = enabled == 1
	return &s, nil
}

func (r *AnalyticsRepo) UpsertShelfSettings(ctx context.Context, s *shelf.Settings) error {
	enabled := 0
	if s.Enabled {
		enabled = 1
	}
	now := nowUTC()
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO shelf_settings(user_id, enabled, dust_days, cobweb_days, worm_days, updated_at) VALUES(?,?,?,?,?,?)
		 ON CONFLICT(user_id) DO UPDATE SET enabled=excluded.enabled, dust_days=excluded.dust_days,
		   cobweb_days=excluded.cobweb_days, worm_days=excluded.worm_days, updated_at=excluded.updated_at`,
		s.UserID, enabled, s.DustDays, s.CobwebDays, s.WormDays, now)
	return err
}

func (r *AnalyticsRepo) ShelfEntries(ctx context.Context, userID string) ([]shelf.Entry, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT ub.id, ub.book_id, b.title, ub.reading_status, ub.is_favorite,
		        COALESCE(MAX(rs.started_at), ub.updated_at) AS last_activity
		 FROM user_books ub
		 JOIN books b ON b.id = ub.book_id AND b.deleted_at IS NULL
		 LEFT JOIN reading_sessions rs ON rs.user_book_id = ub.id
		 WHERE ub.user_id=?
		 GROUP BY ub.id
		 ORDER BY ub.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shelf.Entry
	for rows.Next() {
		var (
			e   shelf.Entry
			fav int
			act sql.NullString
		)
		if err := rows.Scan(&e.UserBookID, &e.BookID, &e.BookTitle, &e.ReadingStatus, &fav, &act); err != nil {
			return nil, err
		}
		e.IsFavorite = fav == 1
		e.LastActivity = act.String
		out = append(out, e)
	}
	return out, rows.Err()
}
