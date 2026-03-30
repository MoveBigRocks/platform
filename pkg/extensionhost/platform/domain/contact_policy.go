package platformdomain

import (
	"fmt"
	"strings"
	"time"
)

// NormalizeContactEmail canonicalizes contact email addresses for lookups and persistence.
func NormalizeContactEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsValidContactEmail performs basic domain-level contact email validation.
func IsValidContactEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	return strings.Contains(parts[1], ".")
}

// PrepareForSave normalizes and validates contact state before persistence.
func (c *Contact) PrepareForSave() error {
	c.Email = NormalizeContactEmail(c.Email)
	if strings.TrimSpace(c.WorkspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if c.Email == "" {
		return fmt.Errorf("email is required")
	}
	if !IsValidContactEmail(c.Email) {
		return fmt.Errorf("invalid email format: %s", c.Email)
	}
	return nil
}

// Block marks a contact as blocked.
func (c *Contact) Block(reason string, updatedAt time.Time) {
	c.IsBlocked = true
	c.BlockedReason = strings.TrimSpace(reason)
	c.UpdatedAt = updatedAt
}

// Unblock clears a contact block.
func (c *Contact) Unblock(updatedAt time.Time) {
	c.IsBlocked = false
	c.BlockedReason = ""
	c.UpdatedAt = updatedAt
}
