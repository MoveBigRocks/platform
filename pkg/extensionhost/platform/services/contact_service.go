package platformservices

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

// ContactService manages contact CRUD operations
type ContactService struct {
	contactStore shared.ContactStore
}

// NewContactService creates a new contact service
func NewContactService(contactStore shared.ContactStore) *ContactService {
	return &ContactService{
		contactStore: contactStore,
	}
}

// CreateContactParams contains parameters for creating a contact
type CreateContactParams struct {
	WorkspaceID string
	Email       string
	Name        string
	Phone       string
	Company     string
	Source      string
	Metadata    map[string]interface{}
}

// CreateContact creates a new contact with validation
func (cs *ContactService) CreateContact(ctx context.Context, params CreateContactParams) (*platformdomain.Contact, error) {
	contact := platformdomain.NewContact(params.WorkspaceID, params.Email)
	if err := contact.PrepareForSave(); err != nil {
		return nil, err
	}

	// Check for duplicate
	existing, err := cs.contactStore.GetContactByEmail(ctx, params.WorkspaceID, contact.Email)
	if err == nil && existing != nil {
		// Contact already exists, return it
		return existing, nil
	}

	contact.Name = params.Name
	contact.Phone = params.Phone
	contact.Company = params.Company

	// Add source to custom fields if provided
	if params.Source != "" {
		contact.CustomFields.SetString("source", params.Source)
	}

	// Merge additional metadata
	for key, value := range params.Metadata {
		contact.CustomFields.SetAny(key, value)
	}

	// Store the contact
	if err := cs.contactStore.CreateContact(ctx, contact); err != nil {
		return nil, fmt.Errorf("failed to store contact: %w", err)
	}

	return contact, nil
}

// GetContact retrieves a contact by ID within a workspace
func (cs *ContactService) GetContact(ctx context.Context, workspaceID, contactID string) (*platformdomain.Contact, error) {
	return cs.contactStore.GetContact(ctx, workspaceID, contactID)
}

// GetContactByEmail retrieves a contact by email
func (cs *ContactService) GetContactByEmail(ctx context.Context, workspaceID, email string) (*platformdomain.Contact, error) {
	normalizedEmail := platformdomain.NormalizeContactEmail(email)
	return cs.contactStore.GetContactByEmail(ctx, workspaceID, normalizedEmail)
}

// UpdateContact updates an existing contact
func (cs *ContactService) UpdateContact(ctx context.Context, contact *platformdomain.Contact) error {
	if contact.ID == "" {
		return fmt.Errorf("contact ID is required")
	}

	if err := contact.PrepareForSave(); err != nil {
		return err
	}

	return cs.contactStore.UpdateContact(ctx, contact)
}

// ListWorkspaceContacts lists all contacts for a workspace
func (cs *ContactService) ListWorkspaceContacts(ctx context.Context, workspaceID string) ([]*platformdomain.Contact, error) {
	return cs.contactStore.ListWorkspaceContacts(ctx, workspaceID)
}

// DeleteContact deletes a contact
func (cs *ContactService) DeleteContact(ctx context.Context, workspaceID, contactID string) error {
	return cs.contactStore.DeleteContact(ctx, workspaceID, contactID)
}

// BlockContact blocks a contact with a reason
func (cs *ContactService) BlockContact(ctx context.Context, workspaceID, contactID, reason string) error {
	contact, err := cs.contactStore.GetContact(ctx, workspaceID, contactID)
	if err != nil {
		return fmt.Errorf("contact not found: %w", err)
	}

	contact.Block(reason, time.Now())

	return cs.contactStore.UpdateContact(ctx, contact)
}

// UnblockContact unblocks a contact
func (cs *ContactService) UnblockContact(ctx context.Context, workspaceID, contactID string) error {
	contact, err := cs.contactStore.GetContact(ctx, workspaceID, contactID)
	if err != nil {
		return fmt.Errorf("contact not found: %w", err)
	}

	contact.Unblock(time.Now())

	return cs.contactStore.UpdateContact(ctx, contact)
}
