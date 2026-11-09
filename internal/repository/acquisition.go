package repository

import (
	"context"

	"kutubcore/internal/domain/acquisition"
)

// AcquisitionRepository persists acquisitions, items, and copies.
type AcquisitionRepository interface {
	// CreateAcquisition inserts acquisition + items + copies atomically.
	CreateAcquisition(ctx context.Context, a *acquisition.Acquisition) error
	ListAcquisitions(ctx context.Context, userID string, p Page) ([]acquisition.Acquisition, int, error)
	GetAcquisition(ctx context.Context, userID, id string) (*acquisition.Acquisition, error)
	DeleteAcquisition(ctx context.Context, userID, id string) error

	// ListCopies returns the user's physical/digital copies (inventory).
	ListCopies(ctx context.Context, userID string, bookID string, p Page) ([]acquisition.Copy, int, error)
	// HasCopyForBook reports whether user has >=1 copy of book (rule 1).
	HasCopyForBook(ctx context.Context, userID, bookID string) (bool, error)
}
