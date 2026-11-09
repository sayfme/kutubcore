package reading

// Progress is the current reading position for a UserBook (1:1).
type Progress struct {
	ID              string  `json:"id"`
	UserBookID      string  `json:"user_book_id"`
	CurrentPage     int     `json:"current_page,omitempty"`
	TotalPages      int     `json:"total_pages,omitempty"`
	ProgressPercent float64 `json:"progress_percent"`
	StartedAt       string  `json:"started_at,omitempty"`
	CompletedAt     string  `json:"completed_at,omitempty"`
	UpdatedAt       string  `json:"updated_at"`
}

// Session is one reading session (started/ended, pages read).
type Session struct {
	ID         string `json:"id"`
	UserBookID string `json:"user_book_id"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at,omitempty"`
	PagesRead  int    `json:"pages_read,omitempty"`
	Note       string `json:"reading_note,omitempty"`
}
