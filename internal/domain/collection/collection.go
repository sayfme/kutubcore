package collection

// Physical / digital / reading statuses (Blueprint §5).
// Kept as maps for validation (portable VARCHAR+CHECK, not PG enums).
var PhysicalStatuses = map[string]bool{"not_collected": true, "want_to_buy": true, "owned": true}
var DigitalStatuses = map[string]bool{"not_available": true, "want_to_download": true, "downloaded": true}
var ReadingStatuses = map[string]bool{
	"want_to_read": true, "currently_reading": true, "on_hold": true,
	"completed": true, "abandoned": true,
}

// UserBook is one user's personal copy entry for a catalog book.
type UserBook struct {
	ID             string `json:"id"`
	UserID         string `json:"user_id"`
	BookID         string `json:"book_id"`
	BookTitle      string `json:"book_title,omitempty"`
	PhysicalStatus string `json:"physical_status"`
	DigitalStatus  string `json:"digital_status"`
	ReadingStatus  string `json:"reading_status"`
	IsFavorite     bool   `json:"is_favorite"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`

	Progress *ProgressData `json:"progress,omitempty"`
	Review   *ReviewData   `json:"review,omitempty"`
}

// ProgressData is embedded in UserBook responses (Milestone C).
type ProgressData struct {
	CurrentPage     int     `json:"current_page,omitempty"`
	TotalPages      int     `json:"total_pages,omitempty"`
	ProgressPercent float64 `json:"progress_percent"`
	StartedAt       string  `json:"started_at,omitempty"`
	CompletedAt     string  `json:"completed_at,omitempty"`
}

// ReviewData is the embedded 1:1 review view.
type ReviewData struct {
	Rating     int    `json:"rating,omitempty"`
	Reaction   string `json:"reaction,omitempty"`
	ReviewText string `json:"review_text,omitempty"`
}
