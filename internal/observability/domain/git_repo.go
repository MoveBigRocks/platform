package observabilitydomain

import (
	"time"
)

// GitRepo connects an Application to its source code repository
type GitRepo struct {
	ID            string
	ApplicationID string
	WorkspaceID   string

	// Connection
	RepoURL       string // https://github.com/org/repo
	DefaultBranch string // main, master, etc.

	// Auth (encrypted at application level before storage)
	AccessToken string

	// Path mapping for monorepos
	// e.g., "services/api/" means stacktrace paths are relative to this prefix
	PathPrefix string

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}
