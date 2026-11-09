package repository

import (
	"context"

	"kutubcore/internal/domain/bookshop"
	"kutubcore/internal/domain/source"
)

// DirectoryRepository persists bookshops + collection sources (Milestone D).
type DirectoryRepository interface {
	ListBookshops(ctx context.Context, query string, p Page) ([]bookshop.Bookshop, int, error)
	GetBookshop(ctx context.Context, id string) (*bookshop.Bookshop, error)
	CreateBookshop(ctx context.Context, b *bookshop.Bookshop) error
	UpdateBookshop(ctx context.Context, b *bookshop.Bookshop) error
	SoftDeleteBookshop(ctx context.Context, id string) error

	AddBookshopLink(ctx context.Context, link *bookshop.Link) error
	DeleteBookshopLink(ctx context.Context, bookshopID, linkID string) error

	ListSources(ctx context.Context, query string, p Page) ([]source.Source, int, error)
	GetSource(ctx context.Context, id string) (*source.Source, error)
	CreateSource(ctx context.Context, s *source.Source) error
	UpdateSource(ctx context.Context, s *source.Source) error
	SoftDeleteSource(ctx context.Context, id string) error

	AddSourceLink(ctx context.Context, link *source.Link) error
	DeleteSourceLink(ctx context.Context, sourceID, linkID string) error
}
