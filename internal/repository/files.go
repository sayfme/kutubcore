package repository

import (
	"context"

	"kutubcore/internal/domain/files"
)

// FileRepository persists file metadata + locations.
type FileRepository interface {
	ListFiles(ctx context.Context, bookID string) ([]files.File, error)
	GetFile(ctx context.Context, id string) (*files.File, error)
	CreateFile(ctx context.Context, f *files.File) error
	AddLocation(ctx context.Context, loc *files.Location) error
	DeleteFile(ctx context.Context, id string) error
	SetCover(ctx context.Context, bookID, fileID string) error
	// HasAvailableFile reports whether book has >=1 available location (rule 2).
	HasAvailableFile(ctx context.Context, bookID string) (bool, error)
}
