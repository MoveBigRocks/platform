package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

func (s *ErrorMonitoringStore) GetErrorEventsByEmail(ctx context.Context, email string, since time.Time) ([]*observabilitydomain.ErrorEvent, error) {
	var dbModels []models.ErrorEvent
	query := `SELECT * FROM ${SCHEMA_NAME}.error_events WHERE "user" ->> 'email' = ? AND timestamp >= ?`
	if err := s.selectContext(ctx, &dbModels, query, email, since); err != nil {
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

func (s *ErrorMonitoringStore) FindCaseForContactAndIssue(ctx context.Context, contactID, issueID string) (*servicedomain.Case, error) {
	var dbCase models.Case
	query := `SELECT * FROM core_service.cases WHERE contact_id = ? AND root_cause_issue_id = ? AND deleted_at IS NULL LIMIT 1`
	if err := s.db.Get(ctx).GetContext(ctx, &dbCase, query, contactID, issueID); err != nil {
		return nil, TranslateSqlxError(err, "cases")
	}

	caseStore := CaseStore{db: s.db}
	return caseStore.mapToDomain(&dbCase), nil
}

func (s *ErrorMonitoringStore) GetUnresolvedIssuesWithCases(ctx context.Context, workspaceID string) ([]*observabilitydomain.Issue, error) {
	var dbModels []models.Issue
	query := `SELECT * FROM ${SCHEMA_NAME}.issues WHERE workspace_id = ? AND status = ? AND has_related_case = TRUE`
	if err := s.selectContext(ctx, &dbModels, query, workspaceID, observabilitydomain.IssueStatusUnresolved); err != nil {
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
