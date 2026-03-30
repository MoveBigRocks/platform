package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// CaseStore implements shared.CaseStore using SQLite/sqlx.
// Methods are organized across multiple files:
//   - case_store.go: struct, constructor, core case CRUD
//   - case_communication.go: communication operations
//   - case_attachment.go: attachment and KB linkage operations
//   - case_assignment.go: assignment history operations
type CaseStore struct {
	db *SqlxDB
}

func NewCaseStore(db *SqlxDB) *CaseStore {
	return &CaseStore{db: db}
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *CaseStore) mapToModel(c *servicedomain.Case) (*models.Case, error) {
	m := NewBulkMarshaler("case.mapToModel")
	m.Add("tags", c.Tags).
		Add("linked_issues", c.LinkedIssueIDs).
		Add("custom_fields", c.CustomFields)

	if err := m.Error(); err != nil {
		return nil, err
	}

	return &models.Case{
		ID:                        c.ID,
		WorkspaceID:               c.WorkspaceID,
		HumanID:                   c.HumanID,
		Subject:                   c.Subject,
		Description:               c.Description,
		Status:                    string(c.Status),
		Priority:                  string(c.Priority),
		Channel:                   string(c.Channel),
		Category:                  c.Category,
		QueueID:                   optionalString(c.QueueID),
		ContactID:                 optionalString(c.ContactID),
		PrimaryCatalogNodeID:      optionalString(c.PrimaryCatalogNodeID),
		OriginatingConversationID: optionalString(c.OriginatingConversationID),
		ContactEmail:              c.ContactEmail,
		ContactName:               c.ContactName,
		AssignedToID:              optionalString(c.AssignedToID),
		TeamID:                    optionalString(c.TeamID),
		Source:                    string(c.Source),
		SourceID:                  "",
		SourceLink:                "",
		Tags:                      m.Get("tags"),
		Resolution:                "",
		ResolutionNote:            "",
		ResponseDueAt:             c.ResponseDueAt,
		ResolutionDueAt:           c.ResolutionDueAt,
		FirstResponseAt:           c.FirstResponseAt,
		ResolvedAt:                c.ResolvedAt,
		ClosedAt:                  c.ClosedAt,
		ResponseTimeMinutes:       c.ResponseTimeMinutes,
		ResolutionTimeMinutes:     c.ResolutionTimeMinutes,
		ReopenCount:               c.ReopenCount,
		MessageCount:              c.MessageCount,
		LinkedIssueIDs:            m.Get("linked_issues"),
		RootCauseIssueID:          optionalString(c.RootCauseIssueID),
		IssueResolved:             c.IssueResolved,
		IssueResolvedAt:           c.IssueResolvedAt,
		ContactNotified:           c.ContactNotified,
		ContactNotifiedAt:         c.ContactNotifiedAt,
		NotificationTemplate:      c.NotificationTemplate,
		AutoCreated:               c.AutoCreated,
		IsSystemCase:              c.IsSystemCase,
		CustomFields:              m.Get("custom_fields"),
		CreatedBy:                 nil,
		CreatedAt:                 c.CreatedAt,
		UpdatedAt:                 c.UpdatedAt,
	}, nil
}

