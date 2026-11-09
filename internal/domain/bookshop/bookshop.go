package bookshop

// Shop types (Blueprint §6).
var ShopTypes = map[string]bool{"physical": true, "online": true, "both": true}

// Bookshop is a physical/online bookshop directory entry.
type Bookshop struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShopType  string `json:"shop_type"`
	Address   string `json:"address,omitempty"`
	City      string `json:"city,omitempty"`
	Country   string `json:"country,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Website   string `json:"website,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Links     []Link `json:"links,omitempty"`
}

// Link is a bookshop URL (website/store page).
type Link struct {
	ID         string `json:"id"`
	BookshopID string `json:"bookshop_id,omitempty"`
	Label      string `json:"label"`
	URL        string `json:"url"`
	IsPrimary  bool   `json:"is_primary"`
}
