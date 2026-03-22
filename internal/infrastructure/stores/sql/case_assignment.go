package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

func (s *CaseStore) CreateCaseAssignmentHistory(ctx context.Context, history *shareddomain.CaseAssignmentHistory) error {
	requiredSkillsJSON, err := marshalJSONString(history.RequiredSkills, "required_skills")
	if err != nil {
		return fmt.Errorf("CreateCaseAssignmentHistory: %w", err)
	}
	matchedSkillsJSON, err := marshalJSONString(history.MatchedSkills, "matched_skills")
	if err != nil {
		return fmt.Errorf("CreateCaseAssignmentHistory: %w", err)
	}
	tagsJSON, err := marshalJSONString(history.Tags, "tags")
	if err != nil {
		return fmt.Errorf("CreateCaseAssignmentHistory: %w", err)
	}
	customFieldsJSON, err := marshalJSONString(history.CustomFields, "custom_fields")
	if err != nil {
		return fmt.Errorf("CreateCaseAssignmentHistory: %w", err)
	}
	autoAssignConfigJSON, err := marshalJSONString(history.AutoAssignmentConfig, "auto_assignment_config")
	if err != nil {
		return fmt.Errorf("CreateCaseAssignmentHistory: %w", err)
	}
	alternativeCandidatesJSON, err := marshalJSONString(history.AlternativeCandidates, "alternative_candidates")
	if err != nil {
		return fmt.Errorf("CreateCaseAssignmentHistory: %w", err)
	}
	caseCustomFieldsJSON, err := marshalJSONString(history.CaseCustomFields, "case_custom_fields")
	if err != nil {
		return fmt.Errorf("CreateCaseAssignmentHistory: %w", err)
	}

	now := time.Now()
	normalizePersistedUUID(&history.ID)
	query := `INSERT INTO core_service.case_assignment_history (
		id, workspace_id, case_id, assignment_type, assigned_to_user_id, assigned_to_team_id,
		assigned_user_name, assigned_team_name, previous_user_id, previous_team_id,
		previous_user_name, previous_team_name, reason, status, assigned_at, accepted_at,
		completed_at, duration, assigned_by_id, assigned_by_name, assigned_by_type,
		rule_id, workflow_id, priority, is_urgent, sla_deadline, workload_before, workload_after,
		required_skills, matched_skills, skill_match_score, assignee_available, assignee_timezone,
		assignment_during_hours, response_time, resolution_time, customer_satisfaction,
		was_escalated, escalated_at, escalated_to_user_id, escalated_to_team_id, escalation_reason,
		was_transferred, transferred_at, transferred_to_user_id, transferred_to_team_id, transfer_reason,
		was_accepted, accepted_by_id, rejected_at, rejection_reason, auto_assignment_config,
		assignment_score, alternative_candidates, notification_sent, notification_sent_at,
		notification_method, notification_viewed, notification_viewed_at, case_status, case_priority,
		case_subject, case_created_at, case_custom_fields, comments, tags, custom_fields,
		created_at, updated_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	) RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		history.ID, history.WorkspaceID, history.CaseID, string(history.AssignmentType),
		nullableUUIDValue(history.AssignedToUserID), nullableUUIDValue(history.AssignedToTeamID), history.AssignedUserName, history.AssignedTeamName,
		nullableUUIDValue(history.PreviousUserID), nullableUUIDValue(history.PreviousTeamID), history.PreviousUserName, history.PreviousTeamName,
		string(history.Reason), string(history.Status), history.AssignedAt, history.AcceptedAt,
		history.CompletedAt, history.Duration, nullableLegacyUUIDValue(history.AssignedByID), history.AssignedByName,
		history.AssignedByType, nullableUUIDValue(history.RuleID), nullableUUIDValue(history.WorkflowID), history.Priority, history.IsUrgent,
		history.SLADeadline, history.WorkloadBefore, history.WorkloadAfter, requiredSkillsJSON,
		matchedSkillsJSON, history.SkillMatchScore, history.AssigneeAvailable, history.AssigneeTimezone,
		history.AssignmentDuringHours, history.ResponseTime, history.ResolutionTime, history.CustomerSatisfaction,
		history.WasEscalated, history.EscalatedAt, nullableUUIDValue(history.EscalatedToUserID), nullableUUIDValue(history.EscalatedToTeamID),
		history.EscalationReason, history.WasTransferred, history.TransferredAt, nullableUUIDValue(history.TransferredToUserID),
		nullableUUIDValue(history.TransferredToTeamID), history.TransferReason, history.WasAccepted, nullableLegacyUUIDValue(history.AcceptedByID),
		history.RejectedAt, history.RejectionReason, autoAssignConfigJSON, history.AssignmentScore,
		alternativeCandidatesJSON, history.NotificationSent, history.NotificationSentAt,
		history.NotificationMethod, history.NotificationViewed, history.NotificationViewedAt,
		history.CaseStatus, history.CasePriority, history.CaseSubject, history.CaseCreatedAt,
		caseCustomFieldsJSON, history.Comments, tagsJSON, customFieldsJSON, now, now,
	).Scan(&history.ID)
	return TranslateSqlxError(err, "case_assignment_history")
}

func (s *CaseStore) GetCaseAssignmentHistory(ctx context.Context, historyID string) (*shareddomain.CaseAssignmentHistory, error) {
	var dbHistory models.CaseAssignmentHistory
	query := `SELECT * FROM core_service.case_assignment_history WHERE id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbHistory, query, historyID)
	if err != nil {
		return nil, TranslateSqlxError(err, "case_assignment_history")
	}
	return s.mapAssignmentHistoryToDomain(&dbHistory), nil
}

