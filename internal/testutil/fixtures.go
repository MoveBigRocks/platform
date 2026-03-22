package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

var testCounter uint64

// UniqueID generates a unique identifier for test isolation.
func UniqueID(prefix string) string {
	counter := atomic.AddUint64(&testCounter, 1)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	return fmt.Sprintf("%s_%d_%s", prefix, counter, hex.EncodeToString(randomBytes))
}

// UniqueWorkspaceID generates a unique workspace ID for test isolation.
// Includes test name for debugging failed tests.
func UniqueWorkspaceID(t testing.TB) string {
	t.Helper()
	return id.New()
}

// UniqueUserID generates a unique user ID for test isolation.
func UniqueUserID(t testing.TB) string {
	t.Helper()
	return id.New()
}

// UniqueCaseID generates a unique case ID for test isolation.
func UniqueCaseID(t testing.TB) string {
	t.Helper()
	return id.New()
}

// UniqueProjectID generates a unique project ID for test isolation.
func UniqueProjectID(t testing.TB) string {
	t.Helper()
	return id.New()
}

// UniqueEmail generates a unique email for test isolation.
func UniqueEmail(t testing.TB) string {
	t.Helper()
	return fmt.Sprintf("test_%s@example.com", UniqueID("email"))
}

// SetupTestStore creates an isolated PostgreSQL-backed store.
func SetupTestStore(t testing.TB) (stores.Store, func()) {
	t.Helper()
	return SetupTestPostgresStore(t)
}

// SetupTestSQLStore creates an isolated PostgreSQL-backed SQL store.
func SetupTestSQLStore(t testing.TB) (stores.Store, func()) {
	t.Helper()
	return SetupTestPostgresStore(t)
}

// SetTestTenantContext sets the tenant context for test isolation.
func SetTestTenantContext(t testing.TB, store stores.Store, workspaceID string) {
	t.Helper()
	if err := store.SetTenantContext(context.Background(), workspaceID); err != nil {
		t.Fatalf("failed to set tenant context: %v", err)
	}
}

// ClearTestTenantContext clears the tenant context.
func ClearTestTenantContext(t testing.TB, store stores.Store) {
	t.Helper()
	if err := store.SetTenantContext(context.Background(), ""); err != nil {
		t.Fatalf("failed to clear tenant context: %v", err)
	}
}

// NewIsolatedWorkspace creates a test workspace with unique ID for test isolation.
func NewIsolatedWorkspace(t testing.TB) *platformdomain.Workspace {
	t.Helper()
	wsID := UniqueWorkspaceID(t)
	counter := atomic.AddUint64(&testCounter, 1)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	slug := fmt.Sprintf("test-ws-%d-%s", counter, hex.EncodeToString(randomBytes))
	shortCode := hex.EncodeToString(randomBytes)[:4]
	return &platformdomain.Workspace{
		ID:        wsID,
		Name:      "Test Workspace " + wsID,
		Slug:      slug,
		ShortCode: shortCode,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// NewIsolatedUser creates a test user with unique ID for test isolation.
func NewIsolatedUser(t testing.TB, workspaceID string) *platformdomain.User {
	t.Helper()
	_ = workspaceID
	userID := UniqueUserID(t)
	email := UniqueEmail(t)
	return &platformdomain.User{
		ID:            userID,
		Email:         email,
		Name:          "Test User " + userID[:8],
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// NewIsolatedContact creates a test contact with unique ID for test isolation.
func NewIsolatedContact(t testing.TB, workspaceID string) *platformdomain.Contact {
	t.Helper()
	contactID := UniqueID("contact")
	email := UniqueEmail(t)
	return &platformdomain.Contact{
		ID:          contactID,
		Email:       email,
		Name:        "Test Contact " + contactID[:8],
		WorkspaceID: workspaceID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// NewIsolatedCase creates a test case with unique ID for test isolation.
func NewIsolatedCase(t testing.TB, workspaceID string) *servicedomain.Case {
	t.Helper()
	caseID := UniqueCaseID(t)
	email := UniqueEmail(t)
	caseObj := servicedomain.NewCase(workspaceID, "Test Case "+caseID[:8], email)
	caseObj.ID = caseID
	caseObj.GenerateHumanID("test")
	return caseObj
}

// NewIsolatedProject creates a test project with unique ID for test isolation.
func NewIsolatedProject(t testing.TB, workspaceID string) *observabilitydomain.Project {
	t.Helper()
	projectID := UniqueProjectID(t)
	project := observabilitydomain.NewProject(workspaceID, "", "Test Application "+projectID[:8], strings.ToLower(projectID[:12]), "javascript")
	project.ID = projectID
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()
	return project
}

// CreateTestWorkspace creates a workspace in the store and returns its ID.
func CreateTestWorkspace(t testing.TB, store stores.Store, slug string) string {
	t.Helper()
	workspace := NewIsolatedWorkspace(t)
	if slug != "" {
		workspace.Slug = UniqueID(slug)
	}

	ctx := context.Background()
	err := store.Workspaces().CreateWorkspace(ctx, workspace)
	if err != nil {
		t.Fatalf("failed to create test workspace: %v", err)
	}
	return workspace.ID
}

// FutureTime returns a time in the future (useful for testing cutoffs).
func FutureTime() time.Time {
	return time.Now().Add(24 * time.Hour)
}
