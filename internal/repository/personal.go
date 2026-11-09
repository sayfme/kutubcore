package repository

import (
	"context"

	"kutubcore/internal/domain/collection"
	"kutubcore/internal/domain/reading"
	"kutubcore/internal/domain/review"
)

// PersonalFilter narrows a user's personal library listing.
type PersonalFilter struct {
	Query          string // book title (LIKE)
	PhysicalStatus string
	DigitalStatus  string
	ReadingStatus  string
	Favorite       *bool
}

// PersonalRepository persists per-user library data. Every method takes
// userID and must verify ownership (Blueprint §4: personal records checked
// against the authenticated user on every request).
type PersonalRepository interface {
	ListUserBooks(ctx context.Context, userID string, f PersonalFilter, p Page) ([]collection.UserBook, int, error)
	GetUserBook(ctx context.Context, userID, id string) (*collection.UserBook, error)
	CreateUserBook(ctx context.Context, ub *collection.UserBook) error
	UpdateUserBook(ctx context.Context, ub *collection.UserBook) error
	DeleteUserBook(ctx context.Context, userID, id string) error

	GetProgress(ctx context.Context, userBookID string) (*reading.Progress, error)
	UpsertProgress(ctx context.Context, p *reading.Progress) error

	ListSessions(ctx context.Context, userBookID string) ([]reading.Session, error)
	CreateSession(ctx context.Context, s *reading.Session) error

	ListNotes(ctx context.Context, userBookID string) ([]review.Note, error)
	GetNote(ctx context.Context, userBookID, noteID string) (*review.Note, error)
	CreateNote(ctx context.Context, n *review.Note) error
	UpdateNote(ctx context.Context, n *review.Note) error
	DeleteNote(ctx context.Context, userBookID, noteID string) error

	GetReview(ctx context.Context, userBookID string) (*review.Review, error)
	UpsertReview(ctx context.Context, r *review.Review) error
	DeleteReview(ctx context.Context, userBookID string) error

	Dashboard(ctx context.Context, userID string) (*Dashboard, error)
}

// Dashboard is the personal metrics view (Blueprint §11).
// Catalog metrics must be visually separated from personal metrics.
type Dashboard struct {
	TotalBooks        int `json:"total_books"`
	PhysicalOwned     int `json:"physical_owned"`
	DigitalDownloaded int `json:"digital_downloaded"`
	CurrentlyReading  int `json:"currently_reading"`
	Completed         int `json:"completed"`
	WantToRead        int `json:"want_to_read"`
	Favorites         int `json:"favorites"`

	RecentSessions []reading.Session     `json:"recent_sessions,omitempty"`
	RecentlyAdded  []collection.UserBook `json:"recently_added,omitempty"`
}