func (s *CaseStore) ListCaseAssignmentHistoryByCase(ctx context.Context, caseID string) ([]*shareddomain.CaseAssignmentHistory, error) {
	var dbHistories []models.CaseAssignmentHistory
	query := `SELECT * FROM core_service.case_assignment_history WHERE case_id = ? ORDER BY assigned_at DESC`
	err := s.db.Get(ctx).SelectContext(ctx, &dbHistories, query, caseID)
	if err != nil {
		return nil, TranslateSqlxError(err, "case_assignment_history")
	}

	histories := make([]*shareddomain.CaseAssignmentHistory, len(dbHistories))
	for i, h := range dbHistories {
		histories[i] = s.mapAssignmentHistoryToDomain(&h)
	}
	return histories, nil
}

func (s *CaseStore) UpdateCaseAssignmentHistory(ctx context.Context, history *shareddomain.CaseAssignmentHistory) error {
	requiredSkillsJSON, err := marshalJSONString(history.RequiredSkills, "required_skills")
	if err != nil {
		return fmt.Errorf("UpdateCaseAssignmentHistory: %w", err)
	}
	matchedSkillsJSON, err := marshalJSONString(history.MatchedSkills, "matched_skills")
	if err != nil {
		return fmt.Errorf("UpdateCaseAssignmentHistory: %w", err)
	}
	tagsJSON, err := marshalJSONString(history.Tags, "tags")
	if err != nil {
		return fmt.Errorf("UpdateCaseAssignmentHistory: %w", err)
	}
	customFieldsJSON, err := marshalJSONString(history.CustomFields, "custom_fields")
	if err != nil {
		return fmt.Errorf("UpdateCaseAssignmentHistory: %w", err)
	}

	query := `UPDATE core_service.case_assignment_history SET
		status = ?, accepted_at = ?, completed_at = ?, duration = ?,
		response_time = ?, resolution_time = ?, customer_satisfaction = ?,
		was_escalated = ?, escalated_at = ?, escalated_to_user_id = ?,
		escalated_to_team_id = ?, escalation_reason = ?, was_transferred = ?,
		transferred_at = ?, transferred_to_user_id = ?, transferred_to_team_id = ?,
		transfer_reason = ?, was_accepted = ?, accepted_by_id = ?,
		rejected_at = ?, rejection_reason = ?, notification_sent = ?,
		notification_sent_at = ?, notification_viewed = ?, notification_viewed_at = ?,
		comments = ?, tags = ?, custom_fields = ?, required_skills = ?,
		matched_skills = ?, updated_at = ?
	WHERE id = ? AND workspace_id = ?`

	_, err = s.db.Get(ctx).ExecContext(ctx, query,
		string(history.Status), history.AcceptedAt, history.CompletedAt, history.Duration,
		history.ResponseTime, history.ResolutionTime, history.CustomerSatisfaction,
		history.WasEscalated, history.EscalatedAt, nullableUUIDValue(history.EscalatedToUserID),
		nullableUUIDValue(history.EscalatedToTeamID), history.EscalationReason, history.WasTransferred,
		history.TransferredAt, nullableUUIDValue(history.TransferredToUserID), nullableUUIDValue(history.TransferredToTeamID),
		history.TransferReason, history.WasAccepted, nullableLegacyUUIDValue(history.AcceptedByID),
		history.RejectedAt, history.RejectionReason, history.NotificationSent,
		history.NotificationSentAt, history.NotificationViewed, history.NotificationViewedAt,
		history.Comments, tagsJSON, customFieldsJSON, requiredSkillsJSON,
		matchedSkillsJSON, time.Now(),
		history.ID, history.WorkspaceID,
	)
	return TranslateSqlxError(err, "case_assignment_history")
}

