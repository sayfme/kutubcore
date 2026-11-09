package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"kutubcore/internal/domain/analytics"
	"kutubcore/internal/domain/shelf"
	"kutubcore/internal/repository"
)

// AnalyticsService aggregates activity, goals, and shelf stages (Milestone G).
type AnalyticsService struct {
	Repo repository.AnalyticsRepository
}

var monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

func (s *AnalyticsService) Analytics(ctx context.Context, userID string, months int) (*analytics.Summary, error) {
	return s.Repo.Analytics(ctx, userID, months)
}

func (s *AnalyticsService) CurrentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

func (s *AnalyticsService) SetGoal(ctx context.Context, userID, periodMonth string, target int) (*analytics.Goal, error) {
	periodMonth = strings.TrimSpace(periodMonth)
	if periodMonth == "" {
		periodMonth = s.CurrentMonth()
	}
	if !monthRe.MatchString(periodMonth) {
		return nil, validationErr("period_month must be YYYY-MM")
	}
	if target < 1 || target > 100 {
		return nil, validationErr("books_target must be 1-100")
	}
	g := &analytics.Goal{UserID: userID, PeriodMonth: periodMonth, BooksTarget: target}
	if err := s.Repo.UpsertGoal(ctx, g); err != nil {
		return nil, err
	}
	got, err := s.Repo.GetGoal(ctx, userID, periodMonth)
	if err != nil {
		return g, nil
	}
	return got, nil
}

func (s *AnalyticsService) GetGoal(ctx context.Context, userID, periodMonth string) (*analytics.Goal, error) {
	if periodMonth == "" {
		periodMonth = s.CurrentMonth()
	}
	return s.Repo.GetGoal(ctx, userID, periodMonth)
}

// ── shelf ───────────────────────────────────────────────────────────────────

func (s *AnalyticsService) ShelfSettings(ctx context.Context, userID string) (*shelf.Settings, error) {
	return s.Repo.GetShelfSettings(ctx, userID)
}

func (s *AnalyticsService) UpdateShelfSettings(ctx context.Context, userID string, st *shelf.Settings) (*shelf.Settings, error) {
	if st.DustDays < 1 {
		return nil, validationErr("dust_days must be >= 1")
	}
	if st.CobwebDays <= st.DustDays {
		return nil, validationErr("cobweb_days must be > dust_days")
	}
	if st.WormDays <= st.CobwebDays {
		return nil, validationErr("worm_days must be > cobweb_days")
	}
	st.UserID = userID
	if err := s.Repo.UpsertShelfSettings(ctx, st); err != nil {
		return nil, err
	}
	return s.Repo.GetShelfSettings(ctx, userID)
}

// ShelfWithStages computes cosmetic stages (pure read; never writes book data).
func (s *AnalyticsService) ShelfWithStages(ctx context.Context, userID string) ([]shelf.Entry, *shelf.Settings, error) {
	settings, err := s.Repo.GetShelfSettings(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	entries, err := s.Repo.ShelfEntries(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	// Disabled cosmetic feature: return healthy stages, no dust/worms.
	if !settings.Enabled {
		for i := range entries {
			entries[i].Stage = shelf.StageFresh
			entries[i].Label = "On the shelf"
		}
		return entries, settings, nil
	}

	now := time.Now().UTC()
	for i := range entries {
		e := &entries[i]
		if e.ReadingStatus == "completed" {
			e.Stage = shelf.StageDone
			e.Label = "Finished ✓"
			e.DaysInactive = 0
			continue
		}
		last, err := parseTime(e.LastActivity)
		if err != nil || last.IsZero() {
			last = now
		}
		days := int(now.Sub(last).Hours() / 24)
		if days < 0 {
			days = 0
		}
		e.DaysInactive = days
		e.Stage, e.Label = stageFor(days, settings)
	}
	return entries, settings, nil
}

func stageFor(days int, s *shelf.Settings) (stage, label string) {
	switch {
	case days < s.DustDays:
		return shelf.StageFresh, "On the shelf"
	case days < s.CobwebDays:
		return shelf.StageDust, "A light dust"
	case days < s.WormDays:
		return shelf.StageCobweb, "Small cobweb"
	default:
		if days >= s.WormDays*2 {
			return shelf.StageHeavy, "Bookworms at work!"
		}
		return shelf.StageWorm, "Bookworm"
	}
}
