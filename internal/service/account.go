package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"kutubcore/internal/domain/id"
	"kutubcore/internal/domain/user"
	"kutubcore/internal/repository"
)

// AccountService handles password reset + email verification (Milestone F).
// Plaintext tokens are returned to the handler for delivery (email or, in
// local/dev without SMTP, the server log) and never stored — only their
// SHA-256 hashes persist (repository.TokenRepository).
type AccountService struct {
	Users    repository.UserRepository
	Tokens   repository.TokenRepository
	Sessions repository.SessionRepository
}

const (
	forgotPasswordExpiry = time.Hour
	verifyExpiry         = 72 * time.Hour
)

// ForgotPassword creates a reset token for the identifier, if the account
// exists. Returns the plaintext token or "" when no account matches
// (caller must still answer 200 — no enumeration).
func (s *AccountService) ForgotPassword(ctx context.Context, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", nil
	}
	u, err := s.findByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", nil // don't leak existence
		}
		return "", err
	}
	token, err := newSessionToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().UTC().Add(forgotPasswordExpiry).Format(time.RFC3339Nano)
	_ = s.Tokens.DeleteExpiredResets(ctx, time.Now().UTC().Format(time.RFC3339Nano))
	if err := s.Tokens.CreatePasswordReset(ctx, id.Must(), u.ID, hashToken(token), expires); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword consumes the token and sets a new password, invalidating
// all existing sessions for the user.
func (s *AccountService) ResetPassword(ctx context.Context, token, newPassword string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return validationErr("token is required")
	}
	if len(newPassword) < 8 {
		return validationErr("password must be at least 8 characters")
	}
	th := hashToken(token)
	userID, expires, used, err := s.Tokens.FindPasswordReset(ctx, th)
	if err != nil {
		return validationErr("invalid reset token")
	}
	if used {
		return validationErr("reset token already used")
	}
	exp, err := parseTime(expires)
	if err != nil || exp.Before(time.Now().UTC()) {
		return validationErr("reset token expired")
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return validationErr("%s", err.Error())
	}
	if err := s.Users.UpdatePasswordHash(ctx, userID, hash); err != nil {
		return err
	}
	if err := s.Tokens.MarkPasswordResetUsed(ctx, th); err != nil {
		return err
	}
	// Force re-login on all devices after a reset.
	if s.Sessions != nil {
		_ = s.Sessions.DeleteByUser(ctx, userID)
	}
	return nil
}

// CreateVerification creates a verification token for a user (used on
// registration and resend). Returns plaintext or "" when no email present.
func (s *AccountService) CreateVerification(ctx context.Context, userID string) (string, error) {
	u, err := s.Users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if u.Email == "" || u.EmailVerified {
		return "", nil
	}
	token, err := newSessionToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().UTC().Add(verifyExpiry).Format(time.RFC3339Nano)
	_ = s.Tokens.DeleteExpiredVerifications(ctx, time.Now().UTC().Format(time.RFC3339Nano))
	if err := s.Tokens.CreateEmailVerification(ctx, id.Must(), userID, hashToken(token), expires); err != nil {
		return "", err
	}
	return token, nil
}

// VerifyEmail consumes the verification token and marks the address verified.
func (s *AccountService) VerifyEmail(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return validationErr("token is required")
	}
	th := hashToken(token)
	userID, expires, used, err := s.Tokens.FindEmailVerification(ctx, th)
	if err != nil {
		return validationErr("invalid verification token")
	}
	if used {
		return validationErr("verification token already used")
	}
	exp, err := parseTime(expires)
	if err != nil || exp.Before(time.Now().UTC()) {
		return validationErr("verification token expired")
	}
	if err := s.Tokens.MarkEmailVerificationUsed(ctx, th); err != nil {
		return err
	}
	return s.Users.MarkEmailVerified(ctx, userID)
}

func (s *AccountService) findByIdentifier(ctx context.Context, identifier string) (*user.User, error) {
	if strings.Contains(identifier, "@") {
		return s.Users.GetByEmail(ctx, identifier)
	}
	u, err := s.Users.GetByMobile(ctx, identifier)
	if err != nil && errors.Is(err, repository.ErrNotFound) {
		return s.Users.GetByEmail(ctx, identifier)
	}
	return u, err
}

func parseTime(s string) (time.Time, error) {
	// Go RFC3339Nano/RFC3339 + SQLite datetime('now') bare "YYYY-MM-DD HH:MM:SS".
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errUnparsableTime(s)
}

type timeParseError string

func (e timeParseError) Error() string { return "unparsable time: " + string(e) }

func errUnparsableTime(s string) error { return timeParseError(s) }
