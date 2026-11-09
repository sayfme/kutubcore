// Package writer defines writer directory types (Blueprint §6).
package writer

// Writer is a catalog writer/author.
type Writer struct {
	ID        string `json:"id"`
	FullName  string `json:"full_name"`
	Biography string `json:"biography,omitempty"`
	BirthDate string `json:"birth_date,omitempty"`
	DeathDate string `json:"death_date,omitempty"`
	Website   string `json:"website,omitempty"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