func (s *CaseStore) CreateCase(ctx context.Context, c *servicedomain.Case) error {
	dbCase, err := s.mapToModel(c)
	if err != nil {
		return fmt.Errorf("map case to model: %w", err)
	}

	normalizePersistedUUID(&c.ID)
	dbCase.ID = c.ID
	query := `INSERT INTO core_service.cases (
		id, workspace_id, human_id, subject, description, status, priority, channel,
		category, queue_id, contact_id, primary_catalog_node_id, originating_conversation_session_id,
		contact_email, contact_name, assigned_to_id, team_id,
		source, source_id, source_link, tags, resolution, resolution_note,
		response_due_at, resolution_due_at, first_response_at, resolved_at, closed_at,
		response_time_minutes, resolution_time_minutes, reopen_count, message_count,
		linked_issue_ids, root_cause_issue_id, issue_resolved, issue_resolved_at,
		contact_notified, contact_notified_at, notification_template,
		auto_created, is_system_case, custom_fields, created_by, created_at, updated_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?, ?, ?, ?
	) RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		dbCase.ID, dbCase.WorkspaceID, dbCase.HumanID, dbCase.Subject, dbCase.Description,
		dbCase.Status, dbCase.Priority, dbCase.Channel, dbCase.Category,
		nullableUUIDPtrValue(dbCase.QueueID), nullableUUIDPtrValue(dbCase.ContactID),
		nullableUUIDPtrValue(dbCase.PrimaryCatalogNodeID), nullableUUIDPtrValue(dbCase.OriginatingConversationID),
		dbCase.ContactEmail, dbCase.ContactName, nullableUUIDPtrValue(dbCase.AssignedToID), nullableUUIDPtrValue(dbCase.TeamID),
		dbCase.Source, dbCase.SourceID, dbCase.SourceLink, dbCase.Tags, dbCase.Resolution,
		dbCase.ResolutionNote, dbCase.ResponseDueAt, dbCase.ResolutionDueAt, dbCase.FirstResponseAt,
		dbCase.ResolvedAt, dbCase.ClosedAt, dbCase.ResponseTimeMinutes, dbCase.ResolutionTimeMinutes,
		dbCase.ReopenCount, dbCase.MessageCount, dbCase.LinkedIssueIDs, nullableUUIDPtrValue(dbCase.RootCauseIssueID),
		dbCase.IssueResolved, dbCase.IssueResolvedAt, dbCase.ContactNotified, dbCase.ContactNotifiedAt,
		dbCase.NotificationTemplate, dbCase.AutoCreated, dbCase.IsSystemCase, dbCase.CustomFields,
		nullableLegacyUUIDPtrValue(dbCase.CreatedBy), dbCase.CreatedAt, dbCase.UpdatedAt,
	).Scan(&c.ID)
	return TranslateSqlxError(err, "cases")
}

func (s *CaseStore) GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error) {
	var dbCase models.Case
	query := `SELECT * FROM core_service.cases WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbCase, query, caseID)
	if err != nil {
		return nil, TranslateSqlxError(err, "cases")
	}
	return s.mapToDomain(&dbCase), nil
}

// GetCaseInWorkspace retrieves a case only if it belongs to the specified workspace.
// Returns ErrNotFound if case doesn't exist OR belongs to different workspace (defense-in-depth).
func (s *CaseStore) GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*servicedomain.Case, error) {
	var dbCase models.Case
	query := `SELECT * FROM core_service.cases WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbCase, query, caseID, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "cases")
	}
	return s.mapToDomain(&dbCase), nil
}

func (s *CaseStore) GetCaseByHumanID(ctx context.Context, humanID string) (*servicedomain.Case, error) {
	var dbCase models.Case
	query := `SELECT * FROM core_service.cases WHERE human_id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbCase, query, humanID)
	if err != nil {
		return nil, TranslateSqlxError(err, "cases")
	}
	return s.mapToDomain(&dbCase), nil
}

func (s *CaseStore) GetCasesByIDs(ctx context.Context, caseIDs []string) ([]*servicedomain.Case, error) {
	if len(caseIDs) == 0 {
		return []*servicedomain.Case{}, nil
	}

	query, args, err := buildInQuery(`SELECT * FROM core_service.cases WHERE id IN (?) AND deleted_at IS NULL`, caseIDs)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}

	var dbCases []models.Case
	if err := s.db.Get(ctx).SelectContext(ctx, &dbCases, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "cases")
	}

	result := make([]*servicedomain.Case, len(dbCases))
	for i, c := range dbCases {
		result[i] = s.mapToDomain(&c)
	}
	return result, nil
}

// GetCaseByIssueAndContact finds an existing case for a specific issue and contact.
// Used for idempotency - prevents duplicate case creation on event retry.
func (s *CaseStore) GetCaseByIssueAndContact(ctx context.Context, workspaceID, issueID, contactID string) (*servicedomain.Case, error) {
	var dbCase models.Case
	query := `SELECT * FROM core_service.cases WHERE workspace_id = ? AND root_cause_issue_id = ? AND contact_id = ? AND deleted_at IS NULL LIMIT 1`
	err := s.db.Get(ctx).GetContext(ctx, &dbCase, query, workspaceID, issueID, contactID)
	if err != nil {
		return nil, TranslateSqlxError(err, "cases")
	}
	return s.mapToDomain(&dbCase), nil
}

