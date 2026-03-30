package sql

import (
	"context"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

// =============================================================================
// Email Blacklist Operations
// =============================================================================

func (s *EmailStore) CreateEmailBlacklist(ctx context.Context, blacklist *servicedomain.EmailBlacklist) error {
	normalizePersistedUUID(&blacklist.ID)
	query := `INSERT INTO core_service.email_blacklists (
		id, workspace_id, email, domain, pattern, type, reason, is_active,
		block_inbound, block_outbound, expires_at, block_count, last_blocked_at,
		created_by_id, created_at, updated_at
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		blacklist.ID, blacklist.WorkspaceID, blacklist.Email, blacklist.Domain,
		blacklist.Pattern, blacklist.Type, blacklist.Reason, blacklist.IsActive,
		blacklist.BlockInbound, blacklist.BlockOutbound, blacklist.ExpiresAt,
		blacklist.BlockCount, blacklist.LastBlockedAt, blacklist.CreatedByID,
		blacklist.CreatedAt, blacklist.UpdatedAt,
	).Scan(&blacklist.ID)
	return TranslateSqlxError(err, "email_blacklists")
}

func (s *EmailStore) ListWorkspaceEmailBlacklists(ctx context.Context, workspaceID string) ([]*servicedomain.EmailBlacklist, error) {
	var dbBlacklists []models.EmailBlacklist
	query := `SELECT * FROM core_service.email_blacklists WHERE workspace_id = ? AND deleted_at IS NULL ORDER BY created_at DESC`
	err := s.db.Get(ctx).SelectContext(ctx, &dbBlacklists, query, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "email_blacklists")
	}

	result := make([]*servicedomain.EmailBlacklist, len(dbBlacklists))
	for i, bl := range dbBlacklists {
		result[i] = s.mapBlacklistToDomain(&bl)
	}
	return result, nil
}

func (s *EmailStore) CheckEmailBlacklist(ctx context.Context, workspaceID, email, domain string) (*servicedomain.EmailBlacklist, error) {
	var dbBlacklist models.EmailBlacklist
	query := `SELECT * FROM core_service.email_blacklists
		WHERE workspace_id = ? AND is_active = TRUE AND deleted_at IS NULL
		AND (email = ? OR domain = ?)
		LIMIT 1`

	err := s.db.Get(ctx).GetContext(ctx, &dbBlacklist, query, workspaceID, email, domain)
	if err != nil {
		if TranslateSqlxError(err, "email_blacklists") == shared.ErrNotFound {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "email_blacklists")
	}
	return s.mapBlacklistToDomain(&dbBlacklist), nil
}

func (s *EmailStore) mapBlacklistToDomain(bl *models.EmailBlacklist) *servicedomain.EmailBlacklist {
	return &servicedomain.EmailBlacklist{
		ID:            bl.ID,
		WorkspaceID:   bl.WorkspaceID,
		Email:         bl.Email,
		Domain:        bl.Domain,
		Pattern:       bl.Pattern,
		Type:          bl.Type,
		Reason:        bl.Reason,
		IsActive:      bl.IsActive,
		BlockInbound:  bl.BlockInbound,
		BlockOutbound: bl.BlockOutbound,
		ExpiresAt:     bl.ExpiresAt,
		BlockCount:    bl.BlockCount,
		LastBlockedAt: bl.LastBlockedAt,
		CreatedByID:   bl.CreatedByID,
		CreatedAt:     bl.CreatedAt,
		UpdatedAt:     bl.UpdatedAt,
	}
}
