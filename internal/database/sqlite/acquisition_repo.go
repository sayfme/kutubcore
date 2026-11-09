package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"kutubcore/internal/domain/acquisition"
	"kutubcore/internal/repository"
)

var _ repository.AcquisitionRepository = (*AcquisitionRepo)(nil)

// AcquisitionRepo is the SQLite AcquisitionRepository.
type AcquisitionRepo struct{ DB *sql.DB }

func NewAcquisitionRepo(db *sql.DB) *AcquisitionRepo { return &AcquisitionRepo{DB: db} }

// CreateAcquisition inserts acquisition + items + copies in one transaction.
// Each unit of quantity creates one book_copies row.
func (r *AcquisitionRepo) CreateAcquisition(ctx context.Context, a *acquisition.Acquisition) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := nowUTC()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO acquisitions(id, user_id, source_id, bookshop_id, acquisition_date, total_cost, currency_code, notes, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		a.ID, a.UserID, nullStr(a.SourceID), nullStr(a.BookshopID), nullStr(a.AcquisitionDate), nFloat(a.TotalCost), nullStr(a.CurrencyCode), nullStr(a.Notes), now); err != nil {
		return err
	}

	for i := range a.Items {
		it := &a.Items[i]
		if it.ID == "" {
			it.ID = uuidGen()
		}
		it.AcquisitionID = a.ID
		if it.Quantity < 1 {
			it.Quantity = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO acquisition_items(id, acquisition_id, book_id, edition_id, quantity, unit_price, format)
			 VALUES(?,?,?,?,?,?,?)`,
			it.ID, a.ID, it.BookID, nullStr(it.EditionID), it.Quantity, nFloat(it.UnitPrice), it.Format); err != nil {
			return err
		}
		// One copy per unit.
		for j := 0; j < it.Quantity; j++ {
			cp := acquisition.Copy{ID: uuidGen(), AcquisitionItemID: it.ID, EditionID: it.EditionID, Format: it.Format}
			it.Copies = append(it.Copies, cp)
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO book_copies(id, acquisition_item_id, edition_id, format, condition, storage_location, acquired_at, notes)
				 VALUES(?,?,?,?,?,?,?,?)`,
				cp.ID, it.ID, nullStr(cp.EditionID), cp.Format, nullStr(""), nullStr(""), nullStr(a.AcquisitionDate), nullStr("")); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	a.CreatedAt = now
	return nil
}

