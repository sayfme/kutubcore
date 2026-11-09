package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"kutubcore/internal/domain/id"
	"kutubcore/internal/domain/user"
	"kutubcore/internal/repository"
)

// Errors mapped to HTTP codes in handlers.
var (
	ErrValidation = errors.New("validation error")
	// ErrInvalidPassword is declared in password.go.
)

func validationErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// AuthService implements registration, login, sessions (Blueprint §4, §14).
type AuthService struct {
	Users    repository.UserRepository
	Sessions repository.SessionRepository
	TTL      time.Duration
}

func (s *AuthService) Register(ctx context.Context, displayName, email, mobile, password string) (*user.User, error) {
	displayName = strings.TrimSpace(displayName)
	email = strings.ToLower(strings.TrimSpace(email))
	mobile = strings.TrimSpace(mobile)
	if displayName == "" {
		return nil, validationErr("display_name is required")
	}
	if email == "" && mobile == "" {
		return nil, validationErr("at least one of email or mobile is required")
	}
	if email != "" && !looksLikeEmail(email) {
		return nil, validationErr("invalid email")
	}
	if len(password) < 8 {
		return nil, validationErr("password must be at least 8 characters")
	}
	if email != "" {
		if _, err := s.Users.GetByEmail(ctx, email); err == nil {
			return nil, validationErr("email already registered")
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	}
	if mobile != "" {
		if _, err := s.Users.GetByMobile(ctx, mobile); err == nil {
			return nil, validationErr("mobile already registered")
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, validationErr("%s", err.Error())
	}
	uid, err := id.New()
	if err != nil {
		return nil, err
	}
	u := &user.User{ID: uid, DisplayName: displayName, Email: email, Mobile: mobile, IsActive: true}
	if err := s.Users.Create(ctx, u, hash); err != nil {
		return nil, err
	}
	return s.Users.GetByID(ctx, uid)
}

func (s *AuthService) Login(ctx context.Context, identifier, password, userAgent, ip string) (token string, loggedIn *user.User, err error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return "", nil, validationErr("identifier and password are required")
	}
	var u *user.User
	if strings.Contains(identifier, "@") {
		u, err = s.Users.GetByEmail(ctx, identifier)
	} else {
		// Try mobile first, then email (identifier may be either).
		u, err = s.Users.GetByMobile(ctx, identifier)
		if errors.Is(err, repository.ErrNotFound) {
			u, err = s.Users.GetByEmail(ctx, identifier)
		}
	}
	if err != nil {
		return "", nil, ErrInvalidPassword // don't leak which identifier exists
	}
	hash, err := s.Users.PasswordHash(ctx, u.ID)
	if err != nil || hash == "" || !VerifyPassword(hash, password) {
		return "", nil, ErrInvalidPassword
	}
	token, err = newSessionToken()
	if err != nil {
		return "", nil, err
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	sessID, err := id.New()
	if err != nil {
		return "", nil, err
	}
	sess := &repository.Session{
		ID:        sessID,
		UserID:    u.ID,
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().UTC().Add(ttl).Format(time.RFC3339Nano),
		UserAgent: userAgent,
		IP:        ip,
	}
	if err := s.Sessions.Create(ctx, sess); err != nil {
		return "", nil, err
	}
	return token, u, nil
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (*user.User, error) {
	if strings.TrimSpace(token) == "" {
		return nil, repository.ErrNotFound
	}
	sess, err := s.Sessions.GetByTokenHash(ctx, hashToken(token))
	if err != nil {
		return nil, err
	}
	u, err := s.Users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.Sessions.DeleteByTokenHash(ctx, hashToken(token))
}

func (s *AuthService) ActivateOrganizer(ctx context.Context, userID string) ([]string, error) {
	u, err := s.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, r := range u.Roles {
		if r == user.RoleOrganizer {
			return u.Roles, nil // already organizer (idempotent)
		}
	}
	if err := s.Users.AddRole(ctx, userID, user.RoleOrganizer); err != nil {
		return nil, err
	}
	return s.Users.Roles(ctx, userID)
}

func looksLikeEmail(s string) bool {
	at := strings.Index(s, "@")
	return at > 0 && at < len(s)-1 && strings.Contains(s[at+1:], ".")
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := cryptoRand(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
