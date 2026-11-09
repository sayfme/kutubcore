// Package book defines catalog domain types: Book (work) and Edition
// (published version) are distinct (Blueprint §5, §20).
package book

// Allowed edition formats (Blueprint §6).
var Formats = map[string]bool{
	"paperback": true, "hardcover": true, "ebook": true,
	"pdf": true, "epub": true, "audiobook": true, "other": true,
}

// Book is a logical work. Edition-specific fields (ISBN, publisher,
// year) live on Edition, never here.
type Book struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	Subtitle      string      `json:"subtitle,omitempty"`
	Description   string      `json:"description,omitempty"`
	OriginalTitle string      `json:"original_title,omitempty"`
	Version       int         `json:"version"`
	CreatedBy     string      `json:"created_by,omitempty"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	Writers       []WriterRef `json:"writers,omitempty"`
	Genres        []GenreRef  `json:"genres,omitempty"`
	Editions      []Edition   `json:"editions,omitempty"`
}

// WriterRef is the book's writer credit.
type WriterRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Order int    `json:"order"`
}

// GenreRef is a linked genre.
type GenreRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Edition is one published version of a Book.
type Edition struct {
	ID              string       `json:"id"`
	BookID          string       `json:"book_id"`
	LanguageID      string       `json:"language_id,omitempty"`
	LanguageCode    string       `json:"language_code,omitempty"`
	Publisher       string       `json:"publisher,omitempty"`
	EditionNumber   string       `json:"edition_number,omitempty"`
	PublicationYear int          `json:"publication_year,omitempty"`
	Format          string       `json:"format"`
	PageCount       int          `json:"page_count,omitempty"`
	Version         int          `json:"version"`
	CreatedAt       string       `json:"created_at"`
	UpdatedAt       string       `json:"updated_at"`
	Identifiers     []Identifier `json:"identifiers,omitempty"`
}

// Identifier is an ISBN or similar code on an Edition.
type Identifier struct {
	Type  string `json:"identifier_type"`
	Value string `json:"identifier_value"`
}
