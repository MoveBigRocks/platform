package sql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

type ConversationStore struct {
	db *SqlxDB
}

func NewConversationStore(db *SqlxDB) *ConversationStore {
	return &ConversationStore{db: db}
}

func (s *ConversationStore) CreateConversationSession(ctx context.Context, session *servicedomain.ConversationSession) error {
	normalizePersistedUUID(&session.ID)

	metadata, err := marshalJSONString(session.Metadata, "metadata")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.conversation_sessions (
			id, workspace_id, channel, status, primary_contact_id, primary_catalog_node_id,
			active_form_spec_id, active_form_submission_id, linked_case_id, handling_team_id,
			assigned_operator_user_id, delegated_runtime_connector_id, title, language_code,
			source_ref, external_session_key, opened_at, last_activity_at, closed_at,
			metadata_json, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		session.ID,
		session.WorkspaceID,
		string(session.Channel),
		string(session.Status),
		nullableUUIDValue(session.PrimaryContactID),
		nullableUUIDValue(session.PrimaryCatalogNodeID),
		nullableUUIDValue(session.ActiveFormSpecID),
		nullableUUIDValue(session.ActiveFormSubmissionID),
		nullableUUIDValue(session.LinkedCaseID),
		nullableUUIDValue(session.HandlingTeamID),
		nullableUUIDValue(session.AssignedOperatorUserID),
		nullableUUIDValue(session.DelegatedRuntimeConnectorID),
		nullableString(session.Title),
		nullableString(session.LanguageCode),
		nullableString(session.SourceRef),
		nullableString(session.ExternalSessionKey),
		session.OpenedAt,
		session.LastActivityAt,
		session.ClosedAt,
		metadata,
		session.CreatedAt,
		session.UpdatedAt,
	).Scan(&session.ID)
	return TranslateSqlxError(err, "conversation_sessions")
}

func (s *ConversationStore) GetConversationSession(ctx context.Context, sessionID string) (*servicedomain.ConversationSession, error) {
	var model models.ConversationSession
	query := `SELECT * FROM core_service.conversation_sessions WHERE id = ?`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, sessionID); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "conversation_sessions")
	}
	return mapConversationSessionToDomain(&model), nil
}

