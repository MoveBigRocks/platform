//go:build integration

package platformservices

import (
	"context"
	"errors"
	"testing"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/id"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContactService(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())

	require.NotNil(t, cs)
}

func TestContactService_CreateContact(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())

	tests := []struct {
		name      string
		params    CreateContactParams
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid contact",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "test@example.com",
				Name:        "Test User",
				Phone:       "+1234567890",
				Company:     "Test Company",
			},
			wantErr: false,
		},
		{
			name: "missing workspace_id",
			params: CreateContactParams{
				Email: "test@example.com",
			},
			wantErr:   true,
			errSubstr: "workspace_id is required",
		},
		{
			name: "missing email",
			params: CreateContactParams{
				WorkspaceID: id.New(),
			},
			wantErr:   true,
			errSubstr: "email is required",
		},
		{
			name: "invalid email format - no @",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "invalid-email",
			},
			wantErr:   true,
			errSubstr: "invalid email format",
		},
		{
			name: "invalid email format - no domain",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "test@",
			},
			wantErr:   true,
			errSubstr: "invalid email format",
		},
		{
			name: "invalid email format - no username",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "@example.com",
			},
			wantErr:   true,
			errSubstr: "invalid email format",
		},
		{
			name: "invalid email format - no dot in domain",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "test@example",
			},
			wantErr:   true,
			errSubstr: "invalid email format",
		},
		{
			name: "email with spaces gets trimmed",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "  spaces@example.com  ",
				Name:        "Spaces User",
			},
			wantErr: false,
		},
		{
			name: "email case normalized",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "UPPER@EXAMPLE.COM",
				Name:        "Upper User",
			},
			wantErr: false,
		},
		{
			name: "contact with source",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "source@example.com",
				Name:        "Source User",
				Source:      "website",
			},
			wantErr: false,
		},
		{
			name: "contact with metadata",
			params: CreateContactParams{
				WorkspaceID: id.New(),
				Email:       "meta@example.com",
				Name:        "Meta User",
				Metadata: map[string]interface{}{
					"key1": "value1",
					"key2": 123,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.params.WorkspaceID != "" {
				createWorkspaceFixture(t, ctx, store, tt.params.WorkspaceID)
			}

			contact, err := cs.CreateContact(ctx, tt.params)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, contact)
			assert.NotEmpty(t, contact.ID)
			assert.Equal(t, tt.params.WorkspaceID, contact.WorkspaceID)
			assert.Equal(t, tt.params.Name, contact.Name)
			assert.Equal(t, tt.params.Phone, contact.Phone)
			assert.Equal(t, tt.params.Company, contact.Company)
		})
	}
}

func TestContactService_CreateContact_DuplicateReturnsExisting(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create first contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "duplicate@example.com",
		Name:        "Original Name",
	}
	original, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, original)

	// Try to create duplicate - should return existing
	params.Name = "New Name"
	duplicate, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, duplicate)

	// Should be the same contact (same ID)
	assert.Equal(t, original.ID, duplicate.ID)
	// Name should be the original name since we return existing
	assert.Equal(t, "Original Name", duplicate.Name)
}

func TestContactService_GetContact(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create a contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "get@example.com",
		Name:        "Get User",
	}
	created, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)

	// Get the contact
	contact, err := cs.GetContact(ctx, params.WorkspaceID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, contact)
	assert.Equal(t, created.ID, contact.ID)
	assert.Equal(t, params.Email, contact.Email)

	// Get non-existent contact
	_, err = cs.GetContact(ctx, params.WorkspaceID, id.New())
	require.Error(t, err)
}

func TestContactService_GetContactByEmail(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create a contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "byemail@example.com",
		Name:        "Email User",
	}
	created, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)

	// Get by email
	contact, err := cs.GetContactByEmail(ctx, params.WorkspaceID, params.Email)
	require.NoError(t, err)
	require.NotNil(t, contact)
	assert.Equal(t, created.ID, contact.ID)

	// Get with uppercase email (should be normalized)
	contact, err = cs.GetContactByEmail(ctx, params.WorkspaceID, "BYEMAIL@EXAMPLE.COM")
	require.NoError(t, err)
	assert.Equal(t, created.ID, contact.ID)

	// Get non-existent email
	_, err = cs.GetContactByEmail(ctx, params.WorkspaceID, "nonexistent@example.com")
	require.Error(t, err)
}

func TestContactService_UpdateContact(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create a contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "update@example.com",
		Name:        "Original Name",
	}
	created, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)

	// Update the contact
	created.Name = "Updated Name"
	created.Phone = "+9876543210"
	err = cs.UpdateContact(ctx, created)
	require.NoError(t, err)

	// Verify update
	contact, err := cs.GetContact(ctx, params.WorkspaceID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", contact.Name)
	assert.Equal(t, "+9876543210", contact.Phone)
}

