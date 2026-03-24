package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

const (
	errorTrackingExtensionSlug = "error-tracking"
	errorTrackingSchemaName    = "ext_demandops_error_tracking"
)

type ErrorMonitoringStore struct {
	db     *SqlxDB
	schema string
}

func NewErrorMonitoringStore(db *SqlxDB) *ErrorMonitoringStore {
	return &ErrorMonitoringStore{db: db, schema: errorTrackingSchemaName}
}

func (s *ErrorMonitoringStore) query(query string) string {
	return strings.ReplaceAll(query, "${SCHEMA_NAME}", s.schema)
}

func (s *ErrorMonitoringStore) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.Get(ctx).ExecContext(ctx, s.query(query), args...)
}

func (s *ErrorMonitoringStore) getContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return s.db.Get(ctx).GetContext(ctx, dest, s.query(query), args...)
}

func (s *ErrorMonitoringStore) selectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return s.db.Get(ctx).SelectContext(ctx, dest, s.query(query), args...)
}

func (s *ErrorMonitoringStore) queryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return s.db.Get(ctx).QueryRowxContext(ctx, s.query(query), args...)
}

func (s *ErrorMonitoringStore) lookupInstallIDForWorkspace(ctx context.Context, workspaceID string) (string, error) {
	var installID string
	err := s.queryRowxContext(ctx,
		`SELECT id
		 FROM core_platform.installed_extensions
		 WHERE workspace_id = ? AND slug = ? AND status = ? AND deleted_at IS NULL
		 ORDER BY activated_at DESC NULLS LAST, installed_at DESC
		 LIMIT 1`,
		workspaceID, errorTrackingExtensionSlug, platformdomain.ExtensionStatusActive).Scan(&installID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("error-tracking extension is not installed for workspace %s", workspaceID)
		}
		return "", err
	}
	return installID, nil
}

func (s *ErrorMonitoringStore) lookupProjectScope(ctx context.Context, projectID string) (workspaceID string, installID string, err error) {
	err = s.queryRowxContext(ctx,
		`SELECT workspace_id, extension_install_id
		 FROM ${SCHEMA_NAME}.projects
		 WHERE id = ? AND deleted_at IS NULL`,
		projectID).Scan(&workspaceID, &installID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", shared.ErrNotFound
		}
		return "", "", err
	}
	return workspaceID, installID, nil
}

// =============================================================================
// Project Operations
// =============================================================================

func (s *ErrorMonitoringStore) CreateProject(ctx context.Context, project *observabilitydomain.Project) error {
	installID, err := s.lookupInstallIDForWorkspace(ctx, project.WorkspaceID)
	if err != nil {
		return err
	}
	normalizePersistedUUID(&project.ID)

	query := `
		INSERT INTO ${SCHEMA_NAME}.projects (
			id, workspace_id, extension_install_id, team_id, name, slug, repository, platform, environment,
			dsn, public_key, secret_key, app_key, project_number, events_per_hour,
			storage_quota_mb, retention_days, status, last_event_at, event_count, deleted_at,
			created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?
		)
		RETURNING id`

	err = s.queryRowxContext(ctx, query,
		project.ID, project.WorkspaceID, installID, nullableUUIDValue(project.TeamID), project.Name, project.Slug, project.Repository, project.Platform, project.Environment,
		project.DSN, project.PublicKey, project.SecretKey, project.AppKey, project.ProjectNumber, project.EventsPerHour,
		project.StorageQuotaMB, project.RetentionDays, project.Status, project.LastEventAt, project.EventCount, nil,
		project.CreatedAt, project.UpdatedAt,
	).Scan(&project.ID)
	return TranslateSqlxError(err, "projects")
}