func (s *ConversationStore) GetConversationSessionByExternalKey(ctx context.Context, workspaceID string, channel servicedomain.ConversationChannel, externalSessionKey string) (*servicedomain.ConversationSession, error) {
	var model models.ConversationSession
	query := `
		SELECT * FROM core_service.conversation_sessions
		WHERE workspace_id = ? AND channel = ? AND external_session_key = ? AND closed_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, string(channel), externalSessionKey); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "conversation_sessions")
	}
	return mapConversationSessionToDomain(&model), nil
}

func (s *ConversationStore) UpdateConversationSession(ctx context.Context, session *servicedomain.ConversationSession) error {
	metadata, err := marshalJSONString(session.Metadata, "metadata")
	if err != nil {
		return err
	}

	query := `
		UPDATE core_service.conversation_sessions SET
			channel = ?, status = ?, primary_contact_id = ?, primary_catalog_node_id = ?,
			active_form_spec_id = ?, active_form_submission_id = ?, linked_case_id = ?,
			handling_team_id = ?, assigned_operator_user_id = ?, delegated_runtime_connector_id = ?, title = ?,
			language_code = ?, source_ref = ?, external_session_key = ?, opened_at = ?,
			last_activity_at = ?, closed_at = ?, metadata_json = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		string(session.Channel),
		string(session.Status),
		nullableUUIDValue(session.PrimaryContactID),
		nullableUUIDValue(session.PrimaryCatalogNodeID),
		nullableUUIDValue(session.ActiveFormSpecID),
		nullableUUIDValue(session.ActiveFormSubmissionID),
		nullableUUIDValue(session.LinkedCaseID),
		nullableUUIDValue(session.HandlingTeamID),
		nullableUUIDValue(session.AssignedOperatorUserID),
		nullableUUIDValue(session.DelegatedRuntimeConnectorID),
		nullableString(session.Title),
		nullableString(session.LanguageCode),
		nullableString(session.SourceRef),
		nullableString(session.ExternalSessionKey),
		session.OpenedAt,
		session.LastActivityAt,
		session.ClosedAt,
		metadata,
		session.UpdatedAt,
		session.ID,
		session.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "conversation_sessions")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ConversationStore) ListWorkspaceConversationSessions(ctx context.Context, workspaceID string, filter servicedomain.ConversationSessionFilter) ([]*servicedomain.ConversationSession, error) {
	conditions := []string{"workspace_id = ?"}
	args := []interface{}{workspaceID}

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}
	if filter.Channel != "" {
		conditions = append(conditions, "channel = ?")
		args = append(args, string(filter.Channel))
	}
	if filter.PrimaryCatalogNodeID != "" {
		conditions = append(conditions, "primary_catalog_node_id = ?")
		args = append(args, filter.PrimaryCatalogNodeID)
	}
	if filter.PrimaryContactID != "" {
		conditions = append(conditions, "primary_contact_id = ?")
		args = append(args, filter.PrimaryContactID)
	}
	if filter.LinkedCaseID != "" {
		conditions = append(conditions, "linked_case_id = ?")
		args = append(args, filter.LinkedCaseID)
	}

	query := `SELECT * FROM core_service.conversation_sessions WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY last_activity_at DESC, id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += ` OFFSET ?`
			args = append(args, filter.Offset)
		}
	}

	var modelsList []models.ConversationSession
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "conversation_sessions")
	}

	sessions := make([]*servicedomain.ConversationSession, len(modelsList))
	for i := range modelsList {
		sessions[i] = mapConversationSessionToDomain(&modelsList[i])
	}
	return sessions, nil
}

func (s *ConversationStore) CreateConversationParticipant(ctx context.Context, participant *servicedomain.ConversationParticipant) error {
	normalizePersistedUUID(&participant.ID)

	metadata, err := marshalJSONString(participant.Metadata, "metadata")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.conversation_participants (
			id, workspace_id, conversation_session_id, participant_kind, participant_ref,
			role_in_session, display_name, joined_at, left_at, metadata_json, created_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		participant.ID,
		participant.WorkspaceID,
		participant.ConversationSessionID,
		string(participant.ParticipantKind),
		participant.ParticipantRef,
		string(participant.RoleInSession),
		nullableString(participant.DisplayName),
		participant.JoinedAt,
		participant.LeftAt,
		metadata,
		participant.CreatedAt,
	).Scan(&participant.ID)
	return TranslateSqlxError(err, "conversation_participants")
}

func (s *ConversationStore) ListConversationParticipants(ctx context.Context, sessionID string) ([]*servicedomain.ConversationParticipant, error) {
	var modelsList []models.ConversationParticipant
	query := `SELECT * FROM core_service.conversation_participants WHERE conversation_session_id = ? ORDER BY joined_at, id`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, sessionID); err != nil {
		return nil, TranslateSqlxError(err, "conversation_participants")
	}

	participants := make([]*servicedomain.ConversationParticipant, len(modelsList))
	for i := range modelsList {
		participants[i] = mapConversationParticipantToDomain(&modelsList[i])
	}
	return participants, nil
}

func (s *ConversationStore) CreateConversationMessage(ctx context.Context, message *servicedomain.ConversationMessage) error {
	normalizePersistedUUID(&message.ID)

	content, err := marshalJSONString(message.Content, "content")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.conversation_messages (
			id, workspace_id, conversation_session_id, participant_id, role, kind,
			visibility, content_text, content_json, created_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		message.ID,
		message.WorkspaceID,
		message.ConversationSessionID,
		nullableUUIDValue(message.ParticipantID),
		string(message.Role),
		string(message.Kind),
		string(message.Visibility),
		nullableString(message.ContentText),
		content,
		message.CreatedAt,
	).Scan(&message.ID)
	return TranslateSqlxError(err, "conversation_messages")
}

func (s *ConversationStore) ListConversationMessages(ctx context.Context, sessionID string, visibility servicedomain.ConversationMessageVisibility) ([]*servicedomain.ConversationMessage, error) {
	query := `SELECT * FROM core_service.conversation_messages WHERE conversation_session_id = ?`
	args := []interface{}{sessionID}
	if visibility != "" {
		query += ` AND visibility = ?`
		args = append(args, string(visibility))
	}
	query += ` ORDER BY created_at, id`

	var modelsList []models.ConversationMessage
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "conversation_messages")
	}

	messages := make([]*servicedomain.ConversationMessage, len(modelsList))
	for i := range modelsList {
		messages[i] = mapConversationMessageToDomain(&modelsList[i])
	}
	return messages, nil
}

func (s *ConversationStore) UpsertConversationWorkingState(ctx context.Context, state *servicedomain.ConversationWorkingState) error {
	marshaler := NewBulkMarshaler("conversation_working_state")
	marshaler.Add("suggested_catalog_nodes", state.SuggestedCatalogNodes).
		Add("collected_fields", state.CollectedFields).
		Add("missing_fields", state.MissingFields)
	if err := marshaler.Error(); err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.conversation_working_state (
			conversation_session_id, workspace_id, primary_catalog_node_id,
			suggested_catalog_nodes_json, classification_confidence, active_policy_profile_ref,
			active_form_spec_id, active_form_submission_id, collected_fields_json,
			missing_fields_json, requires_operator_review, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		ON CONFLICT (conversation_session_id) DO UPDATE SET
			workspace_id = EXCLUDED.workspace_id,
			primary_catalog_node_id = EXCLUDED.primary_catalog_node_id,
			suggested_catalog_nodes_json = EXCLUDED.suggested_catalog_nodes_json,
			classification_confidence = EXCLUDED.classification_confidence,
			active_policy_profile_ref = EXCLUDED.active_policy_profile_ref,
			active_form_spec_id = EXCLUDED.active_form_spec_id,
			active_form_submission_id = EXCLUDED.active_form_submission_id,
			collected_fields_json = EXCLUDED.collected_fields_json,
			missing_fields_json = EXCLUDED.missing_fields_json,
			requires_operator_review = EXCLUDED.requires_operator_review,
			updated_at = EXCLUDED.updated_at`

	_, err := s.db.Get(ctx).ExecContext(ctx, query,
		state.ConversationSessionID,
		state.WorkspaceID,
		nullableUUIDValue(state.PrimaryCatalogNodeID),
		marshaler.Get("suggested_catalog_nodes"),
		state.ClassificationConfidence,
		nullableString(state.ActivePolicyProfileRef),
		nullableUUIDValue(state.ActiveFormSpecID),
		nullableUUIDValue(state.ActiveFormSubmissionID),
		marshaler.Get("collected_fields"),
		marshaler.Get("missing_fields"),
		state.RequiresOperatorReview,
		state.UpdatedAt,
	)
	return TranslateSqlxError(err, "conversation_working_state")
}

