package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

// WorkspaceStore implements shared.WorkspaceStore
type WorkspaceStore struct {
	db *SqlxDB
}

func NewWorkspaceStore(db *SqlxDB) *WorkspaceStore {
	return &WorkspaceStore{db: db}
}

// =============================================================================
// Workspace CRUD
// =============================================================================

func (s *WorkspaceStore) CreateWorkspace(ctx context.Context, w *platformdomain.Workspace) error {
	settings, err := marshalJSONString(w.Settings, "settings")
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	features, err := marshalJSONString(w.Features, "features")
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	normalizePersistedUUID(&w.ID)
	query := `
		INSERT INTO core_platform.workspaces (
			id, name, slug, short_code, description, logo_url, primary_color, accent_color,
			settings, features, storage_bucket, max_users, max_cases, max_storage,
			is_active, is_suspended, suspend_reason, created_at, updated_at, deleted_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		w.ID, w.Name, w.Slug, w.ShortCode, w.Description, w.LogoURL, w.PrimaryColor, w.AccentColor,
		settings, features, w.StorageBucket, w.MaxUsers, w.MaxCases, w.MaxStorage,
		w.IsActive, w.IsSuspended, w.SuspendReason, w.CreatedAt, w.UpdatedAt, nil,
	).Scan(&w.ID)
	return TranslateSqlxError(err, "workspaces")
}

func (s *WorkspaceStore) GetWorkspace(ctx context.Context, workspaceID string) (*platformdomain.Workspace, error) {
	var dbW models.Workspace
	query := `SELECT * FROM core_platform.workspaces WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbW, query, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "workspaces")
	}
	return mapWorkspaceToDomain(&dbW), nil
}

func (s *WorkspaceStore) GetWorkspaceBySlug(ctx context.Context, slug string) (*platformdomain.Workspace, error) {
	var dbW models.Workspace
	query := `SELECT * FROM core_platform.workspaces WHERE slug = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbW, query, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "workspaces")
	}
	return mapWorkspaceToDomain(&dbW), nil
}

func (s *WorkspaceStore) UpdateWorkspace(ctx context.Context, w *platformdomain.Workspace) error {
	settings, err := marshalJSONString(w.Settings, "settings")
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}
	features, err := marshalJSONString(w.Features, "features")
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}

	query := `
		UPDATE core_platform.workspaces SET
			name = ?, slug = ?, short_code = ?, description = ?, logo_url = ?,
			primary_color = ?, accent_color = ?, settings = ?, features = ?,
			storage_bucket = ?, max_users = ?, max_cases = ?, max_storage = ?,
			is_active = ?, is_suspended = ?, suspend_reason = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		w.Name, w.Slug, w.ShortCode, w.Description, w.LogoURL,
		w.PrimaryColor, w.AccentColor, settings, features,
		w.StorageBucket, w.MaxUsers, w.MaxCases, w.MaxStorage,
		w.IsActive, w.IsSuspended, w.SuspendReason, w.UpdatedAt,
		w.ID,
	)
	if err != nil {
		return TranslateSqlxError(err, "workspaces")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *WorkspaceStore) ListWorkspaces(ctx context.Context) ([]*platformdomain.Workspace, error) {
	var dbWorkspaces []models.Workspace
	query := `SELECT * FROM core_platform.workspaces WHERE deleted_at IS NULL`
	if err := s.db.Get(ctx).SelectContext(ctx, &dbWorkspaces, query); err != nil {
		return nil, TranslateSqlxError(err, "workspaces")
	}
	workspaces := make([]*platformdomain.Workspace, len(dbWorkspaces))
	for i, w := range dbWorkspaces {
		workspaces[i] = mapWorkspaceToDomain(&w)
	}
	return workspaces, nil
}

func (s *WorkspaceStore) GetWorkspacesByIDs(ctx context.Context, workspaceIDs []string) ([]*platformdomain.Workspace, error) {
	if len(workspaceIDs) == 0 {
		return []*platformdomain.Workspace{}, nil
	}

	query, args, err := buildInQuery(`SELECT * FROM core_platform.workspaces WHERE id IN (?) AND deleted_at IS NULL`, workspaceIDs)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}

	var dbWorkspaces []models.Workspace
	if err := s.db.Get(ctx).SelectContext(ctx, &dbWorkspaces, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "workspaces")
	}

	workspaces := make([]*platformdomain.Workspace, len(dbWorkspaces))
	for i, w := range dbWorkspaces {
		workspaces[i] = mapWorkspaceToDomain(&w)
	}
	return workspaces, nil
}

