package service

import (
	"context"
	"encoding/json"
	"strings"

	"kutubcore/internal/domain/bookshop"
	"kutubcore/internal/domain/id"
	"kutubcore/internal/domain/source"
	"kutubcore/internal/repository"
)

// DirectoryService implements bookshop/source rules.
type DirectoryService struct {
	Dirs  repository.DirectoryRepository
	Audit repository.AuditRepository
}

func (s *DirectoryService) CreateBookshop(ctx context.Context, actorID string, b *bookshop.Bookshop) (*bookshop.Bookshop, error) {
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		return nil, validationErr("name is required")
	}
	b.ShopType = strings.ToLower(strings.TrimSpace(b.ShopType))
	if b.ShopType == "" {
		b.ShopType = "both"
	}
	if !bookshop.ShopTypes[b.ShopType] {
		return nil, validationErr("invalid shop_type %q", b.ShopType)
	}
	if b.Website != "" && !strings.HasPrefix(b.Website, "http://") && !strings.HasPrefix(b.Website, "https://") {
		return nil, validationErr("website must be http(s)")
	}
	b.ID = id.Must()
	if err := s.Dirs.CreateBookshop(ctx, b); err != nil {
		return nil, err
	}
	created, _ := s.Dirs.GetBookshop(ctx, b.ID)
	after, _ := json.Marshal(created)
	_ = s.Audit.Log(ctx, actorID, "bookshop", b.ID, "create", "", string(after))
	return created, nil
}

func (s *DirectoryService) UpdateBookshop(ctx context.Context, actorID string, b *bookshop.Bookshop) (*bookshop.Bookshop, error) {
	existing, err := s.Dirs.GetBookshop(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	before, _ := json.Marshal(existing)
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		return nil, validationErr("name is required")
	}
	b.ShopType = strings.ToLower(strings.TrimSpace(b.ShopType))
	if b.ShopType == "" {
		b.ShopType = existing.ShopType
	}
	if !bookshop.ShopTypes[b.ShopType] {
		return nil, validationErr("invalid shop_type %q", b.ShopType)
	}
	if b.Version > 0 && existing.Version != b.Version {
		return nil, validationErr("version conflict: expected %d, got %d", existing.Version, b.Version)
	}
	if err := s.Dirs.UpdateBookshop(ctx, b); err != nil {
		return nil, err
	}
	updated, _ := s.Dirs.GetBookshop(ctx, b.ID)
	after, _ := json.Marshal(updated)
	_ = s.Audit.Log(ctx, actorID, "bookshop", b.ID, "update", string(before), string(after))
	return updated, nil
}

func (s *DirectoryService) DeleteBookshop(ctx context.Context, actorID, shopID string) error {
	existing, err := s.Dirs.GetBookshop(ctx, shopID)
	if err != nil {
		return err
	}
	before, _ := json.Marshal(existing)
	if err := s.Dirs.SoftDeleteBookshop(ctx, shopID); err != nil {
		return err
	}
	_ = s.Audit.Log(ctx, actorID, "bookshop", shopID, "delete", string(before), "")
	return nil
}

func (s *DirectoryService) AddBookshopLink(ctx context.Context, shopID string, l *bookshop.Link) (*bookshop.Link, error) {
	if _, err := s.Dirs.GetBookshop(ctx, shopID); err != nil {
		return nil, err
	}
	l.Label = strings.TrimSpace(l.Label)
	l.URL = strings.TrimSpace(l.URL)
	if l.Label == "" || l.URL == "" {
		return nil, validationErr("label and url are required")
	}
	if !strings.HasPrefix(l.URL, "http://") && !strings.HasPrefix(l.URL, "https://") {
		return nil, validationErr("url must be http(s)")
	}
	l.BookshopID = shopID
	if err := s.Dirs.AddBookshopLink(ctx, l); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, validationErr("link already exists")
		}
		return nil, err
	}
	return l, nil
}

func (s *DirectoryService) CreateSource(ctx context.Context, actorID string, s2 *source.Source) (*source.Source, error) {
	s2.Name = strings.TrimSpace(s2.Name)
	if s2.Name == "" {
		return nil, validationErr("name is required")
	}
	s2.SourceType = strings.ToLower(strings.TrimSpace(s2.SourceType))
	if s2.SourceType == "" {
		s2.SourceType = "other"
	}
	if !source.SourceTypes[s2.SourceType] {
		return nil, validationErr("invalid source_type %q", s2.SourceType)
	}
	s2.ID = id.Must()
	if err := s.Dirs.CreateSource(ctx, s2); err != nil {
		return nil, err
	}
	created, _ := s.Dirs.GetSource(ctx, s2.ID)
	after, _ := json.Marshal(created)
	_ = s.Audit.Log(ctx, actorID, "source", s2.ID, "create", "", string(after))
	return created, nil
}

func (s *DirectoryService) UpdateSource(ctx context.Context, actorID string, s2 *source.Source) (*source.Source, error) {
	existing, err := s.Dirs.GetSource(ctx, s2.ID)
	if err != nil {
		return nil, err
	}
	before, _ := json.Marshal(existing)
	s2.Name = strings.TrimSpace(s2.Name)
	if s2.Name == "" {
		return nil, validationErr("name is required")
	}
	s2.SourceType = strings.ToLower(strings.TrimSpace(s2.SourceType))
	if s2.SourceType == "" {
		s2.SourceType = existing.SourceType
	}
	if !source.SourceTypes[s2.SourceType] {
		return nil, validationErr("invalid source_type %q", s2.SourceType)
	}
	if s2.Version > 0 && existing.Version != s2.Version {
		return nil, validationErr("version conflict: expected %d, got %d", existing.Version, s2.Version)
	}
	if err := s.Dirs.UpdateSource(ctx, s2); err != nil {
		return nil, err
	}
	updated, _ := s.Dirs.GetSource(ctx, s2.ID)
	after, _ := json.Marshal(updated)
	_ = s.Audit.Log(ctx, actorID, "source", s2.ID, "update", string(before), string(after))
	return updated, nil
}

func (s *DirectoryService) DeleteSource(ctx context.Context, actorID, sourceID string) error {
	existing, err := s.Dirs.GetSource(ctx, sourceID)
	if err != nil {
		return err
	}
	before, _ := json.Marshal(existing)
	if err := s.Dirs.SoftDeleteSource(ctx, sourceID); err != nil {
		return err
	}
	_ = s.Audit.Log(ctx, actorID, "source", sourceID, "delete", string(before), "")
	return nil
}

func (s *DirectoryService) AddSourceLink(ctx context.Context, sourceID string, l *source.Link) (*source.Link, error) {
	if _, err := s.Dirs.GetSource(ctx, sourceID); err != nil {
		return nil, err
	}
	l.Label = strings.TrimSpace(l.Label)
	l.URL = strings.TrimSpace(l.URL)
	if l.Label == "" || l.URL == "" {
		return nil, validationErr("label and url are required")
	}
	if !strings.HasPrefix(l.URL, "http://") && !strings.HasPrefix(l.URL, "https://") {
		return nil, validationErr("url must be http(s)")
	}
	l.SourceID = sourceID
	if err := s.Dirs.AddSourceLink(ctx, l); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, validationErr("link already exists")
		}
		return nil, err
	}
	return l, nil
}
