package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"kutubcore/internal/domain/user"
	"kutubcore/internal/repository"
)

var _ repository.UserRepository = (*UserRepo)(nil)

// UserRepo is the SQLite UserRepository.
type UserRepo struct{ DB *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{DB: db} }

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func (r *UserRepo) Create(ctx context.Context, u *user.User, passwordHash string) error {
	if u.ID == "" {
		return fmt.Errorf("user id required")
	}
	now := nowUTC()
	email := sql.NullString{}
	if u.Email != "" {
		email = sql.NullString{String: strings.ToLower(strings.TrimSpace(u.Email)), Valid: true}
	}
	mobile := sql.NullString{}
	if u.Mobile != "" {
		mobile = sql.NullString{String: strings.TrimSpace(u.Mobile), Valid: true}
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO users(id, display_name, email, mobile, password_hash, is_active, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		u.ID, strings.TrimSpace(u.DisplayName), email, mobile, passwordHash, 1, now, now)
	if err != nil {
		return err
	}
	// Every registered user starts as Reader (Blueprint §4).
	return r.AddRole(ctx, u.ID, user.RoleReader)
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	return r.scanOne(ctx,
		`SELECT id, display_name, email, mobile, is_active, email_verified_at, created_at, updated_at FROM users WHERE id=? AND is_active=1`, id)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return r.scanOne(ctx,
		`SELECT id, display_name, email, mobile, is_active, email_verified_at, created_at, updated_at FROM users WHERE email=? AND is_active=1`,
		strings.ToLower(strings.TrimSpace(email)))
}

func (r *UserRepo) GetByMobile(ctx context.Context, mobile string) (*user.User, error) {
	return r.scanOne(ctx,
		`SELECT id, display_name, email, mobile, is_active, email_verified_at, created_at, updated_at FROM users WHERE mobile=? AND is_active=1`,
		strings.TrimSpace(mobile))
}

func (r *UserRepo) scanOne(ctx context.Context, query string, arg string) (*user.User, error) {
	var (
		u             user.User
		email, mobile sql.NullString
		active        int
		verified      sql.NullString
	)
	err := r.DB.QueryRowContext(ctx, query, arg).Scan(&u.ID, &u.DisplayName, &email, &mobile, &active, &verified, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = email.String
	}
	if mobile.Valid {
		u.Mobile = mobile.String
	}
	u.IsActive = active == 1
	u.EmailVerified = verified.Valid && verified.String != ""
	roles, err := r.Roles(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	u.Roles = roles
	return &u, nil
}

func (r *UserRepo) PasswordHash(ctx context.Context, userID string) (string, error) {
	var h sql.NullString
	err := r.DB.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=?`, userID).Scan(&h)
	if errors.Is(err, sql.ErrNoRows) {
		return "", repository.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !h.Valid {
		return "", nil
	}
	return h.String, nil
}

func (r *UserRepo) UpdatePasswordHash(ctx context.Context, userID, hash string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE users SET password_hash=?, updated_at=? WHERE id=?`, hash, nowUTC(), userID)
	return err
}

func (r *UserRepo) MarkEmailVerified(ctx context.Context, userID string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE users SET email_verified_at=?, updated_at=? WHERE id=?`, nowUTC(), nowUTC(), userID)
	return err
}

func (r *UserRepo) AddRole(ctx context.Context, userID, roleCode string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT OR IGNORE INTO user_roles(user_id, role_id, created_at)
		 SELECT ?, id, ? FROM roles WHERE code=?`, userID, nowUTC(), roleCode)
	return err
}

func (r *UserRepo) Roles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT r.code FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=? ORDER BY r.code`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}
