package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"kutubcore/internal/domain/files"
	"kutubcore/internal/domain/id"
	"kutubcore/internal/repository"
	"kutubcore/internal/storage"
)

// FileService implements upload/list/download rules (Blueprint §8).
type FileService struct {
	Files    repository.FileRepository
	Books    repository.BookRepository
	Audit    repository.AuditRepository
	FileDir  string
	MaxBytes int64 // default 50 MiB
}

// checksum helpers + allowlists
var allowedBookFormats = map[string]bool{"pdf": true, "epub": true}
var allowedCoverFormats = map[string]bool{"jpg": true, "jpeg": true, "png": true, "webp": true}

func (s *FileService) maxBytes() int64 {
	if s.MaxBytes > 0 {
		return s.MaxBytes
	}
	return 50 << 20
}

// UploadBookFile handles a multipart upload for a book file (PDF/EPUB).
func (s *FileService) UploadBookFile(ctx context.Context, actorID, bookID, filename, format string, data []byte) (*files.File, error) {
	if len(data) == 0 {
		return nil, validationErr("file is empty")
	}
	if int64(len(data)) > s.maxBytes() {
		return nil, validationErr("file too large (max %d bytes)", s.maxBytes())
	}
	if _, err := s.Books.GetBook(ctx, bookID); err != nil {
		return nil, err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = extOf(filename)
	}
	if !allowedBookFormats[format] {
		return nil, validationErr("unsupported file format %q (allowed: pdf, epub)", format)
	}
	rel, err := storage.Save(s.FileDir, "books", data, "."+format)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	now := now()
	f := &files.File{
		ID: id.Must(), BookID: bookID, FileName: safeDisplayName(filename), FileFormat: format,
		FileSize: int64(len(data)), ChecksumSHA256: hex.EncodeToString(sum[:]), MimeType: mimeFor(format), CreatedAt: now,
	}
	if err := s.Files.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	loc := &files.Location{BookFileID: f.ID, LocationType: "local", StoragePath: rel, IsAvailable: true}
	if err := s.Files.AddLocation(ctx, loc); err != nil {
		return nil, err
	}
	f.Locations = []files.Location{*loc}
	_ = s.Audit.Log(ctx, actorID, "file", f.ID, "create", "", fmt.Sprintf(`{"book_id":"%s","format":"%s"}`, bookID, format))
	return f, nil
}

// AddExternalLocation records an external URL (does NOT mean downloaded, rule 10:
// IsAvailable stays false until the file is actually downloaded).
func (s *FileService) AddExternalLocation(ctx context.Context, actorID, bookID, externalURL, format string) (*files.File, error) {
	externalURL = strings.TrimSpace(externalURL)
	if externalURL == "" || (!strings.HasPrefix(externalURL, "http://") && !strings.HasPrefix(externalURL, "https://")) {
		return nil, validationErr("external_url must be http(s)")
	}
	if _, err := s.Books.GetBook(ctx, bookID); err != nil {
		return nil, err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "pdf"
	}
	if !allowedBookFormats[format] && format != "cover" {
		return nil, validationErr("unsupported file format %q", format)
	}
	f := &files.File{ID: id.Must(), BookID: bookID, FileName: externalURL, FileFormat: format, MimeType: mimeFor(format)}
	if err := s.Files.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	loc := &files.Location{BookFileID: f.ID, LocationType: "external_url", ExternalURL: externalURL, IsAvailable: false}
	if err := s.Files.AddLocation(ctx, loc); err != nil {
		return nil, err
	}
	f.Locations = []files.Location{*loc}
	_ = s.Audit.Log(ctx, actorID, "file", f.ID, "create", "", fmt.Sprintf(`{"book_id":"%s","external_url":"%s"}`, bookID, externalURL))
	return f, nil
}

// UploadCover handles a cover image upload and links it as the book cover.
func (s *FileService) UploadCover(ctx context.Context, actorID, bookID, filename string, data []byte) (*files.File, error) {
	if len(data) == 0 {
		return nil, validationErr("file is empty")
	}
	if int64(len(data)) > 5<<20 {
		return nil, validationErr("cover too large (max 5 MiB)")
	}
	if _, err := s.Books.GetBook(ctx, bookID); err != nil {
		return nil, err
	}
	format := extOf(filename)
	if !allowedCoverFormats[format] {
		return nil, validationErr("unsupported cover format %q (allowed: jpg, png, webp)", format)
	}
	rel, err := storage.Save(s.FileDir, "covers", data, "."+format)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	f := &files.File{
		ID: id.Must(), BookID: bookID, FileName: safeDisplayName(filename), FileFormat: "cover",
		FileSize: int64(len(data)), ChecksumSHA256: hex.EncodeToString(sum[:]), MimeType: mimeFor(format),
	}
	if err := s.Files.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	loc := &files.Location{BookFileID: f.ID, LocationType: "local", StoragePath: rel, IsAvailable: true}
	if err := s.Files.AddLocation(ctx, loc); err != nil {
		return nil, err
	}
	f.Locations = []files.Location{*loc}
	if err := s.Files.SetCover(ctx, bookID, f.ID); err != nil {
		return nil, err
	}
	_ = s.Audit.Log(ctx, actorID, "file", f.ID, "cover", "", fmt.Sprintf(`{"book_id":"%s"}`, bookID))
	return f, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func mimeFor(format string) string {
	switch format {
	case "pdf":
		return "application/pdf"
	case "epub":
		return "application/epub+zip"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

func extOf(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 || i == len(name)-1 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}

func safeDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	// Strip any path prefix the client sent (defensive).
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndex(name, "\\"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "file"
	}
	return name
}
