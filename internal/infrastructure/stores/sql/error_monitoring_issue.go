package sql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
)

// =============================================================================
// Issue Operations
// =============================================================================

func (s *ErrorMonitoringStore) CreateIssue(ctx context.Context, issue *observabilitydomain.Issue) error {
	workspaceID, installID, err := s.lookupProjectScope(ctx, issue.ProjectID)
	if err != nil {
		return err
	}
	normalizePersistedUUID(&issue.ID)
	relatedCases, err := marshalJSONString(issue.RelatedCaseIDs, "related_case_ids")
	if err != nil {
		return fmt.Errorf("create issue: %w", err)
	}
	tags, err := marshalJSONString(issue.Tags, "tags")
	if err != nil {
		return fmt.Errorf("create issue: %w", err)
	}

	now := time.Now()
	query := `
		INSERT INTO ${SCHEMA_NAME}.issues (
			id, workspace_id, extension_install_id, project_id, title, culprit, fingerprint, status, level, type,
			first_seen, last_seen, event_count, user_count, assigned_to, resolved_at, resolved_by,
			resolution, resolution_notes, resolved_in_commit, resolved_in_version,
			has_related_case, related_case_ids, tags, permalink, short_id, logger, platform,
			last_event_id, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?
		)
		RETURNING id`

	err = s.queryRowxContext(ctx, query,
		issue.ID, workspaceID, installID, issue.ProjectID, issue.Title, issue.Culprit, issue.Fingerprint,
		issue.Status, issue.Level, issue.Type,
		issue.FirstSeen, issue.LastSeen, issue.EventCount, issue.UserCount, nullableUUIDValue(issue.AssignedTo),
		issue.ResolvedAt, nullableLegacyUUIDValue(issue.ResolvedBy), issue.Resolution, issue.ResolutionNotes,
		issue.ResolvedInCommit, issue.ResolvedInVersion,
		issue.HasRelatedCase, relatedCases, tags, issue.Permalink, issue.ShortID, issue.Logger, issue.Platform,
		issue.LastEventID, now, now,
	).Scan(&issue.ID)
	return TranslateSqlxError(err, "issues")
}

func (s *ErrorMonitoringStore) CreateOrUpdateIssueByFingerprint(ctx context.Context, issue *observabilitydomain.Issue) (*observabilitydomain.Issue, bool, error) {
	workspaceID, installID, err := s.lookupProjectScope(ctx, issue.ProjectID)
	if err != nil {
		return nil, false, err
	}
	normalizePersistedUUID(&issue.ID)
	relatedCases, err := marshalJSONString(issue.RelatedCaseIDs, "related_case_ids")
	if err != nil {
		return nil, false, fmt.Errorf("upsert issue: %w", err)
	}
	tags, err := marshalJSONString(issue.Tags, "tags")
	if err != nil {
		return nil, false, fmt.Errorf("upsert issue: %w", err)
	}

	now := time.Now()

	// Atomic upsert using ON CONFLICT to prevent race conditions.
	// If a concurrent request inserts the same fingerprint, this will update instead of failing.
	upsertQuery := `
		INSERT INTO ${SCHEMA_NAME}.issues AS issues (
			id, workspace_id, extension_install_id, project_id, title, culprit, fingerprint, status, level, type,
			first_seen, last_seen, event_count, user_count, assigned_to, resolved_at, resolved_by,
			resolution, resolution_notes, resolved_in_commit, resolved_in_version,
			has_related_case, related_case_ids, tags, permalink, short_id, logger, platform,
			last_event_id, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?
		)
		ON CONFLICT(project_id, fingerprint) DO UPDATE SET
			last_seen = excluded.last_seen,
			event_count = issues.event_count + 1,
			last_event_id = excluded.last_event_id,
			updated_at = excluded.updated_at
		RETURNING id, (xmax = 0) AS was_created`

	var issueID string
	var wasCreated bool
	err = s.queryRowxContext(ctx, upsertQuery,
		issue.ID, workspaceID, installID, issue.ProjectID, issue.Title, issue.Culprit, issue.Fingerprint,
		issue.Status, issue.Level, issue.Type,
		issue.FirstSeen, issue.LastSeen, issue.EventCount, issue.UserCount, nullableUUIDValue(issue.AssignedTo),
		issue.ResolvedAt, nullableLegacyUUIDValue(issue.ResolvedBy), issue.Resolution, issue.ResolutionNotes,
		issue.ResolvedInCommit, issue.ResolvedInVersion,
		issue.HasRelatedCase, relatedCases, tags, issue.Permalink, issue.ShortID, issue.Logger, issue.Platform,
		issue.LastEventID, now, now,
	).Scan(&issueID, &wasCreated)
	if err != nil {
		return nil, false, fmt.Errorf("upsert issue by fingerprint: %w", err)
	}

	resultIssue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return nil, false, fmt.Errorf("fetch upserted issue: %w", err)
	}

	issue.ID = resultIssue.ID

	return resultIssue, wasCreated, nil
}

