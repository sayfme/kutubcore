package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"kutubcore/internal/domain/bookshop"
	"kutubcore/internal/domain/source"
	"kutubcore/internal/repository"
)

var _ repository.DirectoryRepository = (*CatalogRepo)(nil)

// ── bookshops ───────────────────────────────────────────────────────────────

func (r *CatalogRepo) ListBookshops(ctx context.Context, query string, p repository.Page) ([]bookshop.Bookshop, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	where := "deleted_at IS NULL"
	args := []any{}
	_ = args
	if q := strings.TrimSpace(query); q != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+q+"%")
	}
	var total int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookshops WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), p.Limit, (p.Page-1)*p.Limit)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, name, shop_type, address, city, country, phone, email, website, notes, version, created_at, updated_at FROM bookshops WHERE `+where+` ORDER BY name LIMIT ? OFFSET ?`,
		listArgs...)
	if err != nil {
		return nil, 0, err
	}
	var out []bookshop.Bookshop
	for rows.Next() {
		var (
			b                                             bookshop.Bookshop
			addr, city, country, phone, email, web, notes sql.NullString
		)
		if err := rows.Scan(&b.ID, &b.Name, &b.ShopType, &addr, &city, &country, &phone, &email, &web, &notes, &b.Version, &b.CreatedAt, &b.UpdatedAt); err != nil {
			rows.Close()
			return nil, 0, err
		}
		b.Address, b.City, b.Country, b.Phone, b.Email, b.Website, b.Notes = addr.String, city.String, country.String, phone.String, email.String, web.String, notes.String
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	rows.Close()
	for i := range out {
		if err := r.attachBookshopLinks(ctx, &out[i]); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *CatalogRepo) GetBookshop(ctx context.Context, id string) (*bookshop.Bookshop, error) {
	var (
		b                                             bookshop.Bookshop
		addr, city, country, phone, email, web, notes sql.NullString
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, name, shop_type, address, city, country, phone, email, website, notes, version, created_at, updated_at FROM bookshops WHERE id=? AND deleted_at IS NULL`, id).
		Scan(&b.ID, &b.Name, &b.ShopType, &addr, &city, &country, &phone, &email, &web, &notes, &b.Version, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Address, b.City, b.Country, b.Phone, b.Email, b.Website, b.Notes = addr.String, city.String, country.String, phone.String, email.String, web.String, notes.String
	if err := r.attachBookshopLinks(ctx, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *CatalogRepo) CreateBookshop(ctx context.Context, b *bookshop.Bookshop) error {
	if b.Version < 1 {
		b.Version = 1
	}
	now := nowUTC()
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO bookshops(id, name, shop_type, address, city, country, phone, email, website, notes, version, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, strings.TrimSpace(b.Name), b.ShopType, nullStr(b.Address), nullStr(b.City), nullStr(b.Country), nullStr(b.Phone), nullStr(b.Email), nullStr(b.Website), nullStr(b.Notes), b.Version, now, now)
	if err != nil {
		return err
	}
	for i := range b.Links {
		_ = r.AddBookshopLink(ctx, &b.Links[i])
	}
	return nil
}

func (r *CatalogRepo) UpdateBookshop(ctx context.Context, b *bookshop.Bookshop) error {
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx,
		`UPDATE bookshops SET name=?, shop_type=?, address=?, city=?, country=?, phone=?, email=?, website=?, notes=?, version=version+1, updated_at=? WHERE id=? AND deleted_at IS NULL`,
		strings.TrimSpace(b.Name), b.ShopType, nullStr(b.Address), nullStr(b.City), nullStr(b.Country), nullStr(b.Phone), nullStr(b.Email), nullStr(b.Website), nullStr(b.Notes), now, b.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	b.Version++
	return nil
}

func (r *CatalogRepo) SoftDeleteBookshop(ctx context.Context, id string) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE bookshops SET deleted_at=?, version=version+1 WHERE id=? AND deleted_at IS NULL`, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *CatalogRepo) attachBookshopLinks(ctx context.Context, b *bookshop.Bookshop) error {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, label, url, is_primary FROM bookshop_links WHERE bookshop_id=? ORDER BY is_primary DESC, label`, b.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var out []bookshop.Link
	for rows.Next() {
		var (
			l    bookshop.Link
			prim int
		)
		if err := rows.Scan(&l.ID, &l.Label, &l.URL, &prim); err != nil {
			return err
		}
		l.BookshopID = b.ID
		l.IsPrimary = prim == 1
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	b.Links = out
	return nil
}

func (r *CatalogRepo) AddBookshopLink(ctx context.Context, link *bookshop.Link) error {
	if link.ID == "" {
		link.ID = uuidGen()
	}
	prim := 0
	if link.IsPrimary {
		prim = 1
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO bookshop_links(id, bookshop_id, label, url, is_primary) VALUES(?,?,?,?,?)`,
		link.ID, link.BookshopID, strings.TrimSpace(link.Label), strings.TrimSpace(link.URL), prim)
	return err
}

func (r *CatalogRepo) DeleteBookshopLink(ctx context.Context, bookshopID, linkID string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM bookshop_links WHERE id=? AND bookshop_id=?`, linkID, bookshopID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ── sources ─────────────────────────────────────────────────────────────────

func (r *CatalogRepo) ListSources(ctx context.Context, query string, p repository.Page) ([]source.Source, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	where := "deleted_at IS NULL"
	args := []any{}
	if q := strings.TrimSpace(query); q != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+q+"%")
	}
	var total int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM collection_sources WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), p.Limit, (p.Page-1)*p.Limit)
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, name, source_type, description, version, created_at, updated_at FROM collection_sources WHERE `+where+` ORDER BY name LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	var out []source.Source
	for rows.Next() {
		var (
			s    source.Source
			desc sql.NullString
		)
		if err := rows.Scan(&s.ID, &s.Name, &s.SourceType, &desc, &s.Version, &s.CreatedAt, &s.UpdatedAt); err != nil {
			rows.Close()
			return nil, 0, err
		}
		s.Description = desc.String
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	rows.Close()
	for i := range out {
		if err := r.attachSourceLinks(ctx, &out[i]); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *CatalogRepo) GetSource(ctx context.Context, id string) (*source.Source, error) {
	var (
		s    source.Source
		desc sql.NullString
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, name, source_type, description, version, created_at, updated_at FROM collection_sources WHERE id=? AND deleted_at IS NULL`, id).
		Scan(&s.ID, &s.Name, &s.SourceType, &desc, &s.Version, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.Description = desc.String
	if err := r.attachSourceLinks(ctx, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *CatalogRepo) CreateSource(ctx context.Context, s *source.Source) error {
	if s.Version < 1 {
		s.Version = 1
	}
	now := nowUTC()
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO collection_sources(id, name, source_type, description, version, created_at, updated_at) VALUES(?,?,?,?,?,?,?)`,
		s.ID, strings.TrimSpace(s.Name), s.SourceType, nullStr(s.Description), s.Version, now, now)
	if err != nil {
		return err
	}
	for i := range s.Links {
		_ = r.AddSourceLink(ctx, &s.Links[i])
	}
	return nil
}

func (r *CatalogRepo) UpdateSource(ctx context.Context, s *source.Source) error {
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx,
		`UPDATE collection_sources SET name=?, source_type=?, description=?, version=version+1, updated_at=? WHERE id=? AND deleted_at IS NULL`,
		strings.TrimSpace(s.Name), s.SourceType, nullStr(s.Description), now, s.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	s.Version++
	return nil
}

func (r *CatalogRepo) SoftDeleteSource(ctx context.Context, id string) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE collection_sources SET deleted_at=?, version=version+1 WHERE id=? AND deleted_at IS NULL`, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *CatalogRepo) attachSourceLinks(ctx context.Context, s *source.Source) error {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, label, url FROM source_links WHERE source_id=? ORDER BY label`, s.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var out []source.Link
	for rows.Next() {
		var l source.Link
		if err := rows.Scan(&l.ID, &l.Label, &l.URL); err != nil {
			return err
		}
		l.SourceID = s.ID
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.Links = out
	return nil
}

func (r *CatalogRepo) AddSourceLink(ctx context.Context, link *source.Link) error {
	if link.ID == "" {
		link.ID = uuidGen()
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO source_links(id, source_id, label, url) VALUES(?,?,?,?)`,
		link.ID, link.SourceID, strings.TrimSpace(link.Label), strings.TrimSpace(link.URL))
	return err
}

func (r *CatalogRepo) DeleteSourceLink(ctx context.Context, sourceID, linkID string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM source_links WHERE id=? AND source_id=?`, linkID, sourceID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}
