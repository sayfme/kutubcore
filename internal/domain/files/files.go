package files

// File is a PDF/EPUB digital file (or a cover image via file_format=cover).
type File struct {
	ID             string     `json:"id"`
	BookID         string     `json:"book_id"`
	EditionID      string     `json:"edition_id,omitempty"`
	FileName       string     `json:"file_name"`
	FileFormat     string     `json:"file_format"`
	FileSize       int64      `json:"file_size,omitempty"`
	ChecksumSHA256 string     `json:"checksum_sha256,omitempty"`
	MimeType       string     `json:"mime_type,omitempty"`
	CreatedAt      string     `json:"created_at"`
	Locations      []Location `json:"locations,omitempty"`
}

// Location is one stored copy of a File (local disk or external URL).
type Location struct {
	ID           string `json:"id"`
	BookFileID   string `json:"book_file_id,omitempty"`
	LocationType string `json:"location_type"`
	StoragePath  string `json:"storage_path,omitempty"`
	ExternalURL  string `json:"external_url,omitempty"`
	IsAvailable  bool   `json:"is_available"`
}
