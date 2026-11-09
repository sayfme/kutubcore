package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"kutubcore/internal/domain/files"
	"kutubcore/internal/repository"
)

var _ repository.FileRepository = (*CatalogRepo)(nil)

func (r *CatalogRepo) ListFiles(ctx context.Context, bookID string) ([]files.File, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, book_id, edition_id, file_name, file_format, file_size, checksum_sha256, mime_type, created_at FROM book_files WHERE book_id=? ORDER BY created_at DESC`, bookID)
	if err != nil {
		return nil, err
	}
	var out []files.File
	for rows.Next() {
		var (
			f                  files.File
			ed                 sql.NullString
			size               sql.NullInt64
			sum, mime, created sql.NullString
		)
		if err := rows.Scan(&f.ID, &f.BookID, &ed, &f.FileName, &f.FileFormat, &size, &sum, &mime, &created); err != nil {
			rows.Close()
			return nil, err
		}
		f.EditionID = ed.String
		if size.Valid {
			f.FileSize = size.Int64
		}
		f.ChecksumSHA256, f.MimeType, f.CreatedAt = sum.String, mime.String, created.String
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range out {
		if err := r.attachFileLocations(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *CatalogRepo) GetFile(ctx context.Context, id string) (*files.File, error) {
	var (
		f         files.File
		ed        sql.NullString
		size      sql.NullInt64
		sum, mime sql.NullString
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, book_id, edition_id, file_name, file_format, file_size, checksum_sha256, mime_type, created_at FROM book_files WHERE id=?`, id).
		Scan(&f.ID, &f.BookID, &ed, &f.FileName, &f.FileFormat, &size, &sum, &mime, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.EditionID = ed.String
	if size.Valid {
		f.FileSize = size.Int64
	}
	f.ChecksumSHA256, f.MimeType = sum.String, mime.String
	if err := r.attachFileLocations(ctx, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *CatalogRepo) attachFileLocations(ctx context.Context, f *files.File) error {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, location_type, storage_path, external_url, is_available FROM file_locations WHERE book_file_id=?`, f.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var out []files.Location
	for rows.Next() {
		var (
			l         files.Location
			path, ext sql.NullString
			avail     int
		)
		if err := rows.Scan(&l.ID, &l.LocationType, &path, &ext, &avail); err != nil {
			return err
		}
		l.BookFileID = f.ID
		l.StoragePath, l.ExternalURL = path.String, ext.String
		l.IsAvailable = avail == 1
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	f.Locations = out
	return nil
}

func (r *CatalogRepo) CreateFile(ctx context.Context, f *files.File) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO book_files(id, book_id, edition_id, file_name, file_format, file_size, checksum_sha256, mime_type, created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		f.ID, f.BookID, nullStr(f.EditionID), strings.TrimSpace(f.FileName), f.FileFormat, nInt64(f.FileSize), nullStr(f.ChecksumSHA256), nullStr(f.MimeType), nowUTC())
	return err
}

func (r *CatalogRepo) AddLocation(ctx context.Context, loc *files.Location) error {
	if loc.ID == "" {
		loc.ID = uuidGen()
	}
	avail := 0
	if loc.IsAvailable {
		avail = 1
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO file_locations(id, book_file_id, location_type, storage_path, external_url, is_available) VALUES(?,?,?,?,?,?)`,
		loc.ID, loc.BookFileID, loc.LocationType, nullStr(loc.StoragePath), nullStr(loc.ExternalURL), avail)
	return err
}

func (r *CatalogRepo) DeleteFile(ctx context.Context, id string) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM book_files WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *CatalogRepo) SetCover(ctx context.Context, bookID, fileID string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE books SET cover_image_id=?, updated_at=? WHERE id=? AND deleted_at IS NULL`, fileID, nowUTC(), bookID)
	return err
}

func (r *CatalogRepo) HasAvailableFile(ctx context.Context, bookID string) (bool, error) {
	var n int
	err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM book_files bf JOIN file_locations fl ON fl.book_file_id=bf.id
		 WHERE bf.book_id=? AND fl.is_available=1`, bookID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func nInt64(n int64) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}
