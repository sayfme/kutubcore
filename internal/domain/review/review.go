package review

// Reaction labels (Blueprint §5) — independent from star rating.
var Reactions = map[string]bool{"favorite": true, "average": true, "meh": true, "legend": true}

// Note is a private journal note (Blueprint §5/§7: private by default).
type Note struct {
	ID         string `json:"id"`
	UserBookID string `json:"user_book_id"`
	Title      string `json:"title,omitempty"`
	Content    string `json:"content"`
	PageNumber int    `json:"page_number,omitempty"`
	Visibility string `json:"visibility"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Review is the 1:1 star rating + reaction + text for a UserBook.
type Review struct {
	ID         string `json:"id"`
	UserBookID string `json:"user_book_id"`
	Rating     int    `json:"rating,omitempty"`
	Reaction   string `json:"reaction,omitempty"`
	ReviewText string `json:"review_text,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