func TestContactService_UpdateContact_Validation(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create a contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "updateval@example.com",
		Name:        "Original Name",
	}
	created, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)

	tests := []struct {
		name      string
		contact   *platformdomain.Contact
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "missing ID",
			contact:   &platformdomain.Contact{},
			wantErr:   true,
			errSubstr: "contact ID is required",
		},
		{
			name: "invalid email format",
			contact: &platformdomain.Contact{
				ID:          created.ID,
				WorkspaceID: params.WorkspaceID,
				Email:       "invalid",
			},
			wantErr:   true,
			errSubstr: "invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cs.UpdateContact(ctx, tt.contact)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestContactService_ListWorkspaceContacts(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())

	// Create multiple contacts
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)
	for i := 0; i < 3; i++ {
		params := CreateContactParams{
			WorkspaceID: workspaceID,
			Email:       "contact" + string(rune('0'+i)) + "@example.com",
			Name:        "Contact " + string(rune('0'+i)),
		}
		_, err := cs.CreateContact(ctx, params)
		require.NoError(t, err)
	}

	// List contacts
	contacts, err := cs.ListWorkspaceContacts(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, contacts, 3)

	// List empty workspace
	contacts, err = cs.ListWorkspaceContacts(ctx, id.New())
	require.NoError(t, err)
	assert.Len(t, contacts, 0)
}

func TestContactService_DeleteContact(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create a contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "delete@example.com",
		Name:        "Delete User",
	}
	created, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)

	// Delete the contact
	err = cs.DeleteContact(ctx, params.WorkspaceID, created.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = cs.GetContact(ctx, params.WorkspaceID, created.ID)
	require.Error(t, err)
}

func TestContactService_BlockContact(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create a contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "block@example.com",
		Name:        "Block User",
	}
	created, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)

	// Block the contact
	reason := "Spam activity"
	err = cs.BlockContact(ctx, params.WorkspaceID, created.ID, reason)
	require.NoError(t, err)

	// Verify blocked
	contact, err := cs.GetContact(ctx, params.WorkspaceID, created.ID)
	require.NoError(t, err)
	assert.True(t, contact.IsBlocked)
	assert.Equal(t, reason, contact.BlockedReason)
}

func TestContactService_BlockContact_NotFound(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())

	// Try to block non-existent contact
	err := cs.BlockContact(ctx, id.New(), id.New(), "reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contact not found")
}

func TestContactService_UnblockContact(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())
	workspaceID := id.New()
	createWorkspaceFixture(t, ctx, store, workspaceID)

	// Create and block a contact
	params := CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       "unblock@example.com",
		Name:        "Unblock User",
	}
	created, err := cs.CreateContact(ctx, params)
	require.NoError(t, err)

	err = cs.BlockContact(ctx, params.WorkspaceID, created.ID, "Test block")
	require.NoError(t, err)

	// Unblock the contact
	err = cs.UnblockContact(ctx, params.WorkspaceID, created.ID)
	require.NoError(t, err)

	// Verify unblocked
	contact, err := cs.GetContact(ctx, params.WorkspaceID, created.ID)
	require.NoError(t, err)
	assert.False(t, contact.IsBlocked)
	assert.Empty(t, contact.BlockedReason)
}

func TestContactService_UnblockContact_NotFound(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupTestStore(t)
	defer cleanup()
	cs := NewContactService(store.Contacts())

	// Try to unblock non-existent contact
	err := cs.UnblockContact(ctx, id.New(), id.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contact not found")
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email with subdomain", "test@sub.example.com", true},
		{"valid email with plus", "test+tag@example.com", true},
		{"no @", "testexample.com", false},
		{"no domain", "test@", false},
		{"no username", "@example.com", false},
		{"no dot in domain", "test@example", false},
		{"multiple @", "test@@example.com", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := platformdomain.IsValidContactEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func createWorkspaceFixture(t *testing.T, ctx context.Context, store shared.Store, workspaceID string) {
	t.Helper()
	if _, err := store.Workspaces().GetWorkspace(ctx, workspaceID); err == nil {
		return
	} else if !errors.Is(err, shared.ErrNotFound) {
		require.NoError(t, err)
	}

	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.ID = workspaceID
	workspace.Name = "Workspace " + workspaceID
	workspace.Slug = workspaceID

	err := store.Workspaces().CreateWorkspace(ctx, workspace)
	require.NoError(t, err)
}
