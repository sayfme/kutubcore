// Package user defines identity domain types (Blueprint §4-§6).
package user

// Role codes (Blueprint §4).
const (
	RoleReader    = "reader"
	RoleOrganizer = "organizer"
)

// User is a registered account. PasswordHash never leaves the server.
type User struct {
	ID            string   `json:"id"`
	DisplayName   string   `json:"display_name"`
	Email         string   `json:"email,omitempty"`
	Mobile        string   `json:"mobile,omitempty"`
	IsActive      bool     `json:"is_active"`
	EmailVerified bool     `json:"email_verified"`
	Roles         []string `json:"roles,omitempty"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// Role is a named permission record.
type Role struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}