func (s *ErrorMonitoringStore) GetIssue(ctx context.Context, issueID string) (*observabilitydomain.Issue, error) {
	var model models.Issue
	query := `SELECT * FROM ${SCHEMA_NAME}.issues WHERE id = ?`
	err := s.getContext(ctx, &model, query, issueID)
	if err != nil {
		return nil, TranslateSqlxError(err, "issues")
	}
	return s.mapIssueToDomain(&model)
}

// GetIssueInWorkspace retrieves an issue only if it belongs to the specified workspace.
// Returns ErrNotFound if issue doesn't exist OR belongs to different workspace (defense-in-depth).
func (s *ErrorMonitoringStore) GetIssueInWorkspace(ctx context.Context, workspaceID, issueID string) (*observabilitydomain.Issue, error) {
	var model models.Issue
	query := `SELECT * FROM ${SCHEMA_NAME}.issues WHERE id = ? AND workspace_id = ?`
	err := s.getContext(ctx, &model, query, issueID, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "issues")
	}
	return s.mapIssueToDomain(&model)
}

func (s *ErrorMonitoringStore) GetIssuesByIDs(ctx context.Context, issueIDs []string) ([]*observabilitydomain.Issue, error) {
	if len(issueIDs) == 0 {
		return []*observabilitydomain.Issue{}, nil
	}

	query, args, err := buildInQuery(s.query(`SELECT * FROM ${SCHEMA_NAME}.issues WHERE id IN (?)`), issueIDs)
	if err != nil {
		return nil, err
	}

	var dbIssues []models.Issue
	if err := s.db.Get(ctx).SelectContext(ctx, &dbIssues, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "issues")
	}

	issues := make([]*observabilitydomain.Issue, 0, len(dbIssues))
	for _, m := range dbIssues {
		issue, err := s.mapIssueToDomain(&m)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (s *ErrorMonitoringStore) GetIssueByFingerprint(ctx context.Context, projectID, fingerprint string) (*observabilitydomain.Issue, error) {
	var model models.Issue
	query := `SELECT * FROM ${SCHEMA_NAME}.issues WHERE project_id = ? AND fingerprint = ?`
	err := s.getContext(ctx, &model, query, projectID, fingerprint)
	if err != nil {
		return nil, TranslateSqlxError(err, "issues")
	}
	return s.mapIssueToDomain(&model)
}

func (s *ErrorMonitoringStore) UpdateIssue(ctx context.Context, issue *observabilitydomain.Issue) error {
	relatedCases, err := marshalJSONString(issue.RelatedCaseIDs, "related_case_ids")
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	tags, err := marshalJSONString(issue.Tags, "tags")
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	query := `
		UPDATE ${SCHEMA_NAME}.issues SET
			title = ?, culprit = ?, status = ?, level = ?, type = ?,
			first_seen = ?, last_seen = ?, event_count = ?, user_count = ?,
			assigned_to = ?, resolved_at = ?, resolved_by = ?, resolution = ?,
			resolution_notes = ?, resolved_in_commit = ?, resolved_in_version = ?,
			has_related_case = ?, related_case_ids = ?, tags = ?,
			logger = ?, platform = ?, last_event_id = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	result, err := s.execContext(ctx, query,
		issue.Title, issue.Culprit, issue.Status, issue.Level, issue.Type,
		issue.FirstSeen, issue.LastSeen, issue.EventCount, issue.UserCount,
		nullableUUIDValue(issue.AssignedTo), issue.ResolvedAt, nullableLegacyUUIDValue(issue.ResolvedBy), issue.Resolution,
		issue.ResolutionNotes, issue.ResolvedInCommit, issue.ResolvedInVersion,
		issue.HasRelatedCase, relatedCases, tags,
		issue.Logger, issue.Platform, issue.LastEventID, time.Now(),
		issue.ID, issue.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "issues")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ErrorMonitoringStore) ListProjectIssues(ctx context.Context, projectID string, filter shared.IssueFilter) ([]*observabilitydomain.Issue, error) {
	var conditions []string
	args := []interface{}{}

	conditions = append(conditions, "project_id = ?")
	args = append(args, projectID)

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, filter.Level)
	}
	if filter.AssignedTo != "" {
		conditions = append(conditions, "assigned_to = ?")
		args = append(args, filter.AssignedTo)
	}

	query := "SELECT * FROM ${SCHEMA_NAME}.issues WHERE " + strings.Join(conditions, " AND ") + " ORDER BY last_seen DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	var dbModels []models.Issue
	if err := s.selectContext(ctx, &dbModels, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "issues")
	}

	result := make([]*observabilitydomain.Issue, len(dbModels))
	for i, m := range dbModels {
		domainIssue, err := s.mapIssueToDomain(&m)
		if err != nil {
			return nil, fmt.Errorf("map issue %s: %w", m.ID, err)
		}
		result[i] = domainIssue
	}
	return result, nil
}

func (s *ErrorMonitoringStore) ListIssues(ctx context.Context, filters shared.IssueFilters) ([]*observabilitydomain.Issue, int, error) {
	var conditions []string
	args := []interface{}{}

	conditions = append(conditions, "1=1")

	if filters.WorkspaceID != "" {
		conditions = append(conditions, "workspace_id = ?")
		args = append(args, filters.WorkspaceID)
	}
	if filters.ProjectID != "" {
		conditions = append(conditions, "project_id = ?")
		args = append(args, filters.ProjectID)
	}
	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}
	if filters.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, filters.Level)
	}

	baseQuery := "FROM ${SCHEMA_NAME}.issues WHERE " + strings.Join(conditions, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := s.getContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "issues")
	}

	selectQuery := "SELECT * " + baseQuery + " ORDER BY last_seen DESC"

	if filters.Limit > 0 {
		selectQuery += " LIMIT ?"
		args = append(args, filters.Limit)
	}
	if filters.Offset > 0 {
		selectQuery += " OFFSET ?"
		args = append(args, filters.Offset)
	}

	var dbModels []models.Issue
	if err := s.selectContext(ctx, &dbModels, selectQuery, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "issues")
	}

	result := make([]*observabilitydomain.Issue, len(dbModels))
	for i, m := range dbModels {
		domainIssue, err := s.mapIssueToDomain(&m)
		if err != nil {
			return nil, 0, err
		}
		result[i] = domainIssue
	}
	return result, total, nil
}

func (s *ErrorMonitoringStore) ListAllIssues(ctx context.Context, filters shared.IssueFilters) ([]*observabilitydomain.Issue, int, error) {
	var conditions []string
	args := []interface{}{}

	conditions = append(conditions, "1=1")

	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}
	if filters.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, filters.Level)
	}

	baseQuery := "FROM ${SCHEMA_NAME}.issues WHERE " + strings.Join(conditions, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := s.getContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "issues")
	}

	selectQuery := "SELECT * " + baseQuery + " ORDER BY last_seen DESC"

	if filters.Limit > 0 {
		selectQuery += " LIMIT ?"
		args = append(args, filters.Limit)
	}
	if filters.Offset > 0 {
		selectQuery += " OFFSET ?"
		args = append(args, filters.Offset)
	}

	var dbModels []models.Issue
	if err := s.selectContext(ctx, &dbModels, selectQuery, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "issues")
	}

	result := make([]*observabilitydomain.Issue, len(dbModels))
	for i, m := range dbModels {
		domainIssue, err := s.mapIssueToDomain(&m)
		if err != nil {
			return nil, 0, err
		}
		result[i] = domainIssue
	}
	return result, total, nil
}

func (s *ErrorMonitoringStore) AtomicUpdateIssueStats(ctx context.Context, workspaceID, issueID string, lastEventID string, lastSeen time.Time, incrementUserCount bool) (*observabilitydomain.Issue, error) {
	now := time.Now()

	var query string
	if incrementUserCount {
		query = `
			UPDATE ${SCHEMA_NAME}.issues SET
				event_count = event_count + 1,
				user_count = user_count + 1,
				last_seen = ?,
				last_event_id = ?,
				updated_at = ?
			WHERE id = ? AND workspace_id = ?`
	} else {
		query = `
			UPDATE ${SCHEMA_NAME}.issues SET
				event_count = event_count + 1,
				last_seen = ?,
				last_event_id = ?,
				updated_at = ?
			WHERE id = ? AND workspace_id = ?`
	}

	result, err := s.execContext(ctx, query, lastSeen, lastEventID, now, issueID, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "issues")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return nil, shared.ErrNotFound
	}
	// If rowsErr != nil, ExecContext succeeded but we can't verify row count
	// Optimistically continue to return the updated issue

	return s.GetIssue(ctx, issueID)
}
