package servicedomain

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// Queue is a workspace-scoped container for grouping cases.
// ATS can use queues as jobs, while other extensions can use them as queues or topics.
type Queue struct {
	ID          string
	WorkspaceID string
	Slug        string
	Name        string
	Description string
	Metadata    shareddomain.TypedCustomFields
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// NewQueue creates a new queue with a normalized slug.
func NewQueue(workspaceID, name, slug, description string) *Queue {
	now := time.Now()
	return &Queue{
		WorkspaceID: workspaceID,
		Slug:        NormalizeQueueSlug(slug, name),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Metadata:    shareddomain.NewTypedCustomFields(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate ensures the queue has the minimum required fields.
func (c *Queue) Validate() error {
	if c.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if c.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if c.Slug != NormalizeQueueSlug(c.Slug, "") {
		return fmt.Errorf("slug must contain only lowercase letters, numbers, and hyphens")
	}
	return nil
}

// Rename updates the queue name and description.
func (c *Queue) Rename(name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	c.Name = name
	c.Description = strings.TrimSpace(description)
	c.UpdatedAt = time.Now()
	return nil
}

// SetSlug normalizes and assigns a slug.
func (c *Queue) SetSlug(slug string) error {
	normalized := NormalizeQueueSlug(slug, c.Name)
	if normalized == "" {
		return fmt.Errorf("slug is required")
	}
	c.Slug = normalized
	c.UpdatedAt = time.Now()
	return nil
}

// NormalizeQueueSlug converts a name or slug to a stable lowercase slug.
func NormalizeQueueSlug(slug, fallbackName string) string {
	source := strings.TrimSpace(slug)
	if source == "" {
		source = strings.TrimSpace(fallbackName)
	}
	source = strings.ToLower(source)

	var b strings.Builder
	lastDash := false
	for _, r := range source {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "queue"
	}
	return result
}