func (s *ConversationStore) GetConversationWorkingState(ctx context.Context, sessionID string) (*servicedomain.ConversationWorkingState, error) {
	var model models.ConversationWorkingState
	query := `SELECT * FROM core_service.conversation_working_state WHERE conversation_session_id = ?`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, sessionID); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "conversation_working_state")
	}
	return mapConversationWorkingStateToDomain(&model), nil
}

func (s *ConversationStore) CreateConversationOutcome(ctx context.Context, outcome *servicedomain.ConversationOutcome) error {
	normalizePersistedUUID(&outcome.ID)

	resultRef, err := marshalJSONString(outcome.ResultRef, "result_ref")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.conversation_outcomes (
			id, workspace_id, conversation_session_id, kind, result_ref_json, created_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		outcome.ID,
		outcome.WorkspaceID,
		outcome.ConversationSessionID,
		string(outcome.Kind),
		resultRef,
		outcome.CreatedAt,
	).Scan(&outcome.ID)
	return TranslateSqlxError(err, "conversation_outcomes")
}

func (s *ConversationStore) ListConversationOutcomes(ctx context.Context, sessionID string) ([]*servicedomain.ConversationOutcome, error) {
	var modelsList []models.ConversationOutcome
	query := `SELECT * FROM core_service.conversation_outcomes WHERE conversation_session_id = ? ORDER BY created_at, id`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, sessionID); err != nil {
		return nil, TranslateSqlxError(err, "conversation_outcomes")
	}

	outcomes := make([]*servicedomain.ConversationOutcome, len(modelsList))
	for i := range modelsList {
		outcomes[i] = mapConversationOutcomeToDomain(&modelsList[i])
	}
	return outcomes, nil
}

func mapConversationSessionToDomain(model *models.ConversationSession) *servicedomain.ConversationSession {
	return &servicedomain.ConversationSession{
		ID:                          model.ID,
		WorkspaceID:                 model.WorkspaceID,
		Channel:                     servicedomain.ConversationChannel(model.Channel),
		Status:                      servicedomain.ConversationStatus(model.Status),
		PrimaryContactID:            valueOrEmpty(model.PrimaryContactID),
		PrimaryCatalogNodeID:        valueOrEmpty(model.PrimaryCatalogNodeID),
		ActiveFormSpecID:            valueOrEmpty(model.ActiveFormSpecID),
		ActiveFormSubmissionID:      valueOrEmpty(model.ActiveFormSubmissionID),
		LinkedCaseID:                valueOrEmpty(model.LinkedCaseID),
		HandlingTeamID:              valueOrEmpty(model.HandlingTeamID),
		AssignedOperatorUserID:      valueOrEmpty(model.AssignedOperatorUserID),
		DelegatedRuntimeConnectorID: valueOrEmpty(model.DelegatedRuntimeConnectorID),
		Title:                       valueOrEmpty(model.Title),
		LanguageCode:                valueOrEmpty(model.LanguageCode),
		SourceRef:                   valueOrEmpty(model.SourceRef),
		ExternalSessionKey:          valueOrEmpty(model.ExternalSessionKey),
		OpenedAt:                    model.OpenedAt,
		LastActivityAt:              model.LastActivityAt,
		ClosedAt:                    model.ClosedAt,
		Metadata:                    unmarshalTypedSchemaOrEmpty(model.MetadataJSON, "conversation_sessions", "metadata_json"),
		CreatedAt:                   model.CreatedAt,
		UpdatedAt:                   model.UpdatedAt,
	}
}