func (s *WorkspaceStore) ListUserWorkspaces(ctx context.Context, userID string) ([]*platformdomain.Workspace, error) {
	query := `
		SELECT w.* FROM core_platform.workspaces w
		JOIN core_identity.user_workspace_roles uwr ON uwr.workspace_id = w.id
		WHERE uwr.user_id = ? AND uwr.revoked_at IS NULL AND w.deleted_at IS NULL`

	var dbWorkspaces []models.Workspace
	if err := s.db.Get(ctx).SelectContext(ctx, &dbWorkspaces, query, userID); err != nil {
		return nil, TranslateSqlxError(err, "workspaces")
	}

	workspaces := make([]*platformdomain.Workspace, len(dbWorkspaces))
	for i, w := range dbWorkspaces {
		workspaces[i] = mapWorkspaceToDomain(&w)
	}
	return workspaces, nil
}

func (s *WorkspaceStore) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	query := `UPDATE core_platform.workspaces SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), workspaceID)
	if err != nil {
		return TranslateSqlxError(err, "workspaces")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func mapWorkspaceToDomain(w *models.Workspace) *platformdomain.Workspace {
	// Helper to safely dereference nullable strings
	strVal := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}
	intVal := func(i *int) int {
		if i != nil {
			return *i
		}
		return 0
	}
	int64Val := func(i *int64) int64 {
		if i != nil {
			return *i
		}
		return 0
	}

	settingsStr := strVal(w.Settings)
	settings := unmarshalMetadataOrEmpty(settingsStr, "workspaces", "settings")
	var features []string
	featuresStr := strVal(w.Features)
	unmarshalJSONField(featuresStr, &features, "workspaces", "features")

	return &platformdomain.Workspace{
		ID:            w.ID,
		Name:          w.Name,
		Slug:          w.Slug,
		ShortCode:     strVal(w.ShortCode),
		Description:   strVal(w.Description),
		LogoURL:       strVal(w.LogoURL),
		PrimaryColor:  strVal(w.PrimaryColor),
		AccentColor:   strVal(w.AccentColor),
		Settings:      settings,
		Features:      features,
		StorageBucket: strVal(w.StorageBucket),
		MaxUsers:      intVal(w.MaxUsers),
		MaxCases:      intVal(w.MaxCases),
		MaxStorage:    int64Val(w.MaxStorage),
		IsActive:      w.IsActive,
		IsSuspended:   w.IsSuspended,
		SuspendReason: strVal(w.SuspendReason),
		CreatedAt:     w.CreatedAt,
		UpdatedAt:     w.UpdatedAt,
		DeletedAt:     w.DeletedAt,
	}
}

// =============================================================================
// User-Workspace Roles
// =============================================================================

func (s *WorkspaceStore) CreateUserWorkspaceRole(ctx context.Context, role *platformdomain.UserWorkspaceRole) error {
	permissions, err := marshalJSONString(role.Permissions, "permissions")
	if err != nil {
		return fmt.Errorf("create user workspace role: %w", err)
	}

	normalizePersistedUUID(&role.ID)
	query := `
		INSERT INTO core_identity.user_workspace_roles (
			id, user_id, workspace_id, role, permissions, invited_by, revoked_at, expires_at, created_at, updated_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		role.ID, role.UserID, role.WorkspaceID, string(role.Role), permissions,
		role.InvitedBy, role.RevokedAt, role.ExpiresAt, role.CreatedAt, role.UpdatedAt,
	).Scan(&role.ID)
	return TranslateSqlxError(err, "user_workspace_roles")
}

