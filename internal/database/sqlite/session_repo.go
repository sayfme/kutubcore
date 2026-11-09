package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"kutubcore/internal/repository"
)

var _ repository.SessionRepository = (*SessionRepo)(nil)

// SessionRepo is the SQLite SessionRepository.
type SessionRepo struct{ DB *sql.DB }

func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{DB: db} }

func (r *SessionRepo) Create(ctx context.Context, s *repository.Session) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, token_hash, expires_at, created_at, user_agent, ip) VALUES(?,?,?,?,?,?,?)`,
		s.ID, s.UserID, s.TokenHash, s.ExpiresAt, nowUTC(), s.UserAgent, s.IP)
	return err
}

func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*repository.Session, error) {
	var s repository.Session
	err := r.DB.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at, user_agent, ip FROM sessions WHERE token_hash=?`, tokenHash).
		Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt, &s.UserAgent, &s.IP)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// RFC3339 strings from toUTC compare lexicographically; parse to be safe.
	exp, err := time.Parse(time.RFC3339Nano, s.ExpiresAt)
	if err != nil {
		if exp2, err2 := time.Parse(time.RFC3339, s.ExpiresAt); err2 == nil {
			exp = exp2
		} else {
			return nil, err
		}
	}
	if exp.Before(time.Now().UTC()) {
		_ = r.DeleteByTokenHash(ctx, tokenHash)
		return nil, repository.ErrNotFound
	}
	return &s, nil
}

func (r *SessionRepo) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, tokenHash)
	return err
}

func (r *SessionRepo) DeleteByUser(ctx context.Context, userID string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}

func (r *SessionRepo) DeleteExpired(ctx context.Context, now string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now)
	return err
}