func (s *ErrorMonitoringStore) GetProject(ctx context.Context, projectID string) (*observabilitydomain.Project, error) {
	var model models.Project
	query := `SELECT * FROM ${SCHEMA_NAME}.projects WHERE id = ? AND deleted_at IS NULL`
	err := s.getContext(ctx, &model, query, projectID)
	if err != nil {
		return nil, TranslateSqlxError(err, "projects")
	}
	return s.mapProjectToDomain(&model), nil
}

// GetProjectInWorkspace retrieves a project only if it belongs to the specified workspace.
// Returns ErrNotFound if project doesn't exist OR belongs to different workspace (defense-in-depth).
func (s *ErrorMonitoringStore) GetProjectInWorkspace(ctx context.Context, workspaceID, projectID string) (*observabilitydomain.Project, error) {
	var model models.Project
	query := `SELECT * FROM ${SCHEMA_NAME}.projects WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	err := s.getContext(ctx, &model, query, projectID, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "projects")
	}
	return s.mapProjectToDomain(&model), nil
}

func (s *ErrorMonitoringStore) GetProjectByKey(ctx context.Context, projectKey string) (*observabilitydomain.Project, error) {
	var model models.Project
	query := `SELECT * FROM ${SCHEMA_NAME}.projects WHERE public_key = ? AND deleted_at IS NULL`
	err := s.getContext(ctx, &model, query, projectKey)
	if err != nil {
		return nil, TranslateSqlxError(err, "projects")
	}
	return s.mapProjectToDomain(&model), nil
}

func (s *ErrorMonitoringStore) UpdateProject(ctx context.Context, project *observabilitydomain.Project) error {
	query := `
		UPDATE ${SCHEMA_NAME}.projects SET
			name = ?, slug = ?, repository = ?, platform = ?, environment = ?,
			dsn = ?, public_key = ?, secret_key = ?, app_key = ?, project_number = ?,
			events_per_hour = ?, storage_quota_mb = ?, retention_days = ?, status = ?,
			last_event_at = ?, event_count = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	result, err := s.execContext(ctx, query,
		project.Name, project.Slug, project.Repository, project.Platform, project.Environment,
		project.DSN, project.PublicKey, project.SecretKey, project.AppKey, project.ProjectNumber,
		project.EventsPerHour, project.StorageQuotaMB, project.RetentionDays, project.Status,
		project.LastEventAt, project.EventCount, project.UpdatedAt,
		project.ID, project.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "projects")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ErrorMonitoringStore) IncrementEventCount(ctx context.Context, workspaceID, projectID string, lastEventAt *time.Time) (int64, error) {
	now := time.Now()
	query := `
		UPDATE ${SCHEMA_NAME}.projects
		SET event_count = event_count + 1,
		    last_event_at = COALESCE(?, last_event_at),
		    updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL
	`

	_, err := s.execContext(ctx, query, lastEventAt, now, projectID, workspaceID)
	if err != nil {
		return 0, TranslateSqlxError(err, "projects")
	}

	// Fetch the new count
	var newCount int64
	selectQuery := `SELECT event_count FROM ${SCHEMA_NAME}.projects WHERE id = ? AND workspace_id = ?`
	if err := s.getContext(ctx, &newCount, selectQuery, projectID, workspaceID); err != nil {
		return 0, TranslateSqlxError(err, "projects")
	}

	return newCount, nil
}

func (s *ErrorMonitoringStore) ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*observabilitydomain.Project, error) {
	var dbModels []models.Project
	query := `SELECT * FROM ${SCHEMA_NAME}.projects WHERE workspace_id = ? AND deleted_at IS NULL`
	err := s.selectContext(ctx, &dbModels, query, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "projects")
	}

	result := make([]*observabilitydomain.Project, len(dbModels))
	for i, m := range dbModels {
		result[i] = s.mapProjectToDomain(&m)
	}
	return result, nil
}

