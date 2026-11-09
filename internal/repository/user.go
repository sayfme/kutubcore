package repository

import (
	"context"

	"kutubcore/internal/domain/user"
)

// UserRepository persists users and role assignments.
type UserRepository interface {
	Create(ctx context.Context, u *user.User, passwordHash string) error
	GetByID(ctx context.Context, id string) (*user.User, error)
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	GetByMobile(ctx context.Context, mobile string) (*user.User, error)
	PasswordHash(ctx context.Context, userID string) (string, error)
	UpdatePasswordHash(ctx context.Context, userID, hash string) error
	MarkEmailVerified(ctx context.Context, userID string) error
	AddRole(ctx context.Context, userID, roleCode string) error
	Roles(ctx context.Context, userID string) ([]string, error)
}