func (s *CaseStore) UpdateCase(ctx context.Context, c *servicedomain.Case) error {
	dbCase, err := s.mapToModel(c)
	if err != nil {
		return fmt.Errorf("map case to model: %w", err)
	}

	query := `UPDATE core_service.cases SET
		workspace_id = ?, human_id = ?, subject = ?, description = ?,
		status = ?, priority = ?, channel = ?, category = ?, queue_id = ?, contact_id = ?,
		primary_catalog_node_id = ?, originating_conversation_session_id = ?,
		contact_email = ?, contact_name = ?, assigned_to_id = ?, team_id = ?,
		source = ?, source_id = ?, source_link = ?, tags = ?, resolution = ?,
		resolution_note = ?, response_due_at = ?, resolution_due_at = ?,
		first_response_at = ?, resolved_at = ?, closed_at = ?,
		response_time_minutes = ?, resolution_time_minutes = ?,
		reopen_count = ?, message_count = ?, linked_issue_ids = ?,
		root_cause_issue_id = ?, issue_resolved = ?, issue_resolved_at = ?,
		contact_notified = ?, contact_notified_at = ?, notification_template = ?,
		auto_created = ?, is_system_case = ?, custom_fields = ?,
		created_by = ?, updated_at = ?
	WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		dbCase.WorkspaceID, dbCase.HumanID, dbCase.Subject, dbCase.Description,
		dbCase.Status, dbCase.Priority, dbCase.Channel, dbCase.Category,
		nullableUUIDPtrValue(dbCase.QueueID), nullableUUIDPtrValue(dbCase.ContactID),
		nullableUUIDPtrValue(dbCase.PrimaryCatalogNodeID), nullableUUIDPtrValue(dbCase.OriginatingConversationID),
		dbCase.ContactEmail, dbCase.ContactName, nullableUUIDPtrValue(dbCase.AssignedToID), nullableUUIDPtrValue(dbCase.TeamID),
		dbCase.Source, dbCase.SourceID, dbCase.SourceLink, dbCase.Tags, dbCase.Resolution,
		dbCase.ResolutionNote, dbCase.ResponseDueAt, dbCase.ResolutionDueAt, dbCase.FirstResponseAt,
		dbCase.ResolvedAt, dbCase.ClosedAt, dbCase.ResponseTimeMinutes, dbCase.ResolutionTimeMinutes,
		dbCase.ReopenCount, dbCase.MessageCount, dbCase.LinkedIssueIDs, nullableUUIDPtrValue(dbCase.RootCauseIssueID),
		dbCase.IssueResolved, dbCase.IssueResolvedAt, dbCase.ContactNotified, dbCase.ContactNotifiedAt,
		dbCase.NotificationTemplate, dbCase.AutoCreated, dbCase.IsSystemCase, dbCase.CustomFields,
		nullableLegacyUUIDPtrValue(dbCase.CreatedBy), time.Now(), dbCase.ID, dbCase.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "cases")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *CaseStore) mapToDomain(c *models.Case) *servicedomain.Case {
	var tags []string
	if err := json.Unmarshal([]byte(c.Tags), &tags); err != nil {
		tags = []string{}
	}
	var linkedIssues []string
	if err := json.Unmarshal([]byte(c.LinkedIssueIDs), &linkedIssues); err != nil {
		linkedIssues = []string{}
	}

	customFields := shareddomain.NewTypedCustomFields()
	if c.CustomFields != "" {
		unmarshalJSONField(c.CustomFields, &customFields, "cases", "custom_fields")
	}

	return &servicedomain.Case{
		CaseIdentity: servicedomain.CaseIdentity{
			ID:          c.ID,
			WorkspaceID: c.WorkspaceID,
			HumanID:     c.HumanID,
		},
		Subject:              c.Subject,
		Description:          c.Description,
		Status:               servicedomain.CaseStatus(c.Status),
		Priority:             servicedomain.CasePriority(c.Priority),
		Channel:              servicedomain.CaseChannel(c.Channel),
		Category:             c.Category,
		QueueID:              valueOrEmpty(c.QueueID),
		PrimaryCatalogNodeID: valueOrEmpty(c.PrimaryCatalogNodeID),
		Tags:                 tags,

		CaseContact: servicedomain.CaseContact{
			ContactID:    derefStringPtr(c.ContactID),
			ContactEmail: c.ContactEmail,
			ContactName:  c.ContactName,
		},
		CaseAssignment: servicedomain.CaseAssignment{
			AssignedToID: derefStringPtr(c.AssignedToID),
			TeamID:       derefStringPtr(c.TeamID),
		},
		CaseSourceInfo: servicedomain.CaseSourceInfo{
			Source:       shareddomain.SourceType(c.Source),
			AutoCreated:  c.AutoCreated,
			IsSystemCase: c.IsSystemCase,
		},
		CaseSLA: servicedomain.CaseSLA{
			ResponseDueAt:         c.ResponseDueAt,
			ResolutionDueAt:       c.ResolutionDueAt,
			FirstResponseAt:       c.FirstResponseAt,
			ResolvedAt:            c.ResolvedAt,
			ClosedAt:              c.ClosedAt,
			ResponseTimeMinutes:   c.ResponseTimeMinutes,
			ResolutionTimeMinutes: c.ResolutionTimeMinutes,
		},
		CaseTimestamps: servicedomain.CaseTimestamps{
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		},
		CaseMetrics: servicedomain.CaseMetrics{
			ReopenCount:  c.ReopenCount,
			MessageCount: c.MessageCount,
		},
		CaseRelationships: servicedomain.CaseRelationships{},
		CaseIssueTracking: servicedomain.CaseIssueTracking{
			LinkedIssueIDs:       linkedIssues,
			RootCauseIssueID:     derefStringPtr(c.RootCauseIssueID),
			IssueResolved:        c.IssueResolved,
			IssueResolvedAt:      c.IssueResolvedAt,
			ContactNotified:      c.ContactNotified,
			ContactNotifiedAt:    c.ContactNotifiedAt,
			NotificationTemplate: c.NotificationTemplate,
		},
		OriginatingConversationID: valueOrEmpty(c.OriginatingConversationID),
		CustomFields:              customFields,
	}
}

func (s *CaseStore) ListWorkspaceCases(ctx context.Context, workspaceID string, filter shared.CaseFilter) ([]*servicedomain.Case, error) {
	args := []interface{}{workspaceID}
	conditions := []string{"workspace_id = ?", "deleted_at IS NULL"}

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.StatusNot != "" {
		conditions = append(conditions, "status != ?")
		args = append(args, filter.StatusNot)
	}
	if filter.Priority != "" {
		conditions = append(conditions, "priority = ?")
		args = append(args, filter.Priority)
	}
	if filter.QueueID != "" {
		conditions = append(conditions, "queue_id = ?")
		args = append(args, filter.QueueID)
	}
	if filter.AssignedToID != "" {
		conditions = append(conditions, "assigned_to_id = ?")
		args = append(args, filter.AssignedToID)
	}
	if filter.TeamID != "" {
		conditions = append(conditions, "team_id = ?")
		args = append(args, filter.TeamID)
	}
	if filter.ContactID != "" {
		conditions = append(conditions, "contact_id = ?")
		args = append(args, filter.ContactID)
	}
	if filter.ResolvedBefore != nil {
		conditions = append(conditions, "resolved_at < ?")
		args = append(args, filter.ResolvedBefore)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`SELECT * FROM core_service.cases WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		strings.Join(conditions, " AND "), limit, offset)

	var dbCases []models.Case
	err := s.db.Get(ctx).SelectContext(ctx, &dbCases, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workspace cases: %w", err)
	}

	cases := make([]*servicedomain.Case, len(dbCases))
	for i, c := range dbCases {
		cases[i] = s.mapToDomain(&c)
	}
	return cases, nil
}

