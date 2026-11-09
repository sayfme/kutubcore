package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kutubcore/internal/domain/book"
	"kutubcore/internal/domain/id"
	"kutubcore/internal/domain/writer"
	"kutubcore/internal/repository"
)

// CatalogService implements book/writer business rules (Blueprint §5, §7).
type CatalogService struct {
	Books   repository.BookRepository
	Writers repository.WriterRepository
	Audit   repository.AuditRepository
}

var ErrDuplicate = errors.New("duplicate book possible")

func (s *CatalogService) CreateBook(ctx context.Context, actorID string, b *book.Book, force bool) (*book.Book, error) {
	b.Title = strings.TrimSpace(b.Title)
	if b.Title == "" {
		return nil, validationErr("title is required")
	}
	// Normalize writer IDs for duplicate check.
	var writerIDs []string
	for _, w := range b.Writers {
		if w.ID != "" {
			writerIDs = append(writerIDs, w.ID)
		}
	}
	if dup, err := s.Books.FindDuplicate(ctx, b.Title, writerIDs, ""); err == nil && dup != nil && !force {
		return nil, fmt.Errorf("%w: %q (%s)", ErrDuplicate, dup.Title, dup.ID)
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	b.ID = id.Must()
	b.CreatedBy = actorID
	if err := s.Books.CreateBook(ctx, b); err != nil {
		return nil, err
	}
	created, _ := s.Books.GetBook(ctx, b.ID)
	after, _ := json.Marshal(created)
	_ = s.Audit.Log(ctx, actorID, "book", b.ID, "create", "", string(after))
	return created, nil
}

func (s *CatalogService) UpdateBook(ctx context.Context, actorID string, b *book.Book) (*book.Book, error) {
	existing, err := s.Books.GetBook(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	before, _ := json.Marshal(existing)
	b.Title = strings.TrimSpace(b.Title)
	if b.Title == "" {
		return nil, validationErr("title is required")
	}
	// Optimistic concurrency (Blueprint §9).
	if b.Version > 0 && existing.Version != b.Version {
		return nil, validationErr("version conflict: expected %d, got %d", existing.Version, b.Version)
	}
	if err := s.Books.UpdateBook(ctx, b); err != nil {
		return nil, err
	}
	updated, _ := s.Books.GetBook(ctx, b.ID)
	after, _ := json.Marshal(updated)
	_ = s.Audit.Log(ctx, actorID, "book", b.ID, "update", string(before), string(after))
	return updated, nil
}

func (s *CatalogService) DeleteBook(ctx context.Context, actorID, bookID string) error {
	existing, err := s.Books.GetBook(ctx, bookID)
	if err != nil {
		return err
	}
	before, _ := json.Marshal(existing)
	if err := s.Books.SoftDeleteBook(ctx, bookID); err != nil {
		return err
	}
	_ = s.Audit.Log(ctx, actorID, "book", bookID, "delete", string(before), "")
	return nil
}

func (s *CatalogService) CreateEdition(ctx context.Context, actorID, bookID string, e *book.Edition) (*book.Edition, error) {
	if _, err := s.Books.GetBook(ctx, bookID); err != nil {
		return nil, err
	}
	e.BookID = bookID
	e.Format = strings.ToLower(strings.TrimSpace(e.Format))
	if e.Format == "" {
		return nil, validationErr("format is required")
	}
	if !book.Formats[e.Format] {
		return nil, validationErr("invalid format %q", e.Format)
	}
	if e.PageCount < 0 {
		return nil, validationErr("page_count must be > 0")
	}
	if e.PublicationYear != 0 && (e.PublicationYear < 1000 || e.PublicationYear > 2200) {
		return nil, validationErr("publication_year out of range")
	}
	e.ID = id.Must()
	if err := s.Books.CreateEdition(ctx, e); err != nil {
		// Map FK failure on language to validation.
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return nil, validationErr("invalid language_id")
		}
		return nil, err
	}
	after, _ := json.Marshal(e)
	_ = s.Audit.Log(ctx, actorID, "edition", e.ID, "create", "", string(after))
	return e, nil
}

func (s *CatalogService) CreateWriter(ctx context.Context, actorID string, w *writer.Writer) (*writer.Writer, error) {
	w.FullName = strings.TrimSpace(w.FullName)
	if w.FullName == "" {
		return nil, validationErr("full_name is required")
	}
	if w.Website != "" && !strings.HasPrefix(w.Website, "http://") && !strings.HasPrefix(w.Website, "https://") {
		return nil, validationErr("website must be http(s)")
	}
	w.ID = id.Must()
	if err := s.Writers.CreateWriter(ctx, w); err != nil {
		return nil, err
	}
	created, _ := s.Writers.GetWriter(ctx, w.ID)
	after, _ := json.Marshal(created)
	_ = s.Audit.Log(ctx, actorID, "writer", w.ID, "create", "", string(after))
	return created, nil
}

func (s *CatalogService) UpdateWriter(ctx context.Context, actorID string, w *writer.Writer) (*writer.Writer, error) {
	existing, err := s.Writers.GetWriter(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	before, _ := json.Marshal(existing)
	w.FullName = strings.TrimSpace(w.FullName)
	if w.FullName == "" {
		return nil, validationErr("full_name is required")
	}
	if w.Version > 0 && existing.Version != w.Version {
		return nil, validationErr("version conflict: expected %d, got %d", existing.Version, w.Version)
	}
	if err := s.Writers.UpdateWriter(ctx, w); err != nil {
		return nil, err
	}
	updated, _ := s.Writers.GetWriter(ctx, w.ID)
	after, _ := json.Marshal(updated)
	_ = s.Audit.Log(ctx, actorID, "writer", w.ID, "update", string(before), string(after))
	return updated, nil
}

func (s *CatalogService) DeleteWriter(ctx context.Context, actorID, writerID string) error {
	existing, err := s.Writers.GetWriter(ctx, writerID)
	if err != nil {
		return err
	}
	before, _ := json.Marshal(existing)
	if err := s.Writers.SoftDeleteWriter(ctx, writerID); err != nil {
		return err
	}
	_ = s.Audit.Log(ctx, actorID, "writer", writerID, "delete", string(before), "")
	return nil
}
