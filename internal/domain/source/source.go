package source

// Collection source types (Blueprint §3 module 7).
var SourceTypes = map[string]bool{
	"bookshop": true, "library": true, "gift": true, "friend": true,
	"publisher": true, "online_repository": true, "other": true,
}

// Source is a named collection source (where books come from).
type Source struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	SourceType  string `json:"source_type"`
	Description string `json:"description,omitempty"`
	Version     int    `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Links       []Link `json:"links,omitempty"`
}

// Link is a source URL.
type Link struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id,omitempty"`
	Label    string `json:"label"`
	URL      string `json:"url"`
}
