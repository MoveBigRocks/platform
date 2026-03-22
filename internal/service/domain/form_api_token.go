package servicedomain

import "time"

// FormAPIToken represents an API token for programmatic form submissions
type FormAPIToken struct {
	ID           string
	WorkspaceID  string
	FormID       string
	Token        string
	Name         string
	IsActive     bool
	ExpiresAt    *time.Time
	AllowedHosts []string
	LastUsedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
