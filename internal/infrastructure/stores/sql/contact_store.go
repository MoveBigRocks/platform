package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

// ContactStore implements shared.ContactStore using sqlx
type ContactStore struct {
	db *SqlxDB
}

// NewContactStore creates a new contact store
func NewContactStore(db *SqlxDB) *ContactStore {
	return &ContactStore{db: db}
}

func (s *ContactStore) CreateContact(ctx context.Context, contact *platformdomain.Contact) error {
	tags, err := marshalJSONString(contact.Tags, "tags")
	if err != nil {
		return fmt.Errorf("create contact: %w", err)
	}
	customFields, err := marshalJSONString(contact.CustomFields, "custom_fields")
	if err != nil {
		return fmt.Errorf("create contact: %w", err)
	}

	normalizePersistedUUID(&contact.ID)
	query := `INSERT INTO core_platform.contacts (
		id, workspace_id, email, name, phone, company, tags, notes, custom_fields,
		preferred_language, timezone, is_blocked, blocked_reason, total_cases,
		last_contact_at, created_at, updated_at
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		contact.ID, contact.WorkspaceID, contact.Email, contact.Name, contact.Phone,
		contact.Company, tags, contact.Notes, customFields, contact.PreferredLanguage,
		contact.Timezone, contact.IsBlocked, contact.BlockedReason, contact.TotalCases,
		contact.LastContactAt, contact.CreatedAt, contact.UpdatedAt,
	).Scan(&contact.ID)
	return TranslateSqlxError(err, "contacts")
}

func (s *ContactStore) GetContact(ctx context.Context, workspaceID, contactID string) (*platformdomain.Contact, error) {
	var m models.Contact
	query := `SELECT * FROM core_platform.contacts WHERE id = ? AND workspace_id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &m, query, contactID, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "contacts")
	}
	return s.mapToDomain(&m)
}

func (s *ContactStore) GetContactByEmail(ctx context.Context, workspaceID, email string) (*platformdomain.Contact, error) {
	var m models.Contact
	query := `SELECT * FROM core_platform.contacts WHERE workspace_id = ? AND email = ?`
	err := s.db.Get(ctx).GetContext(ctx, &m, query, workspaceID, email)
	if err != nil {
		return nil, TranslateSqlxError(err, "contacts")
	}
	return s.mapToDomain(&m)
}

func (s *ContactStore) UpdateContact(ctx context.Context, contact *platformdomain.Contact) error {
	tags, err := marshalJSONString(contact.Tags, "tags")
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}
	customFields, err := marshalJSONString(contact.CustomFields, "custom_fields")
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	query := `UPDATE core_platform.contacts SET
		email = ?, name = ?, phone = ?, company = ?, tags = ?, notes = ?,
		custom_fields = ?, preferred_language = ?, timezone = ?, is_blocked = ?,
		blocked_reason = ?, total_cases = ?, last_contact_at = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		contact.Email, contact.Name, contact.Phone,
		contact.Company, tags, contact.Notes, customFields, contact.PreferredLanguage,
		contact.Timezone, contact.IsBlocked, contact.BlockedReason, contact.TotalCases,
		contact.LastContactAt, time.Now(), contact.ID, contact.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "contacts")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ContactStore) ListWorkspaceContacts(ctx context.Context, workspaceID string) ([]*platformdomain.Contact, error) {
	var contacts []models.Contact
	query := `SELECT * FROM core_platform.contacts WHERE workspace_id = ? ORDER BY name ASC`
	err := s.db.Get(ctx).SelectContext(ctx, &contacts, query, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "contacts")
	}

	result := make([]*platformdomain.Contact, len(contacts))
	for i, c := range contacts {
		domainC, err := s.mapToDomain(&c)
		if err != nil {
			return nil, fmt.Errorf("map contact %s: %w", c.ID, err)
		}
		result[i] = domainC
	}
	return result, nil
}

func (s *ContactStore) DeleteContact(ctx context.Context, workspaceID, contactID string) error {
	query := `DELETE FROM core_platform.contacts WHERE id = ? AND workspace_id = ?`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, contactID, workspaceID)
	if err != nil {
		return TranslateSqlxError(err, "contacts")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ContactStore) mapToDomain(m *models.Contact) (*platformdomain.Contact, error) {
	var tags []string

	if m.Tags != "" {
		if err := json.Unmarshal([]byte(m.Tags), &tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}
	customFields := unmarshalTypedCustomFieldsOrEmpty(m.CustomFields, "contacts", "custom_fields")

	return &platformdomain.Contact{
		ID:                m.ID,
		WorkspaceID:       m.WorkspaceID,
		Email:             m.Email,
		Name:              m.Name,
		Phone:             m.Phone,
		Company:           m.Company,
		Tags:              tags,
		Notes:             m.Notes,
		CustomFields:      customFields,
		PreferredLanguage: m.PreferredLanguage,
		Timezone:          m.Timezone,
		IsBlocked:         m.IsBlocked,
		BlockedReason:     m.BlockedReason,
		TotalCases:        m.TotalCases,
		LastContactAt:     m.LastContactAt,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}, nil
}
