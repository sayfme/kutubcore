package repository

import (
	"context"

	"kutubcore/internal/domain/book"
	"kutubcore/internal/domain/writer"
)

// Pagination for list endpoints (Blueprint §9).
type Page struct {
	Limit int
	Page  int // 1-based
}

// BookFilter narrows the public catalog listing.
type BookFilter struct {
	Query    string // matches title / original_title / writer name (LIKE)
	Language string // edition language code (bn, en, ...)
	Genre    string // genre slug
	WriterID string
	Sort     string // created_at | title | updated_at (optional leading -)
}

// BookRepository persists the shared catalog.
type BookRepository interface {
	ListBooks(ctx context.Context, f BookFilter, p Page) ([]book.Book, int, error)
	GetBook(ctx context.Context, id string) (*book.Book, error)
	CreateBook(ctx context.Context, b *book.Book) error
	UpdateBook(ctx context.Context, b *book.Book) error
	SoftDeleteBook(ctx context.Context, id string) error
	RestoreBook(ctx context.Context, id string) error
	ListEditions(ctx context.Context, bookID string) ([]book.Edition, error)
	CreateEdition(ctx context.Context, e *book.Edition) error
	FindDuplicate(ctx context.Context, title string, writerIDs []string, excludeID string) (*book.Book, error)
}

// WriterRepository persists the writer directory.
type WriterRepository interface {
	ListWriters(ctx context.Context, query string, p Page) ([]writer.Writer, int, error)
	GetWriter(ctx context.Context, id string) (*writer.Writer, error)
	CreateWriter(ctx context.Context, w *writer.Writer) error
	UpdateWriter(ctx context.Context, w *writer.Writer) error
	SoftDeleteWriter(ctx context.Context, id string) error
}

// Language and Genre (minimal reference data for Milestone B).
type Language struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Native string `json:"native_name,omitempty"`
	Active bool   `json:"is_active"`
}

type Genre struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"is_active"`
}

// ReferenceRepository reads languages/genres (write paths come in M2+).
type ReferenceRepository interface {
	ListLanguages(ctx context.Context) ([]Language, error)
	ListGenres(ctx context.Context) ([]Genre, error)
}

// AuditRepository writes catalog audit rows (Blueprint §4).
type AuditRepository interface {
	Log(ctx context.Context, actorUserID, entityType, entityID, action, beforeJSON, afterJSON string) error
}
