// Package analytics aggregates reading activity (Milestone G, Blueprint §3-13).
package analytics

// MonthStat is one month of reading activity (UTC month, "YYYY-MM").
type MonthStat struct {
	Month      string `json:"month"`
	PagesRead  int    `json:"pages_read"`
	Sessions   int    `json:"sessions"`
	BooksDone  int    `json:"books_done"`
	AvgSession int    `json:"avg_session_pages"`
}

// Summary rolls the whole window up.
type Summary struct {
	Months     []MonthStat `json:"months"`
	TotalPages int         `json:"total_pages"`
	TotalSess  int         `json:"total_sessions"`
	TotalDone  int         `json:"total_books_done"`
}

// Goal is one month's books target (Blueprint §3-13, Milestone 6).
type Goal struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	PeriodMonth string `json:"period_month"`
	BooksTarget int    `json:"books_target"`
	BooksDone   int    `json:"books_done,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
