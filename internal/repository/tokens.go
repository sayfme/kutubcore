package repository

import "context"

// TokenRepository persists single-use password-reset / email-verification
// tokens (Milestone F). Only token hashes are stored; plaintexts live
// solely in the delivered reset/verify link.
type TokenRepository interface {
	CreatePasswordReset(ctx context.Context, id, userID, tokenHash, expiresAt string) error
	FindPasswordReset(ctx context.Context, tokenHash string) (userID, expiresAt string, used bool, err error)
	MarkPasswordResetUsed(ctx context.Context, tokenHash string) error
	DeleteExpiredResets(ctx context.Context, now string) error

	CreateEmailVerification(ctx context.Context, id, userID, tokenHash, expiresAt string) error
	FindEmailVerification(ctx context.Context, tokenHash string) (userID, expiresAt string, used bool, err error)
	MarkEmailVerificationUsed(ctx context.Context, tokenHash string) error
	DeleteExpiredVerifications(ctx context.Context, now string) error
}
