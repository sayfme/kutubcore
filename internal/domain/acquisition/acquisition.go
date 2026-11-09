package acquisition

// Copy conditions (Blueprint §6).
var Conditions = map[string]bool{"new": true, "good": true, "fair": true, "damaged": true}

// Acquisition is one purchase/receipt event by a user.
type Acquisition struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	SourceID        string  `json:"source_id,omitempty"`
	BookshopID      string  `json:"bookshop_id,omitempty"`
	AcquisitionDate string  `json:"acquisition_date,omitempty"`
	TotalCost       float64 `json:"total_cost,omitempty"`
	CurrencyCode    string  `json:"currency_code,omitempty"`
	Notes           string  `json:"notes,omitempty"`
	CreatedAt       string  `json:"created_at"`
	Items           []Item  `json:"items,omitempty"`
}

// Item is one line of an acquisition (book + quantity).
type Item struct {
	ID            string  `json:"id"`
	AcquisitionID string  `json:"acquisition_id,omitempty"`
	BookID        string  `json:"book_id"`
	EditionID     string  `json:"edition_id,omitempty"`
	Quantity      int     `json:"quantity"`
	UnitPrice     float64 `json:"unit_price,omitempty"`
	Format        string  `json:"format"`
	Copies        []Copy  `json:"copies,omitempty"`
}

// Copy is one physical/digital unit created from an item.
type Copy struct {
	ID                string `json:"id"`
	AcquisitionItemID string `json:"acquisition_item_id,omitempty"`
	EditionID         string `json:"edition_id,omitempty"`
	Format            string `json:"format"`
	Condition         string `json:"condition,omitempty"`
	StorageLocation   string `json:"storage_location,omitempty"`
	AcquiredAt        string `json:"acquired_at,omitempty"`
	Notes             string `json:"notes,omitempty"`
	// Denormalized for UI: the book this copy belongs to (via item).
	BookID    string `json:"book_id,omitempty"`
	BookTitle string `json:"book_title,omitempty"`
}