func nFloat(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

func (r *AcquisitionRepo) ListAcquisitions(ctx context.Context, userID string, p repository.Page) ([]acquisition.Acquisition, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	var total int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM acquisitions WHERE user_id=?`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, user_id, source_id, bookshop_id, acquisition_date, total_cost, currency_code, notes, created_at
		 FROM acquisitions WHERE user_id=? ORDER BY acquisition_date DESC, created_at DESC LIMIT ? OFFSET ?`,
		userID, p.Limit, (p.Page-1)*p.Limit)
	if err != nil {
		return nil, 0, err
	}
	var out []acquisition.Acquisition
	for rows.Next() {
		var (
			a                            acquisition.Acquisition
			src, shop, date, curr, notes sql.NullString
			totalC                       sql.NullFloat64
		)
		if err := rows.Scan(&a.ID, &a.UserID, &src, &shop, &date, &totalC, &curr, &notes, &a.CreatedAt); err != nil {
			rows.Close()
			return nil, 0, err
		}
		a.SourceID, a.BookshopID, a.AcquisitionDate, a.CurrencyCode, a.Notes = src.String, shop.String, date.String, curr.String, notes.String
		if totalC.Valid {
			a.TotalCost = totalC.Float64
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	rows.Close()
	for i := range out {
		if err := r.attachItems(ctx, &out[i]); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *AcquisitionRepo) GetAcquisition(ctx context.Context, userID, id string) (*acquisition.Acquisition, error) {
	var (
		a                            acquisition.Acquisition
		src, shop, date, curr, notes sql.NullString
		totalC                       sql.NullFloat64
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, user_id, source_id, bookshop_id, acquisition_date, total_cost, currency_code, notes, created_at
		 FROM acquisitions WHERE id=? AND user_id=?`, id, userID).
		Scan(&a.ID, &a.UserID, &src, &shop, &date, &totalC, &curr, &notes, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.SourceID, a.BookshopID, a.AcquisitionDate, a.CurrencyCode, a.Notes = src.String, shop.String, date.String, curr.String, notes.String
	if totalC.Valid {
		a.TotalCost = totalC.Float64
	}
	if err := r.attachItems(ctx, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AcquisitionRepo) attachItems(ctx context.Context, a *acquisition.Acquisition) error {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, acquisition_id, book_id, edition_id, quantity, unit_price, format FROM acquisition_items WHERE acquisition_id=?`, a.ID)
	if err != nil {
		return err
	}
	var out []acquisition.Item
	for rows.Next() {
		var (
			it    acquisition.Item
			ed    sql.NullString
			price sql.NullFloat64
		)
		if err := rows.Scan(&it.ID, &it.AcquisitionID, &it.BookID, &ed, &it.Quantity, &price, &it.Format); err != nil {
			rows.Close()
			return err
		}
		it.EditionID = ed.String
		if price.Valid {
			it.UnitPrice = price.Float64
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for i := range out {
		copies, err := r.copiesForItem(ctx, out[i].ID)
		if err != nil {
			return err
		}
		out[i].Copies = copies
	}
	a.Items = out
	return nil
}

func (r *AcquisitionRepo) copiesForItem(ctx context.Context, itemID string) ([]acquisition.Copy, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT bc.id, bc.acquisition_item_id, bc.edition_id, bc.format, bc.condition, bc.storage_location, bc.acquired_at, bc.notes, ai.book_id, b.title
		 FROM book_copies bc JOIN acquisition_items ai ON ai.id=bc.acquisition_item_id
		 JOIN books b ON b.id=ai.book_id
		 WHERE bc.acquisition_item_id=?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []acquisition.Copy
	for rows.Next() {
		var (
			c                                       acquisition.Copy
			ed, cond, loc, at, notes, bookID, title sql.NullString
		)
		if err := rows.Scan(&c.ID, &c.AcquisitionItemID, &ed, &c.Format, &cond, &loc, &at, &notes, &bookID, &title); err != nil {
			return nil, err
		}
		c.EditionID, c.Condition, c.StorageLocation, c.AcquiredAt, c.Notes = ed.String, cond.String, loc.String, at.String, notes.String
		c.BookID, c.BookTitle = bookID.String, title.String
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *AcquisitionRepo) DeleteAcquisition(ctx context.Context, userID, id string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM acquisitions WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *AcquisitionRepo) ListCopies(ctx context.Context, userID string, bookID string, p repository.Page) ([]acquisition.Copy, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	args := []any{userID}
	where := "a.user_id=?"
	if bookID != "" {
		where += " AND ai.book_id=?"
		args = append(args, bookID)
	}
	var total int
	countSQL := `SELECT COUNT(*) FROM book_copies bc JOIN acquisition_items ai ON ai.id=bc.acquisition_item_id JOIN acquisitions a ON a.id=ai.acquisition_id WHERE ` + where
	if err := r.DB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), p.Limit, (p.Page-1)*p.Limit)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT bc.id, bc.acquisition_item_id, bc.edition_id, bc.format, bc.condition, bc.storage_location, bc.acquired_at, bc.notes, ai.book_id, b.title
		 FROM book_copies bc JOIN acquisition_items ai ON ai.id=bc.acquisition_item_id
		 JOIN acquisitions a ON a.id=ai.acquisition_id
		 JOIN books b ON b.id=ai.book_id
		 WHERE `+where+` ORDER BY bc.acquired_at DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []acquisition.Copy
	for rows.Next() {
		var (
			c                                    acquisition.Copy
			ed, cond, loc, at, notes, bid, title sql.NullString
		)
		if err := rows.Scan(&c.ID, &c.AcquisitionItemID, &ed, &c.Format, &cond, &loc, &at, &notes, &bid, &title); err != nil {
			return nil, 0, err
		}
		c.EditionID, c.Condition, c.StorageLocation, c.AcquiredAt, c.Notes = ed.String, cond.String, loc.String, at.String, notes.String
		c.BookID, c.BookTitle = bid.String, title.String
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *AcquisitionRepo) HasCopyForBook(ctx context.Context, userID, bookID string) (bool, error) {
	var n int
	err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM book_copies bc
		 JOIN acquisition_items ai ON ai.id=bc.acquisition_item_id
		 JOIN acquisitions a ON a.id=ai.acquisition_id
		 WHERE a.user_id=? AND ai.book_id=?`, userID, bookID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
