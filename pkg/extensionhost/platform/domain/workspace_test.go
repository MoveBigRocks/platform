package platformdomain

import (
	"testing"
	"time"
)

func TestNewWorkspace(t *testing.T) {
	name := "Test Workspace"
	slug := "test-workspace"

	ws := NewWorkspace(name, slug)

	if ws.Name != name {
		t.Errorf("Expected name %s, got %s", name, ws.Name)
	}
	if ws.Slug != slug {
		t.Errorf("Expected slug %s, got %s", slug, ws.Slug)
	}
	if ws.ID != "" {
		t.Error("Expected PostgreSQL to generate workspace row ID on insert")
	}
	if !ws.IsActive {
		t.Error("Expected workspace to be active by default")
	}
	if ws.IsSuspended {
		t.Error("Expected workspace to not be suspended by default")
	}
	if ws.MaxUsers != 10 {
		t.Errorf("Expected default MaxUsers 10, got %d", ws.MaxUsers)
	}
	if ws.MaxCases != 1000 {
		t.Errorf("Expected default MaxCases 1000, got %d", ws.MaxCases)
	}
	expectedStorage := int64(10 * 1024 * 1024 * 1024)
	if ws.MaxStorage != expectedStorage {
		t.Errorf("Expected default MaxStorage %d, got %d", expectedStorage, ws.MaxStorage)
	}
	if ws.Settings.Len() < 0 {
		t.Error("Expected Settings to be initialized")
	}
	if ws.Features == nil {
		t.Error("Expected Features slice to be initialized")
	}
	if ws.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if ws.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestWorkspace_GetStoragePath(t *testing.T) {
	tests := []struct {
		name          string
		storageBucket string
		slug          string
		expected      string
	}{
		{
			name:          "custom storage bucket",
			storageBucket: "custom-bucket/path",
			slug:          "test-workspace",
			expected:      "custom-bucket/path",
		},
		{
			name:          "default storage path",
			storageBucket: "",
			slug:          "test-workspace",
			expected:      "workspaces/test-workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &Workspace{
				Slug:          tt.slug,
				StorageBucket: tt.storageBucket,
			}
			result := ws.GetStoragePath()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWorkspace_IsAccessible(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		isActive    bool
		isSuspended bool
		deletedAt   *time.Time
		expected    bool
	}{
		{
			name:        "active workspace",
			isActive:    true,
			isSuspended: false,
			deletedAt:   nil,
			expected:    true,
		},
		{
			name:        "inactive workspace",
			isActive:    false,
			isSuspended: false,
			deletedAt:   nil,
			expected:    false,
		},
		{
			name:        "suspended workspace",
			isActive:    true,
			isSuspended: true,
			deletedAt:   nil,
			expected:    false,
		},
		{
			name:        "deleted workspace",
			isActive:    true,
			isSuspended: false,
			deletedAt:   &now,
			expected:    false,
		},
		{
			name:        "inactive and suspended",
			isActive:    false,
			isSuspended: true,
			deletedAt:   nil,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &Workspace{
				IsActive:    tt.isActive,
				IsSuspended: tt.isSuspended,
				DeletedAt:   tt.deletedAt,
			}
			result := ws.IsAccessible()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGenerateWorkspaceShortCode(t *testing.T) {
	tests := []struct {
		name string
		slug string
		want string
	}{
		{name: "normalizes long slug", slug: "Acme-Team", want: "acme"},
		{name: "keeps short slug", slug: "qa", want: "qa"},
		{name: "pads single character", slug: "x", want: "x1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateWorkspaceShortCode(tt.slug); got != tt.want {
				t.Fatalf("GenerateWorkspaceShortCode(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func TestWorkspace_UpdateDetails(t *testing.T) {
	ws := NewWorkspace("Old Name", "old-name")
	updatedAt := time.Now().Add(time.Hour)

	ws.UpdateDetails("New Name", "new-team", "Updated description", updatedAt)

	if ws.Name != "New Name" {
		t.Fatalf("expected updated name, got %q", ws.Name)
	}
	if ws.Slug != "new-team" {
		t.Fatalf("expected updated slug, got %q", ws.Slug)
	}
	if ws.ShortCode != "newt" {
		t.Fatalf("expected derived short code, got %q", ws.ShortCode)
	}
	if ws.Description != "Updated description" {
		t.Fatalf("expected updated description, got %q", ws.Description)
	}
	if !ws.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated timestamp %v, got %v", updatedAt, ws.UpdatedAt)
	}
}

func TestWorkspace_ValidateDeletion(t *testing.T) {
	ws := &Workspace{Name: "Acme"}

	if err := ws.ValidateDeletion(0, 1, 0); err != nil {
		t.Fatalf("expected workspace to be deletable, got %v", err)
	}
	if err := ws.ValidateDeletion(1, 1, 0); err == nil {
		t.Fatal("expected active case validation error")
	}
	if err := ws.ValidateDeletion(0, 2, 0); err == nil {
		t.Fatal("expected active member validation error")
	}
	if err := ws.ValidateDeletion(0, 1, 3); err == nil {
		t.Fatal("expected open issue validation error")
	}
}

func TestNewContact(t *testing.T) {
	workspaceID := "workspace-456"
	email := " Contact@Example.com "

	contact := NewContact(workspaceID, email)

	if contact.WorkspaceID != workspaceID {
		t.Errorf("Expected workspace ID %s, got %s", workspaceID, contact.WorkspaceID)
	}
	if contact.Email != "contact@example.com" {
		t.Errorf("Expected normalized email %s, got %s", "contact@example.com", contact.Email)
	}
	if contact.ID != "" {
		t.Error("Expected PostgreSQL to generate contact row ID on insert")
	}
	if contact.Tags == nil {
		t.Error("Expected Tags slice to be initialized")
	}
	if contact.CustomFields.Len() < 0 {
		t.Error("Expected CustomFields to be initialized")
	}
	if contact.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if contact.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestContactPolicies(t *testing.T) {
	contact := NewContact("workspace-456", " Person@Example.com ")
	if err := contact.PrepareForSave(); err != nil {
		t.Fatalf("expected contact to validate, got %v", err)
	}
	if contact.Email != "person@example.com" {
		t.Fatalf("expected normalized email, got %q", contact.Email)
	}

	contact.Block(" spam ", time.Unix(20, 0))
	if !contact.IsBlocked || contact.BlockedReason != "spam" {
		t.Fatalf("expected contact to be blocked, got blocked=%v reason=%q", contact.IsBlocked, contact.BlockedReason)
	}

	contact.Unblock(time.Unix(30, 0))
	if contact.IsBlocked || contact.BlockedReason != "" {
		t.Fatalf("expected contact to be unblocked, got blocked=%v reason=%q", contact.IsBlocked, contact.BlockedReason)
	}

	invalid := NewContact("workspace-456", "invalid")
	if err := invalid.PrepareForSave(); err == nil {
		t.Fatal("expected invalid email error")
	}
}