func (s *CaseStore) mapAssignmentHistoryToDomain(h *models.CaseAssignmentHistory) *shareddomain.CaseAssignmentHistory {
	var requiredSkills, matchedSkills, tags []string
	customFields := shareddomain.NewMetadata()
	autoAssignConfig := shareddomain.NewMetadata()
	caseCustomFields := shareddomain.NewMetadata()
	var alternativeCandidates []shareddomain.AssignmentCandidate

	unmarshalJSONField(h.RequiredSkills, &requiredSkills, "case_assignment_history", "required_skills")
	unmarshalJSONField(h.MatchedSkills, &matchedSkills, "case_assignment_history", "matched_skills")
	unmarshalJSONField(h.Tags, &tags, "case_assignment_history", "tags")
	unmarshalJSONField(h.CustomFields, &customFields, "case_assignment_history", "custom_fields")
	unmarshalJSONField(h.AutoAssignmentConfig, &autoAssignConfig, "case_assignment_history", "auto_assignment_config")
	unmarshalJSONField(h.CaseCustomFields, &caseCustomFields, "case_assignment_history", "case_custom_fields")
	unmarshalJSONField(h.AlternativeCandidates, &alternativeCandidates, "case_assignment_history", "alternative_candidates")

	return &shareddomain.CaseAssignmentHistory{
		ID:                    h.ID,
		WorkspaceID:           h.WorkspaceID,
		CaseID:                h.CaseID,
		AssignmentType:        shareddomain.AssignmentType(h.AssignmentType),
		AssignedToUserID:      derefStringPtr(h.AssignedToUserID),
		AssignedToTeamID:      derefStringPtr(h.AssignedToTeamID),
		AssignedUserName:      h.AssignedUserName,
		AssignedTeamName:      h.AssignedTeamName,
		PreviousUserID:        derefStringPtr(h.PreviousUserID),
		PreviousTeamID:        derefStringPtr(h.PreviousTeamID),
		PreviousUserName:      h.PreviousUserName,
		PreviousTeamName:      h.PreviousTeamName,
		Reason:                shareddomain.AssignmentReason(h.Reason),
		Status:                shareddomain.AssignmentStatus(h.Status),
		AssignedAt:            h.AssignedAt,
		AcceptedAt:            h.AcceptedAt,
		CompletedAt:           h.CompletedAt,
		Duration:              h.Duration,
		AssignedByID:          derefStringPtr(h.AssignedByID),
		AssignedByName:        h.AssignedByName,
		AssignedByType:        h.AssignedByType,
		RuleID:                derefStringPtr(h.RuleID),
		WorkflowID:            derefStringPtr(h.WorkflowID),
		Priority:              h.Priority,
		IsUrgent:              h.IsUrgent,
		SLADeadline:           h.SLADeadline,
		WorkloadBefore:        h.WorkloadBefore,
		WorkloadAfter:         h.WorkloadAfter,
		RequiredSkills:        requiredSkills,
		MatchedSkills:         matchedSkills,
		SkillMatchScore:       h.SkillMatchScore,
		AssigneeAvailable:     h.AssigneeAvailable,
		AssigneeTimezone:      h.AssigneeTimezone,
		AssignmentDuringHours: h.AssignmentDuringHours,
		ResponseTime:          h.ResponseTime,
		ResolutionTime:        h.ResolutionTime,
		CustomerSatisfaction:  h.CustomerSatisfaction,
		WasEscalated:          h.WasEscalated,
		EscalatedAt:           h.EscalatedAt,
		EscalatedToUserID:     derefStringPtr(h.EscalatedToUserID),
		EscalatedToTeamID:     derefStringPtr(h.EscalatedToTeamID),
		EscalationReason:      h.EscalationReason,
		WasTransferred:        h.WasTransferred,
		TransferredAt:         h.TransferredAt,
		TransferredToUserID:   derefStringPtr(h.TransferredToUserID),
		TransferredToTeamID:   derefStringPtr(h.TransferredToTeamID),
		TransferReason:        h.TransferReason,
		WasAccepted:           h.WasAccepted,
		AcceptedByID:          derefStringPtr(h.AcceptedByID),
		RejectedAt:            h.RejectedAt,
		RejectionReason:       h.RejectionReason,
		AutoAssignmentConfig:  autoAssignConfig,
		AssignmentScore:       h.AssignmentScore,
		AlternativeCandidates: alternativeCandidates,
		NotificationSent:      h.NotificationSent,
		NotificationSentAt:    h.NotificationSentAt,
		NotificationMethod:    h.NotificationMethod,
		NotificationViewed:    h.NotificationViewed,
		NotificationViewedAt:  h.NotificationViewedAt,
		CaseStatus:            h.CaseStatus,
		CasePriority:          h.CasePriority,
		CaseSubject:           h.CaseSubject,
		CaseCreatedAt:         h.CaseCreatedAt,
		CaseCustomFields:      caseCustomFields,
		Comments:              h.Comments,
		Tags:                  tags,
		CustomFields:          customFields,
	}
}