func (s *ErrorMonitoringStore) ListAllProjects(ctx context.Context) ([]*observabilitydomain.Project, error) {
	var dbModels []models.Project
	query := `SELECT * FROM ${SCHEMA_NAME}.projects WHERE deleted_at IS NULL ORDER BY workspace_id, name`
	err := s.selectContext(ctx, &dbModels, query)
	if err != nil {
		return nil, TranslateSqlxError(err, "projects")
	}

	result := make([]*observabilitydomain.Project, len(dbModels))
	for i, m := range dbModels {
		result[i] = s.mapProjectToDomain(&m)
	}
	return result, nil
}

func (s *ErrorMonitoringStore) GetProjectsByIDs(ctx context.Context, projectIDs []string) ([]*observabilitydomain.Project, error) {
	if len(projectIDs) == 0 {
		return []*observabilitydomain.Project{}, nil
	}

	query, args, err := buildInQuery(s.query(`SELECT * FROM ${SCHEMA_NAME}.projects WHERE id IN (?) AND deleted_at IS NULL`), projectIDs)
	if err != nil {
		return nil, err
	}

	var dbModels []models.Project
	if err := s.db.Get(ctx).SelectContext(ctx, &dbModels, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "projects")
	}

	result := make([]*observabilitydomain.Project, len(dbModels))
	for i, m := range dbModels {
		result[i] = s.mapProjectToDomain(&m)
	}
	return result, nil
}

