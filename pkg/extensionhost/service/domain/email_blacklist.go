package servicedomain

import (
	"fmt"
	"regexp"
	"time"
)

// EmailBlacklist represents blocked email addresses or domains
type EmailBlacklist struct {
	ID          string
	WorkspaceID string

	// Block target
	Email   string // Specific email address
	Domain  string // Entire domain
	Pattern string // Regex pattern

	// Block details
	Type     string // "email", "domain", "pattern"
	Reason   string // Why it was blocked
	IsActive bool

	// Block scope
	BlockInbound  bool // Block incoming emails
	BlockOutbound bool // Block outgoing emails

	// Temporary blocks
	ExpiresAt *time.Time

	// Statistics
	BlockCount    int
	LastBlockedAt *time.Time

	// Metadata
	CreatedByID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// NewEmailBlacklist creates a new email blacklist entry
func NewEmailBlacklist(workspaceID, blockType, value, reason string, createdByID string) *EmailBlacklist {
	entry := &EmailBlacklist{
		WorkspaceID:   workspaceID,
		Type:          blockType,
		Reason:        reason,
		IsActive:      true,
		BlockInbound:  true,
		BlockOutbound: false,
		CreatedByID:   createdByID,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	switch blockType {
	case "email":
		entry.Email = value
	case "domain":
		entry.Domain = value
	case "pattern":
		entry.Pattern = value
	}

	return entry
}

// ValidatePattern validates the regex pattern if this is a pattern-type blacklist entry
func (eb *EmailBlacklist) ValidatePattern() error {
	if eb.Type != "pattern" {
		return nil
	}
	if eb.Pattern == "" {
		return fmt.Errorf("pattern is required for pattern-type blacklist entries")
	}
	if _, err := regexp.Compile(eb.Pattern); err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	return nil
}

// IsBlocked checks if an email address is blocked
func (eb *EmailBlacklist) IsBlocked(email, domain string) bool {
	if !eb.IsActive {
		return false
	}

	// Check expiration
	if eb.ExpiresAt != nil && time.Now().After(*eb.ExpiresAt) {
		return false
	}

	switch eb.Type {
	case "email":
		return eb.Email == email
	case "domain":
		return eb.Domain == domain
	case "pattern":
		if eb.Pattern == "" {
			return false
		}
		re, err := regexp.Compile(eb.Pattern)
		if err != nil {
			// Invalid pattern, treat as not blocked
			return false
		}
		// Check pattern against both email and domain
		return re.MatchString(email) || re.MatchString(domain)
	}

	return false
}
