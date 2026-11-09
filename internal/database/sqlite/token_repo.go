package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"kutubcore/internal/repository"
)

var _ repository.TokenRepository = (*TokenRepo)(nil)

// TokenRepo is the SQLite TokenRepository.
type TokenRepo struct{ DB *sql.DB }

func NewTokenRepo(db *sql.DB) *TokenRepo { return &TokenRepo{DB: db} }

func (r *TokenRepo) CreatePasswordReset(ctx context.Context, id, userID, tokenHash, expiresAt string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO password_resets(id, user_id, token_hash, expires_at, created_at) VALUES(?,?,?,?,?)`,
		id, userID, tokenHash, expiresAt, nowUTC())
	return err
}

func (r *TokenRepo) FindPasswordReset(ctx context.Context, tokenHash string) (string, string, bool, error) {
	var userID, expires string
	var used sql.NullString
	err := r.DB.QueryRowContext(ctx,
		`SELECT user_id, expires_at, used_at FROM password_resets WHERE token_hash=?`, tokenHash).Scan(&userID, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, repository.ErrNotFound
	}
	return userID, expires, used.Valid, err
}

func (r *TokenRepo) MarkPasswordResetUsed(ctx context.Context, tokenHash string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE password_resets SET used_at=? WHERE token_hash=?`, nowUTC(), tokenHash)
	return err
}

func (r *TokenRepo) DeleteExpiredResets(ctx context.Context, now string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM password_resets WHERE expires_at < ? OR used_at IS NOT NULL`, now)
	return err
}

func (r *TokenRepo) CreateEmailVerification(ctx context.Context, id, userID, tokenHash, expiresAt string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO email_verifications(id, user_id, token_hash, expires_at, created_at) VALUES(?,?,?,?,?)`,
		id, userID, tokenHash, expiresAt, nowUTC())
	return err
}

func (r *TokenRepo) FindEmailVerification(ctx context.Context, tokenHash string) (string, string, bool, error) {
	var userID, expires string
	var used sql.NullString
	err := r.DB.QueryRowContext(ctx,
		`SELECT user_id, expires_at, used_at FROM email_verifications WHERE token_hash=?`, tokenHash).Scan(&userID, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, repository.ErrNotFound
	}
	return userID, expires, used.Valid, err
}

func (r *TokenRepo) MarkEmailVerificationUsed(ctx context.Context, tokenHash string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE email_verifications SET used_at=? WHERE token_hash=?`, nowUTC(), tokenHash)
	return err
}

func (r *TokenRepo) DeleteExpiredVerifications(ctx context.Context, now string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM email_verifications WHERE expires_at < ? OR used_at IS NOT NULL`, now)
	return err
}
