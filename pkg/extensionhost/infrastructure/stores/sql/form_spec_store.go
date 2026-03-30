package sql

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type FormSpecStore struct {
	db *SqlxDB
}

func NewFormSpecStore(db *SqlxDB) *FormSpecStore {
	return &FormSpecStore{db: db}
}

func (s *FormSpecStore) CreateFormSpec(ctx context.Context, spec *servicedomain.FormSpec) error {
	normalizePersistedUUID(&spec.ID)

	marshaler := NewBulkMarshaler("form_spec")
	marshaler.Add("field_spec", spec.FieldSpec).
		Add("evidence_requirements", spec.EvidenceRequirements).
		Add("inference_rules", spec.InferenceRules).
		Add("approval_policy", spec.ApprovalPolicy).
		Add("submission_policy", spec.SubmissionPolicy).
		Add("destination_policy", spec.DestinationPolicy).
		Add("metadata", spec.Metadata)
	if err := marshaler.Error(); err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.form_specs (
			id, workspace_id, name, slug, public_key, description_markdown, field_spec_json,
			evidence_requirements_json, inference_rules_json, approval_policy_json,
			submission_policy_json, destination_policy_json, supported_channels,
			is_public, status, metadata_json, created_by, created_at, updated_at, deleted_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		spec.ID,
		spec.WorkspaceID,
		spec.Name,
		spec.Slug,
		nullableString(spec.PublicKey),
		spec.DescriptionMarkdown,
		marshaler.Get("field_spec"),
		marshaler.Get("evidence_requirements"),
		marshaler.Get("inference_rules"),
		marshaler.Get("approval_policy"),
		marshaler.Get("submission_policy"),
		marshaler.Get("destination_policy"),
		pq.Array(spec.SupportedChannels),
		spec.IsPublic,
		string(spec.Status),
		marshaler.Get("metadata"),
		nullableLegacyUUIDValue(spec.CreatedBy),
		spec.CreatedAt,
		spec.UpdatedAt,
		spec.DeletedAt,
	).Scan(&spec.ID)
	return TranslateSqlxError(err, "form_specs")
}

func (s *FormSpecStore) GetFormSpec(ctx context.Context, specID string) (*servicedomain.FormSpec, error) {
	var model models.FormSpec
	query := `SELECT * FROM core_service.form_specs WHERE id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, specID); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "form_specs")
	}
	return mapFormSpecToDomain(&model), nil
}

func (s *FormSpecStore) GetFormSpecBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSpec, error) {
	var model models.FormSpec
	query := `SELECT * FROM core_service.form_specs WHERE workspace_id = ? AND slug = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, slug); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "form_specs")
	}
	return mapFormSpecToDomain(&model), nil
}

func (s *FormSpecStore) GetFormSpecByPublicKey(ctx context.Context, publicKey string) (*servicedomain.FormSpec, error) {
	var model models.FormSpec
	query := `SELECT * FROM core_service.form_specs WHERE public_key = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, publicKey); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "form_specs")
	}
	return mapFormSpecToDomain(&model), nil
}

func (s *FormSpecStore) UpdateFormSpec(ctx context.Context, spec *servicedomain.FormSpec) error {
	marshaler := NewBulkMarshaler("form_spec")
	marshaler.Add("field_spec", spec.FieldSpec).
		Add("evidence_requirements", spec.EvidenceRequirements).
		Add("inference_rules", spec.InferenceRules).
		Add("approval_policy", spec.ApprovalPolicy).
		Add("submission_policy", spec.SubmissionPolicy).
		Add("destination_policy", spec.DestinationPolicy).
		Add("metadata", spec.Metadata)
	if err := marshaler.Error(); err != nil {
		return err
	}

	query := `
		UPDATE core_service.form_specs SET
			name = ?, slug = ?, public_key = ?, description_markdown = ?, field_spec_json = ?,
			evidence_requirements_json = ?, inference_rules_json = ?, approval_policy_json = ?,
			submission_policy_json = ?, destination_policy_json = ?, supported_channels = ?,
			is_public = ?, status = ?, metadata_json = ?, updated_at = ?, deleted_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		spec.Name,
		spec.Slug,
		nullableString(spec.PublicKey),
		spec.DescriptionMarkdown,
		marshaler.Get("field_spec"),
		marshaler.Get("evidence_requirements"),
		marshaler.Get("inference_rules"),
		marshaler.Get("approval_policy"),
		marshaler.Get("submission_policy"),
		marshaler.Get("destination_policy"),
		pq.Array(spec.SupportedChannels),
		spec.IsPublic,
		string(spec.Status),
		marshaler.Get("metadata"),
		spec.UpdatedAt,
		spec.DeletedAt,
		spec.ID,
		spec.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "form_specs")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *FormSpecStore) ListWorkspaceFormSpecs(ctx context.Context, workspaceID string) ([]*servicedomain.FormSpec, error) {
	var modelsList []models.FormSpec
	query := `SELECT * FROM core_service.form_specs WHERE workspace_id = ? AND deleted_at IS NULL ORDER BY name ASC`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "form_specs")
	}

	specs := make([]*servicedomain.FormSpec, len(modelsList))
	for i := range modelsList {
		specs[i] = mapFormSpecToDomain(&modelsList[i])
	}
	return specs, nil
}

