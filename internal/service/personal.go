package service

import (
	"context"
	"strings"
	"time"

	"kutubcore/internal/domain/collection"
	"kutubcore/internal/domain/id"
	"kutubcore/internal/domain/reading"
	"kutubcore/internal/domain/review"
	"kutubcore/internal/repository"
)

// PersonalService implements personal-library rules (Blueprint §5, §7).
// Ownership is always derived from the authenticated user, never the client.
type PersonalService struct {
	Personal repository.PersonalRepository
	Books    repository.BookRepository // for existence checks + titles
	// Acquisitions validates rule 1 (owned requires a copy) and Files
	// validates rule 2 (downloaded requires an available file).
	Acquisitions repository.AcquisitionRepository
	Files        repository.FileRepository
}

// checkCollectionRules enforces Blueprint §7:
// 1) owned requires >=1 physical copy record
// 2) downloaded requires >=1 available local/accessible file
// Returns a 422 validation error when the rule is violated.
func (s *PersonalService) checkCollectionRules(ctx context.Context, userID, bookID, physical, digital string) error {
	if physical == "owned" {
		if s.Acquisitions != nil {
			has, err := s.Acquisitions.HasCopyForBook(ctx, userID, bookID)
			if err != nil {
				return err
			}
			if !has {
				return validationErr("owned requires at least one physical copy — record an acquisition first")
			}
		}
	}
	if digital == "downloaded" {
		if s.Files != nil {
			has, err := s.Files.HasAvailableFile(ctx, bookID)
			if err != nil {
				return err
			}
			if !has {
				return validationErr("downloaded requires an available digital file — upload a PDF/EPUB or mark one available first")
			}
		}
	}
	return nil
}

func (s *PersonalService) AddToLibrary(ctx context.Context, userID string, bookID string, physical, digital, readingStatus string, favorite bool) (*collection.UserBook, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, validationErr("book_id is required")
	}
	if _, err := s.Books.GetBook(ctx, bookID); err != nil {
		return nil, err // 404 if catalog book missing/deleted
	}
	physical, digital, readingStatus = defaultIfEmpty(physical, digital, readingStatus)
	if err := validateStatuses(physical, digital, readingStatus); err != nil {
		return nil, err
	}
	if err := s.checkCollectionRules(ctx, userID, bookID, physical, digital); err != nil {
		return nil, err
	}
	ub := &collection.UserBook{ID: id.Must(), UserID: userID, BookID: bookID,
		PhysicalStatus: physical, DigitalStatus: digital, ReadingStatus: readingStatus, IsFavorite: favorite}
	if err := s.Personal.CreateUserBook(ctx, ub); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, validationErr("book already in library")
		}
		return nil, err
	}
	return s.Personal.GetUserBook(ctx, userID, ub.ID)
}

func (s *PersonalService) UpdateStatuses(ctx context.Context, userID string, ub *collection.UserBook) (*collection.UserBook, error) {
	// Load current to keep untouched fields; callers pass pointers via ub fields
	// that were applied on top of existing values in the handler.
	existing, err := s.Personal.GetUserBook(ctx, userID, ub.ID)
	if err != nil {
		return nil, err
	}
	// Preserve fields the handler left empty (PATCH semantics).
	if ub.PhysicalStatus == "" {
		ub.PhysicalStatus = existing.PhysicalStatus
	}
	if ub.DigitalStatus == "" {
		ub.DigitalStatus = existing.DigitalStatus
	}
	if ub.ReadingStatus == "" {
		ub.ReadingStatus = existing.ReadingStatus
	}
	// IsFavorite: handler sets a pointer-backed flag; 0/false means unset here
	// is ambiguous, so the handler must call SetFavorite separately when needed.
	// For full-field updates this block is used directly.
	if err := validateStatuses(ub.PhysicalStatus, ub.DigitalStatus, ub.ReadingStatus); err != nil {
		return nil, err
	}
	// Enforce collection rules when transitioning to owned/downloaded,
	// or when the stored row already violates them (revalidate on write).
	if err := s.checkCollectionRules(ctx, userID, existing.BookID, ub.PhysicalStatus, ub.DigitalStatus); err != nil {
		return nil, err
	}
	ub.UserID = userID
	ub.BookID = existing.BookID // never allow changing book via PATCH
	if err := s.Personal.UpdateUserBook(ctx, ub); err != nil {
		return nil, err
	}
	return s.Personal.GetUserBook(ctx, userID, ub.ID)
}