func (s *WorkspaceStore) GetUserWorkspaceRoles(ctx context.Context, userID string) ([]*platformdomain.UserWorkspaceRole, error) {
	var dbRoles []models.UserWorkspaceRole
	query := `SELECT * FROM core_identity.user_workspace_roles WHERE user_id = ?`
	if err := s.db.Get(ctx).SelectContext(ctx, &dbRoles, query, userID); err != nil {
		return nil, TranslateSqlxError(err, "user_workspace_roles")
	}
	roles := make([]*platformdomain.UserWorkspaceRole, len(dbRoles))
	for i, r := range dbRoles {
		roles[i] = mapUserWorkspaceRoleToDomain(&r)
	}
	return roles, nil
}

func (s *WorkspaceStore) GetWorkspaceUsers(ctx context.Context, workspaceID string) ([]*platformdomain.UserWorkspaceRole, error) {
	var dbRoles []models.UserWorkspaceRole
	query := `SELECT * FROM core_identity.user_workspace_roles WHERE workspace_id = ?`
	if err := s.db.Get(ctx).SelectContext(ctx, &dbRoles, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "user_workspace_roles")
	}
	roles := make([]*platformdomain.UserWorkspaceRole, len(dbRoles))
	for i, r := range dbRoles {
		roles[i] = mapUserWorkspaceRoleToDomain(&r)
	}
	return roles, nil
}

func (s *WorkspaceStore) DeleteUserWorkspaceRole(ctx context.Context, userID, workspaceID string) error {
	query := `DELETE FROM core_identity.user_workspace_roles WHERE user_id = ? AND workspace_id = ?`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, userID, workspaceID)
	if err != nil {
		return TranslateSqlxError(err, "user_workspace_roles")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func mapUserWorkspaceRoleToDomain(r *models.UserWorkspaceRole) *platformdomain.UserWorkspaceRole {
	var permissions []string
	unmarshalJSONField(r.Permissions, &permissions, "user_workspace_roles", "permissions")
	return &platformdomain.UserWorkspaceRole{
		ID:          r.ID,
		UserID:      r.UserID,
		WorkspaceID: r.WorkspaceID,
		Role:        platformdomain.WorkspaceRole(r.Role),
		Permissions: permissions,
		InvitedBy:   r.InvitedBy,
		RevokedAt:   r.RevokedAt,
		ExpiresAt:   r.ExpiresAt,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// =============================================================================
// Teams
// =============================================================================

func (s *WorkspaceStore) CreateTeam(ctx context.Context, team *platformdomain.Team) error {
	settings, err := marshalJSONString(team.Settings, "settings")
	if err != nil {
		return fmt.Errorf("create team: %w", err)
	}
	keywords, err := marshalJSONString(team.AutoAssignKeywords, "auto_assign_keywords")
	if err != nil {
		return fmt.Errorf("create team: %w", err)
	}

	normalizePersistedUUID(&team.ID)
	query := `
		INSERT INTO core_platform.teams (
			id, workspace_id, name, description, email_address, settings,
			response_time_hours, resolution_time_hours, auto_assign, auto_assign_keywords,
			is_active, created_at, updated_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		team.ID, team.WorkspaceID, team.Name, team.Description, team.EmailAddress, settings,
		team.ResponseTimeHours, team.ResolutionTimeHours, team.AutoAssign, keywords,
		team.IsActive, team.CreatedAt, team.UpdatedAt,
	).Scan(&team.ID)
	return TranslateSqlxError(err, "teams")
}

func (s *WorkspaceStore) GetTeam(ctx context.Context, teamID string) (*platformdomain.Team, error) {
	var dbTeam models.Team
	query := `SELECT * FROM core_platform.teams WHERE id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbTeam, query, teamID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "teams")
	}
	return mapTeamToDomain(&dbTeam), nil
}

func (s *WorkspaceStore) ListWorkspaceTeams(ctx context.Context, workspaceID string) ([]*platformdomain.Team, error) {
	var dbTeams []models.Team
	query := `SELECT * FROM core_platform.teams WHERE workspace_id = ?`
	if err := s.db.Get(ctx).SelectContext(ctx, &dbTeams, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "teams")
	}
	teams := make([]*platformdomain.Team, len(dbTeams))
	for i, t := range dbTeams {
		teams[i] = mapTeamToDomain(&t)
	}
	return teams, nil
}