func (s *CaseStore) ListWorkspaceCasesFast(ctx context.Context, workspaceID string, filter shared.CaseFilter) ([]*servicedomain.Case, error) {
	return s.ListWorkspaceCases(ctx, workspaceID, filter)
}

func (s *CaseStore) ListCases(ctx context.Context, filters shared.CaseFilters) ([]*servicedomain.Case, int, error) {
	args := []interface{}{}
	conditions := []string{"deleted_at IS NULL"}

	if filters.WorkspaceID != "" {
		conditions = append(conditions, "workspace_id = ?")
		args = append(args, filters.WorkspaceID)
	}
	if filters.QueueID != "" {
		conditions = append(conditions, "queue_id = ?")
		args = append(args, filters.QueueID)
	}
	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM core_service.cases WHERE %s`, whereClause)
	var total int
	err := s.db.Get(ctx).GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cases count: %w", err)
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`SELECT * FROM core_service.cases WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		whereClause, limit, filters.Offset)

	var dbCases []models.Case
	err = s.db.Get(ctx).SelectContext(ctx, &dbCases, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cases: %w", err)
	}

	cases := make([]*servicedomain.Case, len(dbCases))
	for i, c := range dbCases {
		cases[i] = s.mapToDomain(&c)
	}
	return cases, total, nil
}