func (s *FormSpecStore) DeleteFormSpec(ctx context.Context, workspaceID, specID string) error {
	result, err := s.db.Get(ctx).ExecContext(ctx,
		`UPDATE core_service.form_specs SET deleted_at = ?, updated_at = ? WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`,
		time.Now().UTC(), time.Now().UTC(), specID, workspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "form_specs")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *FormSpecStore) CreateFormSubmission(ctx context.Context, submission *servicedomain.FormSubmission) error {
	normalizePersistedUUID(&submission.ID)

	marshaler := NewBulkMarshaler("form_submission")
	marshaler.Add("collected_fields", submission.CollectedFields).
		Add("missing_fields", submission.MissingFields).
		Add("evidence", submission.Evidence).
		Add("validation_errors", submission.ValidationErrors).
		Add("metadata", submission.Metadata)
	if err := marshaler.Error(); err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.form_submissions (
			id, workspace_id, form_spec_id, conversation_session_id, case_id, contact_id,
			status, channel, submitter_email, submitter_name, completion_token,
			collected_fields_json, missing_fields_json, evidence_json, validation_errors_json,
			metadata_json, submitted_at, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		submission.ID,
		submission.WorkspaceID,
		submission.FormSpecID,
		nullableUUIDValue(submission.ConversationSessionID),
		nullableUUIDValue(submission.CaseID),
		nullableUUIDValue(submission.ContactID),
		string(submission.Status),
		submission.Channel,
		nullableString(submission.SubmitterEmail),
		nullableString(submission.SubmitterName),
		nullableString(submission.CompletionToken),
		marshaler.Get("collected_fields"),
		marshaler.Get("missing_fields"),
		marshaler.Get("evidence"),
		marshaler.Get("validation_errors"),
		marshaler.Get("metadata"),
		submission.SubmittedAt,
		submission.CreatedAt,
		submission.UpdatedAt,
	).Scan(&submission.ID)
	return TranslateSqlxError(err, "form_submissions")
}

func (s *FormSpecStore) GetFormSubmission(ctx context.Context, submissionID string) (*servicedomain.FormSubmission, error) {
	var model models.FormSubmission
	query := `SELECT * FROM core_service.form_submissions WHERE id = ?`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, submissionID); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "form_submissions")
	}
	return mapFormSubmissionToDomain(&model), nil
}

func (s *FormSpecStore) UpdateFormSubmission(ctx context.Context, submission *servicedomain.FormSubmission) error {
	marshaler := NewBulkMarshaler("form_submission")
	marshaler.Add("collected_fields", submission.CollectedFields).
		Add("missing_fields", submission.MissingFields).
		Add("evidence", submission.Evidence).
		Add("validation_errors", submission.ValidationErrors).
		Add("metadata", submission.Metadata)
	if err := marshaler.Error(); err != nil {
		return err
	}

	query := `
		UPDATE core_service.form_submissions SET
			form_spec_id = ?, conversation_session_id = ?, case_id = ?, contact_id = ?,
			status = ?, channel = ?, submitter_email = ?, submitter_name = ?, completion_token = ?,
			collected_fields_json = ?, missing_fields_json = ?, evidence_json = ?,
			validation_errors_json = ?, metadata_json = ?, submitted_at = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		submission.FormSpecID,
		nullableUUIDValue(submission.ConversationSessionID),
		nullableUUIDValue(submission.CaseID),
		nullableUUIDValue(submission.ContactID),
		string(submission.Status),
		submission.Channel,
		nullableString(submission.SubmitterEmail),
		nullableString(submission.SubmitterName),
		nullableString(submission.CompletionToken),
		marshaler.Get("collected_fields"),
		marshaler.Get("missing_fields"),
		marshaler.Get("evidence"),
		marshaler.Get("validation_errors"),
		marshaler.Get("metadata"),
		submission.SubmittedAt,
		submission.UpdatedAt,
		submission.ID,
		submission.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "form_submissions")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *FormSpecStore) ListFormSubmissions(ctx context.Context, workspaceID string, filter servicedomain.FormSubmissionFilter) ([]*servicedomain.FormSubmission, error) {
	conditions := []string{"workspace_id = ?"}
	args := []interface{}{workspaceID}

	if filter.FormSpecID != "" {
		conditions = append(conditions, "form_spec_id = ?")
		args = append(args, filter.FormSpecID)
	}
	if filter.ConversationSessionID != "" {
		conditions = append(conditions, "conversation_session_id = ?")
		args = append(args, filter.ConversationSessionID)
	}
	if filter.CaseID != "" {
		conditions = append(conditions, "case_id = ?")
		args = append(args, filter.CaseID)
	}
	if filter.ContactID != "" {
		conditions = append(conditions, "contact_id = ?")
		args = append(args, filter.ContactID)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}

	query := `SELECT * FROM core_service.form_submissions WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY created_at DESC, id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += ` OFFSET ?`
			args = append(args, filter.Offset)
		}
	}

	var modelsList []models.FormSubmission
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "form_submissions")
	}

	submissions := make([]*servicedomain.FormSubmission, len(modelsList))
	for i := range modelsList {
		submissions[i] = mapFormSubmissionToDomain(&modelsList[i])
	}
	return submissions, nil
}