func mapConversationParticipantToDomain(model *models.ConversationParticipant) *servicedomain.ConversationParticipant {
	return &servicedomain.ConversationParticipant{
		ID:                    model.ID,
		WorkspaceID:           model.WorkspaceID,
		ConversationSessionID: model.ConversationSessionID,
		ParticipantKind:       servicedomain.ConversationParticipantKind(model.ParticipantKind),
		ParticipantRef:        model.ParticipantRef,
		RoleInSession:         servicedomain.ConversationParticipantRole(model.RoleInSession),
		DisplayName:           valueOrEmpty(model.DisplayName),
		JoinedAt:              model.JoinedAt,
		LeftAt:                model.LeftAt,
		Metadata:              unmarshalTypedSchemaOrEmpty(model.MetadataJSON, "conversation_participants", "metadata_json"),
		CreatedAt:             model.CreatedAt,
	}
}

func mapConversationMessageToDomain(model *models.ConversationMessage) *servicedomain.ConversationMessage {
	return &servicedomain.ConversationMessage{
		ID:                    model.ID,
		WorkspaceID:           model.WorkspaceID,
		ConversationSessionID: model.ConversationSessionID,
		ParticipantID:         valueOrEmpty(model.ParticipantID),
		Role:                  servicedomain.ConversationMessageRole(model.Role),
		Kind:                  servicedomain.ConversationMessageKind(model.Kind),
		Visibility:            servicedomain.ConversationMessageVisibility(model.Visibility),
		ContentText:           valueOrEmpty(model.ContentText),
		Content:               unmarshalTypedSchemaOrEmpty(model.ContentJSON, "conversation_messages", "content_json"),
		CreatedAt:             model.CreatedAt,
	}
}

func mapConversationWorkingStateToDomain(model *models.ConversationWorkingState) *servicedomain.ConversationWorkingState {
	var suggestions []servicedomain.ConversationCatalogSuggestion
	unmarshalJSONField(model.SuggestedCatalogNodesJSON, &suggestions, "conversation_working_state", "suggested_catalog_nodes_json")

	return &servicedomain.ConversationWorkingState{
		ConversationSessionID:    model.ConversationSessionID,
		WorkspaceID:              model.WorkspaceID,
		PrimaryCatalogNodeID:     valueOrEmpty(model.PrimaryCatalogNodeID),
		SuggestedCatalogNodes:    suggestions,
		ClassificationConfidence: model.ClassificationConfidence,
		ActivePolicyProfileRef:   valueOrEmpty(model.ActivePolicyProfileRef),
		ActiveFormSpecID:         valueOrEmpty(model.ActiveFormSpecID),
		ActiveFormSubmissionID:   valueOrEmpty(model.ActiveFormSubmissionID),
		CollectedFields:          unmarshalTypedSchemaOrEmpty(model.CollectedFieldsJSON, "conversation_working_state", "collected_fields_json"),
		MissingFields:            unmarshalTypedSchemaOrEmpty(model.MissingFieldsJSON, "conversation_working_state", "missing_fields_json"),
		RequiresOperatorReview:   model.RequiresOperatorReview,
		UpdatedAt:                model.UpdatedAt,
	}
}

func mapConversationOutcomeToDomain(model *models.ConversationOutcome) *servicedomain.ConversationOutcome {
	return &servicedomain.ConversationOutcome{
		ID:                    model.ID,
		WorkspaceID:           model.WorkspaceID,
		ConversationSessionID: model.ConversationSessionID,
		Kind:                  servicedomain.ConversationOutcomeKind(model.Kind),
		ResultRef:             unmarshalTypedSchemaOrEmpty(model.ResultRefJSON, "conversation_outcomes", "result_ref_json"),
		CreatedAt:             model.CreatedAt,
	}
}

var _ shared.ConversationStore = (*ConversationStore)(nil)