func (s *CaseStore) DeleteCase(ctx context.Context, workspaceID, caseID string) error {
	query := `UPDATE core_service.cases SET deleted_at = ? WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), caseID, workspaceID)
	if err != nil {
		return fmt.Errorf("delete case %s: %w", caseID, err)
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *CaseStore) MarkCaseNotified(ctx context.Context, workspaceID, caseID string) error {
	now := time.Now()
	query := `UPDATE core_service.cases SET contact_notified = TRUE, contact_notified_at = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, now, now, caseID, workspaceID)
	if err != nil {
		return fmt.Errorf("mark case notified %s: %w", caseID, err)
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *CaseStore) ListResolvedCasesForAutoClose(ctx context.Context, resolvedBefore time.Time, limit int) ([]*servicedomain.Case, error) {
	query := `SELECT * FROM core_service.cases WHERE status = ? AND resolved_at < ? AND deleted_at IS NULL LIMIT ?`

	var dbCases []models.Case
	err := s.db.Get(ctx).SelectContext(ctx, &dbCases, query, servicedomain.CaseStatusResolved, resolvedBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("list resolved cases for auto close: %w", err)
	}

	cases := make([]*servicedomain.Case, len(dbCases))
	for i, c := range dbCases {
		cases[i] = s.mapToDomain(&c)
	}
	return cases, nil
}

func (s *CaseStore) ListCasesByMessageID(ctx context.Context, workspaceID, messageID string) ([]*servicedomain.Case, error) {
	query := `SELECT c.* FROM core_service.cases c
		JOIN core_service.communications comm ON c.id = comm.case_id
		WHERE c.workspace_id = ? AND comm.message_id = ? AND c.deleted_at IS NULL`

	var dbCases []models.Case
	err := s.db.Get(ctx).SelectContext(ctx, &dbCases, query, workspaceID, messageID)
	if err != nil {
		return nil, TranslateSqlxError(err, "cases")
	}

	cases := make([]*servicedomain.Case, len(dbCases))
	for i, c := range dbCases {
		cases[i] = s.mapToDomain(&c)
	}
	return cases, nil
}

func (s *CaseStore) ListCasesBySubject(ctx context.Context, workspaceID, subject string) ([]*servicedomain.Case, error) {
	query := `SELECT * FROM core_service.cases WHERE workspace_id = ? AND subject = ? AND deleted_at IS NULL`

	var dbCases []models.Case
	err := s.db.Get(ctx).SelectContext(ctx, &dbCases, query, workspaceID, subject)
	if err != nil {
		return nil, fmt.Errorf("list cases by subject: %w", err)
	}

	cases := make([]*servicedomain.Case, len(dbCases))
	for i, c := range dbCases {
		cases[i] = s.mapToDomain(&c)
	}
	return cases, nil
}

func (s *CaseStore) GetCaseCount(ctx context.Context, workspaceID string, filter shared.CaseFilter) (int, error) {
	query := `SELECT COUNT(*) FROM core_service.cases WHERE workspace_id = ? AND deleted_at IS NULL`
	var count int
	err := s.db.Get(ctx).GetContext(ctx, &count, query, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("get case count: %w", err)
	}
	return count, nil
}
