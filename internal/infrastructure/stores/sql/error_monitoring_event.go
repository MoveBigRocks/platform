package sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
)

// =============================================================================
// Error Event Operations
// =============================================================================

func (s *ErrorMonitoringStore) CreateErrorEvent(ctx context.Context, event *observabilitydomain.ErrorEvent) error {
	workspaceID, installID, err := s.lookupProjectScope(ctx, event.ProjectID)
	if err != nil {
		return err
	}
	normalizePersistedUUID(&event.ID)
	exception, err := marshalJSONString(event.Exception, "exception")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	stacktrace, err := marshalJSONString(event.Stacktrace, "stacktrace")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	user, err := marshalJSONString(event.User, "user")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	request, err := marshalJSONString(event.Request, "request")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	tags, err := marshalJSONString(event.Tags, "tags")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	extra, err := marshalJSONString(event.Extra, "extra")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	contexts, err := marshalJSONString(event.Contexts, "contexts")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	breadcrumbs, err := marshalJSONString(event.Breadcrumbs, "breadcrumbs")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}
	fingerprint, err := marshalJSONString(event.Fingerprint, "fingerprint")
	if err != nil {
		return fmt.Errorf("create error event: %w", err)
	}

	query := `
		INSERT INTO ${SCHEMA_NAME}.error_events (
			id, workspace_id, extension_install_id, event_id, project_id, issue_id, timestamp, received, message, level,
			logger, platform, environment, release, dist, exception, stacktrace,
			user, request, tags, extra, contexts, breadcrumbs, fingerprint,
			data_url, size, processed_at, grouped_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?
		)
		RETURNING id`

	err = s.queryRowxContext(ctx, query,
		event.ID, workspaceID, installID, event.EventID, event.ProjectID, nullableUUIDValue(event.IssueID), event.Timestamp, event.Received,
		event.Message, event.Level, event.Logger, event.Platform, event.Environment, event.Release,
		event.Dist, exception, stacktrace, user, request, tags, extra, contexts, breadcrumbs,
		fingerprint, event.DataURL, event.Size, event.ProcessedAt, event.GroupedAt,
	).Scan(&event.ID)
	return TranslateSqlxError(err, "error_events")
}

func (s *ErrorMonitoringStore) GetErrorEvent(ctx context.Context, eventID string) (*observabilitydomain.ErrorEvent, error) {
	var model models.ErrorEvent
	query := `SELECT * FROM ${SCHEMA_NAME}.error_events WHERE id = ? OR event_id = ?`
	err := s.getContext(ctx, &model, query, eventID, eventID)
	if err != nil {
		return nil, TranslateSqlxError(err, "error_events")
	}
	return s.mapEventToDomain(&model)
}

func (s *ErrorMonitoringStore) GetIssueEvents(ctx context.Context, issueID string, limit int) ([]*observabilitydomain.ErrorEvent, error) {
	var dbModels []models.ErrorEvent
	query := `SELECT * FROM ${SCHEMA_NAME}.error_events WHERE issue_id = ? ORDER BY timestamp DESC LIMIT ?`
	if err := s.selectContext(ctx, &dbModels, query, issueID, limit); err != nil {
		return nil, TranslateSqlxError(err, "error_events")
	}

	result := make([]*observabilitydomain.ErrorEvent, len(dbModels))
	for i, m := range dbModels {
		domainEvent, err := s.mapEventToDomain(&m)
		if err != nil {
			return nil, fmt.Errorf("map error event %s: %w", m.ID, err)
		}
		result[i] = domainEvent
	}
	return result, nil
}

func (s *ErrorMonitoringStore) ListProjectEvents(ctx context.Context, projectID string, filter shared.EventFilter) ([]*observabilitydomain.ErrorEvent, error) {
	var conditions []string
	args := []interface{}{}

	conditions = append(conditions, "project_id = ?")
	args = append(args, projectID)

	if filter.IssueID != "" {
		conditions = append(conditions, "issue_id = ?")
		args = append(args, filter.IssueID)
	}
	if filter.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, filter.Level)
	}
	if filter.Environment != "" {
		conditions = append(conditions, "environment = ?")
		args = append(args, filter.Environment)
	}
	if filter.Release != "" {
		conditions = append(conditions, "release = ?")
		args = append(args, filter.Release)
	}

	query := "SELECT * FROM ${SCHEMA_NAME}.error_events WHERE " + strings.Join(conditions, " AND ") + " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	var dbModels []models.ErrorEvent
	if err := s.selectContext(ctx, &dbModels, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "error_events")
	}

	result := make([]*observabilitydomain.ErrorEvent, len(dbModels))
	for i, m := range dbModels {
		domainEvent, err := s.mapEventToDomain(&m)
		if err != nil {
			return nil, fmt.Errorf("map error event %s: %w", m.ID, err)
		}
		result[i] = domainEvent
	}
	return result, nil
}

// UpdateEventIssueID updates the issue_id for an existing error event.
// This is used to link events to issues after they've been grouped.
func (s *ErrorMonitoringStore) UpdateEventIssueID(ctx context.Context, workspaceID, eventID, issueID string) error {
	// Match on both id and event_id columns since either could be used as the identifier
	query := `UPDATE ${SCHEMA_NAME}.error_events SET issue_id = ? WHERE workspace_id = ? AND (id = ? OR event_id = ?)`
	result, err := s.execContext(ctx, query, issueID, workspaceID, eventID, eventID)
	if err != nil {
		return TranslateSqlxError(err, "error_events")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}
