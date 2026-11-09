// Package shelf holds Bookshelf Life cosmetic stages (Milestone G, Blueprint §12).
// Stages never change reading data; they only describe inactivity.
package shelf

// Stage names (cosmetic only).
const (
	StageFresh  = "fresh"
	StageDust   = "dust"
	StageCobweb = "cobweb"
	StageWorm   = "bookworm"
	StageHeavy  = "heavy"
	StageDone   = "completed" // achievement for completed books
)

// Settings are per-user cosmetic thresholds (Blueprint §12 defaults).
type Settings struct {
	UserID     string `json:"user_id"`
	Enabled    bool   `json:"enabled"`
	DustDays   int    `json:"dust_days"`
	CobwebDays int    `json:"cobweb_days"`
	WormDays   int    `json:"worm_days"`
	UpdatedAt  string `json:"updated_at"`
}

// Defaults returns the blueprint thresholds (30/60/90 days).
func Defaults(userID string) Settings {
	return Settings{UserID: userID, Enabled: true, DustDays: 30, CobwebDays: 60, WormDays: 90}
}

// Entry is one shelf book with its computed cosmetic stage.
type Entry struct {
	UserBookID    string `json:"user_book_id"`
	BookID        string `json:"book_id"`
	BookTitle     string `json:"book_title"`
	ReadingStatus string `json:"reading_status"`
	IsFavorite    bool   `json:"is_favorite"`
	LastActivity  string `json:"last_activity,omitempty"`
	DaysInactive  int    `json:"days_inactive"`
	Stage         string `json:"stage"`
	Label         string `json:"label"`
}
