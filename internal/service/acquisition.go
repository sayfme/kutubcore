package service

import (
	"context"
	"strings"

	"kutubcore/internal/domain/acquisition"
	"kutubcore/internal/domain/book"
	"kutubcore/internal/domain/id"
	"kutubcore/internal/repository"
)

// AcquisitionService implements acquisition rules (Blueprint §7).
type AcquisitionService struct {
	Acquisitions repository.AcquisitionRepository
	Books        repository.BookRepository
}

func (s *AcquisitionService) Create(ctx context.Context, userID string, a *acquisition.Acquisition) (*acquisition.Acquisition, error) {
	a.UserID = userID
	if len(a.Items) == 0 {
		return nil, validationErr("at least one item is required")
	}
	if a.CurrencyCode != "" && len(a.CurrencyCode) != 3 {
		return nil, validationErr("currency_code must be 3 letters")
	}
	a.CurrencyCode = strings.ToUpper(strings.TrimSpace(a.CurrencyCode))
	if a.TotalCost < 0 {
		return nil, validationErr("total_cost must be >= 0")
	}
	// Validate source/bookshop presence when provided (FK would fail too,
	// but a friendly 422 beats a 500).
	for i := range a.Items {
		it := &a.Items[i]
		it.BookID = strings.TrimSpace(it.BookID)
		if it.BookID == "" {
			return nil, validationErr("item book_id is required")
		}
		if _, err := s.Books.GetBook(ctx, it.BookID); err != nil {
			return nil, err
		}
		it.Format = strings.ToLower(strings.TrimSpace(it.Format))
		if it.Format == "" {
			it.Format = "paperback"
		}
		if !book.Formats[it.Format] {
			return nil, validationErr("invalid format %q", it.Format)
		}
		if it.Quantity < 0 {
			return nil, validationErr("quantity must be >= 1")
		}
		if it.UnitPrice < 0 {
			return nil, validationErr("unit_price must be >= 0")
		}
	}
	a.ID = id.Must()
	if err := s.Acquisitions.CreateAcquisition(ctx, a); err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return nil, validationErr("invalid source_id, bookshop_id, or edition_id")
		}
		return nil, err
	}
	return s.Acquisitions.GetAcquisition(ctx, userID, a.ID)
}
