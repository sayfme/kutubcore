package repository

import (
	"context"

	"kutubcore/internal/domain/analytics"
	"kutubcore/internal/domain/shelf"
)

// AnalyticsRepository aggregates reading activity + goals + shelf stages.
type AnalyticsRepository interface {
	Analytics(ctx context.Context, userID string, months int) (*analytics.Summary, error)

	GetGoal(ctx context.Context, userID, periodMonth string) (*analytics.Goal, error)
	UpsertGoal(ctx context.Context, g *analytics.Goal) error
	ListGoals(ctx context.Context, userID string) ([]analytics.Goal, error)

	GetShelfSettings(ctx context.Context, userID string) (*shelf.Settings, error)
	UpsertShelfSettings(ctx context.Context, s *shelf.Settings) error
	// ShelfEntries returns books with last activity (timestamp text).
	ShelfEntries(ctx context.Context, userID string) ([]shelf.Entry, error)
}