func (s *PersonalService) SetFavorite(ctx context.Context, userID, id string, fav bool) (*collection.UserBook, error) {
	existing, err := s.Personal.GetUserBook(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	existing.IsFavorite = fav
	existing.UserID = userID
	if err := s.Personal.UpdateUserBook(ctx, existing); err != nil {
		return nil, err
	}
	return s.Personal.GetUserBook(ctx, userID, id)
}

func (s *PersonalService) UpdateProgress(ctx context.Context, userID, id string, currentPage, totalPages int) (*reading.Progress, error) {
	ub, err := s.Personal.GetUserBook(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if currentPage < 0 || totalPages < 0 {
		return nil, validationErr("current_page and total_pages must be >= 0")
	}
	if totalPages > 0 && currentPage > totalPages {
		return nil, validationErr("current_page cannot exceed total_pages")
	}
	percent := 0.0
	if totalPages > 0 {
		percent = float64(currentPage) / float64(totalPages) * 100
	}
	existing, _ := s.Personal.GetProgress(ctx, ub.ID)
	p := &reading.Progress{UserBookID: ub.ID, CurrentPage: currentPage, TotalPages: totalPages, ProgressPercent: percent}
	if existing != nil {
		p.ID = existing.ID
		p.StartedAt = existing.StartedAt
		p.CompletedAt = existing.CompletedAt
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if p.StartedAt == "" && (currentPage > 0 || ub.ReadingStatus == "currently_reading") {
		p.StartedAt = now
	}
	if percent >= 100 && totalPages > 0 {
		if p.CompletedAt == "" {
			p.CompletedAt = now
		}
		// Keep reading status consistent: completed (Rule 3).
		if ub.ReadingStatus != "completed" {
			ub.ReadingStatus = "completed"
			ub.UserID = userID
			_ = s.Personal.UpdateUserBook(ctx, ub)
		}
	}
	if err := s.Personal.UpsertProgress(ctx, p); err != nil {
		return nil, err
	}
	// Re-fetch to get the stored ID/timestamps.
	return s.Personal.GetProgress(ctx, ub.ID)
}

func (s *PersonalService) AddSession(ctx context.Context, userID, id string, sess *reading.Session) (*reading.Session, error) {
	ub, err := s.Personal.GetUserBook(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if sess.StartedAt == "" {
		sess.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if sess.PagesRead < 0 {
		return nil, validationErr("pages_read must be >= 0")
	}
	if sess.EndedAt != "" && sess.EndedAt < sess.StartedAt {
		return nil, validationErr("ended_at must be >= started_at")
	}
	sess.ID = idMust()
	sess.UserBookID = ub.ID
	if err := s.Personal.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *PersonalService) CreateNote(ctx context.Context, userID, id string, n *review.Note) (*review.Note, error) {
	ub, err := s.Personal.GetUserBook(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	n.Content = strings.TrimSpace(n.Content)
	if n.Content == "" {
		return nil, validationErr("content is required")
	}
	if n.PageNumber < 0 {
		return nil, validationErr("page_number must be >= 0")
	}
	n.Visibility = "private" // Notes are private by default (Rule 5).
	n.ID = idMust()
	n.UserBookID = ub.ID
	if err := s.Personal.CreateNote(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *PersonalService) UpdateNote(ctx context.Context, userID, id, noteID string, title, content string, page *int) (*review.Note, error) {
	ub, err := s.Personal.GetUserBook(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	n, err := s.Personal.GetNote(ctx, ub.ID, noteID)
	if err != nil {
		return nil, err
	}
	if content != "" {
		n.Content = content
	}
	if title != "" {
		n.Title = title
	}
	if page != nil {
		if *page < 0 {
			return nil, validationErr("page_number must be >= 0")
		}
		n.PageNumber = *page
	}
	if strings.TrimSpace(n.Content) == "" {
		return nil, validationErr("content is required")
	}
	n.Visibility = "private"
	if err := s.Personal.UpdateNote(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *PersonalService) UpsertReview(ctx context.Context, userID, id string, rating *int, reaction, text string) (*review.Review, error) {
	ub, err := s.Personal.GetUserBook(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return nil, validationErr("rating must be 1-5")
	}
	if reaction != "" && !review.Reactions[reaction] {
		return nil, validationErr("invalid reaction")
	}
	existing, _ := s.Personal.GetReview(ctx, ub.ID)
	rv := &review.Review{UserBookID: ub.ID}
	if existing != nil {
		rv.ID = existing.ID
		rv.CreatedAt = existing.CreatedAt
	}
	if rating != nil {
		rv.Rating = *rating
	} else if existing != nil {
		rv.Rating = existing.Rating
	}
	if reaction != "" {
		rv.Reaction = reaction
	} else if existing != nil {
		rv.Reaction = existing.Reaction
	}
	if text != "" {
		rv.ReviewText = text
	} else if existing != nil {
		rv.ReviewText = existing.ReviewText
	}
	if err := s.Personal.UpsertReview(ctx, rv); err != nil {
		return nil, err
	}
	return s.Personal.GetReview(ctx, ub.ID)
}

func validateStatuses(physical, digital, reading string) error {
	if !collection.PhysicalStatuses[physical] {
		return validationErr("invalid physical_status %q", physical)
	}
	if !collection.DigitalStatuses[digital] {
		return validationErr("invalid digital_status %q", digital)
	}
	if !collection.ReadingStatuses[reading] {
		return validationErr("invalid reading_status %q", reading)
	}
	return nil
}

func defaultIfEmpty(physical, digital, reading string) (string, string, string) {
	if physical == "" {
		physical = "not_collected"
	}
	if digital == "" {
		digital = "not_available"
	}
	if reading == "" {
		reading = "want_to_read"
	}
	return physical, digital, reading
}

func idMust() string { return id.Must() }