func mapTeamToDomain(t *models.Team) *platformdomain.Team {
	settings := unmarshalMetadataOrEmpty(t.Settings, "teams", "settings")
	var keywords []string
	unmarshalJSONField(t.AutoAssignKeywords, &keywords, "teams", "auto_assign_keywords")

	return &platformdomain.Team{
		ID:                  t.ID,
		WorkspaceID:         t.WorkspaceID,
		Name:                t.Name,
		Description:         t.Description,
		EmailAddress:        t.EmailAddress,
		Settings:            settings,
		ResponseTimeHours:   t.ResponseTimeHours,
		ResolutionTimeHours: t.ResolutionTimeHours,
		AutoAssign:          t.AutoAssign,
		AutoAssignKeywords:  keywords,
		IsActive:            t.IsActive,
		CreatedAt:           t.CreatedAt,
		UpdatedAt:           t.UpdatedAt,
	}
}

// =============================================================================
// Team Members
// =============================================================================

func (s *WorkspaceStore) AddTeamMember(ctx context.Context, member *platformdomain.TeamMember) error {
	normalizePersistedUUID(&member.ID)
	query := `
		INSERT INTO core_platform.team_members (
			id, team_id, user_id, workspace_id, role, is_active, joined_at, created_at, updated_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		member.ID, member.TeamID, member.UserID, member.WorkspaceID, string(member.Role),
		member.IsActive, member.JoinedAt, member.CreatedAt, member.UpdatedAt,
	).Scan(&member.ID)
	return TranslateSqlxError(err, "team_members")
}

func (s *WorkspaceStore) GetTeamMembers(ctx context.Context, workspaceID, teamID string) ([]*platformdomain.TeamMember, error) {
	var dbMembers []models.TeamMember
	query := `SELECT * FROM core_platform.team_members WHERE workspace_id = ? AND team_id = ?`
	if err := s.db.Get(ctx).SelectContext(ctx, &dbMembers, query, workspaceID, teamID); err != nil {
		return nil, TranslateSqlxError(err, "team_members")
	}
	members := make([]*platformdomain.TeamMember, len(dbMembers))
	for i, m := range dbMembers {
		members[i] = &platformdomain.TeamMember{
			ID:          m.ID,
			TeamID:      m.TeamID,
			UserID:      m.UserID,
			WorkspaceID: m.WorkspaceID,
			Role:        platformdomain.TeamMemberRole(m.Role),
			IsActive:    m.IsActive,
			JoinedAt:    m.JoinedAt,
			CreatedAt:   m.CreatedAt,
			UpdatedAt:   m.UpdatedAt,
		}
	}
	return members, nil
}

// =============================================================================
// Workspace Settings
// =============================================================================

func (s *WorkspaceStore) GetWorkspaceSettings(ctx context.Context, workspaceID string) (*platformdomain.WorkspaceSettings, error) {
	var dbSettings models.WorkspaceSettings
	query := `SELECT * FROM core_platform.workspace_settings WHERE workspace_id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbSettings, query, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "workspace_settings")
	}

	var settings platformdomain.WorkspaceSettings
	if err := json.Unmarshal([]byte(dbSettings.SettingsJSON), &settings); err != nil {
		settings = platformdomain.WorkspaceSettings{}
	}

	settings.ID = dbSettings.ID
	settings.WorkspaceID = dbSettings.WorkspaceID
	settings.EmailFromName = dbSettings.EmailFromName
	settings.EmailFromAddress = dbSettings.EmailFromAddress
	settings.CreatedAt = dbSettings.CreatedAt
	settings.UpdatedAt = dbSettings.UpdatedAt

	return &settings, nil
}

func (s *WorkspaceStore) CreateWorkspaceSettings(ctx context.Context, settings *platformdomain.WorkspaceSettings) error {
	settingsJSON, err := marshalJSONString(settings, "settings_json")
	if err != nil {
		return fmt.Errorf("create workspace settings: %w", err)
	}

	normalizePersistedUUID(&settings.ID)
	query := `
		INSERT INTO core_platform.workspace_settings (
			id, workspace_id, email_from_name, email_from_address, settings_json, created_at, updated_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		settings.ID, settings.WorkspaceID, settings.EmailFromName, settings.EmailFromAddress,
		settingsJSON, settings.CreatedAt, settings.UpdatedAt,
	).Scan(&settings.ID)
	return TranslateSqlxError(err, "workspace_settings")
}