func mapFormSpecToDomain(model *models.FormSpec) *servicedomain.FormSpec {
	return &servicedomain.FormSpec{
		ID:                   model.ID,
		WorkspaceID:          model.WorkspaceID,
		Name:                 model.Name,
		Slug:                 model.Slug,
		PublicKey:            valueOrEmpty(model.PublicKey),
		DescriptionMarkdown:  model.DescriptionMarkdown,
		FieldSpec:            unmarshalTypedSchemaOrEmpty(model.FieldSpecJSON, "form_specs", "field_spec_json"),
		EvidenceRequirements: unmarshalTypedSchemaSliceOrEmpty(model.EvidenceRequirementsJSON, "form_specs", "evidence_requirements_json"),
		InferenceRules:       unmarshalTypedSchemaSliceOrEmpty(model.InferenceRulesJSON, "form_specs", "inference_rules_json"),
		ApprovalPolicy:       unmarshalTypedSchemaOrEmpty(model.ApprovalPolicyJSON, "form_specs", "approval_policy_json"),
		SubmissionPolicy:     unmarshalTypedSchemaOrEmpty(model.SubmissionPolicyJSON, "form_specs", "submission_policy_json"),
		DestinationPolicy:    unmarshalTypedSchemaOrEmpty(model.DestinationPolicyJSON, "form_specs", "destination_policy_json"),
		SupportedChannels:    unmarshalStringArrayField(model.SupportedChannels, "form_specs", "supported_channels"),
		IsPublic:             model.IsPublic,
		Status:               servicedomain.FormSpecStatus(model.Status),
		Metadata:             unmarshalTypedSchemaOrEmpty(model.MetadataJSON, "form_specs", "metadata_json"),
		CreatedBy:            valueOrEmpty(model.CreatedBy),
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
		DeletedAt:            model.DeletedAt,
	}
}

func mapFormSubmissionToDomain(model *models.FormSubmission) *servicedomain.FormSubmission {
	var validationErrors []string
	unmarshalJSONField(model.ValidationErrorsJSON, &validationErrors, "form_submissions", "validation_errors_json")

	return &servicedomain.FormSubmission{
		ID:                    model.ID,
		WorkspaceID:           model.WorkspaceID,
		FormSpecID:            model.FormSpecID,
		ConversationSessionID: valueOrEmpty(model.ConversationSessionID),
		CaseID:                valueOrEmpty(model.CaseID),
		ContactID:             valueOrEmpty(model.ContactID),
		Status:                servicedomain.FormSubmissionStatus(model.Status),
		Channel:               model.Channel,
		SubmitterEmail:        valueOrEmpty(model.SubmitterEmail),
		SubmitterName:         valueOrEmpty(model.SubmitterName),
		CompletionToken:       valueOrEmpty(model.CompletionToken),
		CollectedFields:       unmarshalTypedSchemaOrEmpty(model.CollectedFieldsJSON, "form_submissions", "collected_fields_json"),
		MissingFields:         unmarshalTypedSchemaOrEmpty(model.MissingFieldsJSON, "form_submissions", "missing_fields_json"),
		Evidence:              unmarshalTypedSchemaSliceOrEmpty(model.EvidenceJSON, "form_submissions", "evidence_json"),
		ValidationErrors:      validationErrors,
		Metadata:              unmarshalTypedSchemaOrEmpty(model.MetadataJSON, "form_submissions", "metadata_json"),
		SubmittedAt:           model.SubmittedAt,
		CreatedAt:             model.CreatedAt,
		UpdatedAt:             model.UpdatedAt,
	}
}

func nullableString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

var _ shared.FormSpecStore = (*FormSpecStore)(nil)