func (s *ErrorMonitoringStore) DeleteProject(ctx context.Context, workspaceID, projectID string) error {
	deletedAt := time.Now().UTC()
	query := `UPDATE ${SCHEMA_NAME}.projects SET deleted_at = ? WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	result, err := s.execContext(ctx, query, deletedAt, projectID, workspaceID)
	if err != nil {
		return TranslateSqlxError(err, "projects")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

// =============================================================================
// Application Aliases (Application is same as Project in domain)
// =============================================================================

func (s *ErrorMonitoringStore) GetApplication(ctx context.Context, workspaceID, appID string) (*observabilitydomain.Application, error) {
	return s.GetProjectInWorkspace(ctx, workspaceID, appID)
}

func (s *ErrorMonitoringStore) GetApplicationByKey(ctx context.Context, appKey string) (*observabilitydomain.Application, error) {
	return s.GetProjectByKey(ctx, appKey)
}

func (s *ErrorMonitoringStore) ListWorkspaceApplications(ctx context.Context, workspaceID string) ([]*observabilitydomain.Application, error) {
	return s.ListWorkspaceProjects(ctx, workspaceID)
}

// =============================================================================
// Git Repo Operations
// =============================================================================

func (s *ErrorMonitoringStore) GetGitRepoByID(ctx context.Context, repoID string) (*observabilitydomain.GitRepo, error) {
	var model models.GitRepo
	query := `SELECT * FROM ${SCHEMA_NAME}.git_repos WHERE id = ?`
	err := s.getContext(ctx, &model, query, repoID)
	if err != nil {
		return nil, TranslateSqlxError(err, "git_repos")
	}
	return s.mapGitRepoToDomain(&model), nil
}

func (s *ErrorMonitoringStore) ListGitReposByApplication(ctx context.Context, applicationID string) ([]*observabilitydomain.GitRepo, error) {
	var dbModels []models.GitRepo
	query := `SELECT * FROM ${SCHEMA_NAME}.git_repos WHERE application_id = ? ORDER BY created_at ASC`
	err := s.selectContext(ctx, &dbModels, query, applicationID)
	if err != nil {
		return nil, TranslateSqlxError(err, "git_repos")
	}

	result := make([]*observabilitydomain.GitRepo, len(dbModels))
	for i, m := range dbModels {
		result[i] = s.mapGitRepoToDomain(&m)
	}
	return result, nil
}

// =============================================================================
// Mappers
// =============================================================================

func (s *ErrorMonitoringStore) mapProjectToDomain(m *models.Project) *observabilitydomain.Project {
	return &observabilitydomain.Project{
		ID:             m.ID,
		WorkspaceID:    m.WorkspaceID,
		TeamID:         derefStringPtr(m.TeamID),
		Name:           m.Name,
		Slug:           m.Slug,
		Repository:     m.Repository,
		Platform:       m.Platform,
		Environment:    m.Environment,
		DSN:            m.DSN,
		PublicKey:      m.PublicKey,
		SecretKey:      m.SecretKey,
		AppKey:         m.AppKey,
		ProjectNumber:  m.ProjectNumber,
		EventsPerHour:  m.EventsPerHour,
		StorageQuotaMB: m.StorageQuotaMB,
		RetentionDays:  m.RetentionDays,
		Status:         m.Status,
		LastEventAt:    m.LastEventAt,
		EventCount:     m.EventCount,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func (s *ErrorMonitoringStore) mapGitRepoToDomain(m *models.GitRepo) *observabilitydomain.GitRepo {
	return &observabilitydomain.GitRepo{
		ID:            m.ID,
		ApplicationID: m.ApplicationID,
		WorkspaceID:   m.WorkspaceID,
		RepoURL:       m.RepoURL,
		DefaultBranch: m.DefaultBranch,
		AccessToken:   m.AccessToken,
		PathPrefix:    m.PathPrefix,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func (s *ErrorMonitoringStore) mapIssueToDomain(m *models.Issue) (*observabilitydomain.Issue, error) {
	var relatedCaseIDs []string
	if m.RelatedCaseIDs != "" {
		if err := json.Unmarshal([]byte(m.RelatedCaseIDs), &relatedCaseIDs); err != nil {
			return nil, fmt.Errorf("unmarshal issue %s related_case_ids: %w", m.ID, err)
		}
	}

	var tags map[string]string
	if m.Tags != "" {
		if err := json.Unmarshal([]byte(m.Tags), &tags); err != nil {
			return nil, fmt.Errorf("unmarshal issue %s tags: %w", m.ID, err)
		}
	}

	return &observabilitydomain.Issue{
		ID:                m.ID,
		WorkspaceID:       m.WorkspaceID,
		ProjectID:         m.ProjectID,
		ApplicationID:     m.ProjectID,
		Title:             m.Title,
		Culprit:           m.Culprit,
		Fingerprint:       m.Fingerprint,
		Status:            m.Status,
		Level:             m.Level,
		Type:              m.Type,
		FirstSeen:         m.FirstSeen,
		LastSeen:          m.LastSeen,
		EventCount:        m.EventCount,
		UserCount:         m.UserCount,
		AssignedTo:        derefStringPtr(m.AssignedTo),
		ResolvedAt:        m.ResolvedAt,
		ResolvedBy:        derefStringPtr(m.ResolvedBy),
		Resolution:        m.Resolution,
		ResolutionNotes:   m.ResolutionNotes,
		ResolvedInCommit:  m.ResolvedInCommit,
		ResolvedInVersion: m.ResolvedInVersion,
		HasRelatedCase:    m.HasRelatedCase,
		RelatedCaseIDs:    relatedCaseIDs,
		Tags:              tags,
		Permalink:         m.Permalink,
		ShortID:           m.ShortID,
		Logger:            m.Logger,
		Platform:          m.Platform,
		LastEventID:       m.LastEventID,
	}, nil
}

func (s *ErrorMonitoringStore) mapEventToDomain(m *models.ErrorEvent) (*observabilitydomain.ErrorEvent, error) {
	var exception []observabilitydomain.ExceptionData
	if m.Exception != "" {
		if err := json.Unmarshal([]byte(m.Exception), &exception); err != nil {
			return nil, fmt.Errorf("unmarshal event %s exception: %w", m.ID, err)
		}
	}

	var stacktrace *observabilitydomain.StacktraceData
	if m.Stacktrace != "" {
		var st observabilitydomain.StacktraceData
		if err := json.Unmarshal([]byte(m.Stacktrace), &st); err != nil {
			return nil, fmt.Errorf("unmarshal event %s stacktrace: %w", m.ID, err)
		}
		stacktrace = &st
	}

	var user *observabilitydomain.UserContext
	if m.User != "" {
		var u observabilitydomain.UserContext
		if err := json.Unmarshal([]byte(m.User), &u); err != nil {
			return nil, fmt.Errorf("unmarshal event %s user: %w", m.ID, err)
		}
		user = &u
	}

	var request *observabilitydomain.RequestContext
	if m.Request != "" {
		var r observabilitydomain.RequestContext
		if err := json.Unmarshal([]byte(m.Request), &r); err != nil {
			return nil, fmt.Errorf("unmarshal event %s request: %w", m.ID, err)
		}
		request = &r
	}

	var tags map[string]string
	if m.Tags != "" {
		if err := json.Unmarshal([]byte(m.Tags), &tags); err != nil {
			return nil, fmt.Errorf("unmarshal event %s tags: %w", m.ID, err)
		}
	}

	extra := unmarshalMetadataOrEmpty(m.Extra, "error_events", "extra")
	contexts := unmarshalMetadataOrEmpty(m.Contexts, "error_events", "contexts")

	var breadcrumbs []observabilitydomain.Breadcrumb
	if m.Breadcrumbs != "" {
		if err := json.Unmarshal([]byte(m.Breadcrumbs), &breadcrumbs); err != nil {
			return nil, fmt.Errorf("unmarshal event %s breadcrumbs: %w", m.ID, err)
		}
	}

	var fingerprint []string
	if m.Fingerprint != "" {
		if err := json.Unmarshal([]byte(m.Fingerprint), &fingerprint); err != nil {
			return nil, fmt.Errorf("unmarshal event %s fingerprint: %w", m.ID, err)
		}
	}

	return &observabilitydomain.ErrorEvent{
		ID:          m.ID,
		EventID:     m.EventID,
		ProjectID:   m.ProjectID,
		IssueID:     derefStringPtr(m.IssueID),
		Timestamp:   m.Timestamp,
		Received:    m.Received,
		Message:     m.Message,
		Level:       m.Level,
		Logger:      m.Logger,
		Platform:    m.Platform,
		Environment: m.Environment,
		Release:     m.Release,
		Dist:        m.Dist,
		Exception:   exception,
		Stacktrace:  stacktrace,
		User:        user,
		Request:     request,
		Tags:        tags,
		Extra:       extra,
		Contexts:    contexts,
		Breadcrumbs: breadcrumbs,
		Fingerprint: fingerprint,
		DataURL:     m.DataURL,
		Size:        m.Size,
		ProcessedAt: m.ProcessedAt,
		GroupedAt:   m.GroupedAt,
	}, nil
}

func (s *ErrorMonitoringStore) mapAlertToDomain(m *models.Alert) (*observabilitydomain.Alert, error) {
	var conditions []observabilitydomain.AlertCondition
	if m.Conditions != "" {
		if err := json.Unmarshal([]byte(m.Conditions), &conditions); err != nil {
			return nil, fmt.Errorf("unmarshal alert %s conditions: %w", m.ID, err)
		}
	}

	var actions []observabilitydomain.AlertAction
	if m.Actions != "" {
		if err := json.Unmarshal([]byte(m.Actions), &actions); err != nil {
			return nil, fmt.Errorf("unmarshal alert %s actions: %w", m.ID, err)
		}
	}

	return &observabilitydomain.Alert{
		ID:              m.ID,
		ProjectID:       m.ProjectID,
		Name:            m.Name,
		Conditions:      conditions,
		Frequency:       time.Duration(m.Frequency),
		Actions:         actions,
		Enabled:         m.Enabled,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
		CooldownMinutes: m.CooldownMinutes,
		LastTriggered:   m.LastTriggered,
	}, nil
}
