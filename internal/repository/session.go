package repository

import "context"

// Session is a server-side login session row.
type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt string
	CreatedAt string
	UserAgent string
	IP        string
}

// SessionRepository persists login sessions (Blueprint §14: server-side
// sessions + HttpOnly cookies; token hash stored, never plaintext).
type SessionRepository interface {
	Create(ctx context.Context, s *Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteByUser(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context, now string) error
}
