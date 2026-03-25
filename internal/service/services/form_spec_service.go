package serviceapp

import (
	"context"
	"strings"
	"time"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

// FormSpecService manages form specs and collected submissions.
type FormSpecService struct {
	formSpecStore  shared.FormSpecStore
	workspaceStore shared.WorkspaceStore
}

func NewFormSpecService(formSpecStore shared.FormSpecStore, workspaceStore shared.WorkspaceStore) *FormSpecService {
	return &FormSpecService{
		formSpecStore:  formSpecStore,
		workspaceStore: workspaceStore,
	}
}

type CreateFormSpecParams struct {
	WorkspaceID          string
	Name                 string
	Slug                 string
	PublicKey            string
	DescriptionMarkdown  string
	FieldSpec            shareddomain.TypedSchema
	EvidenceRequirements []shareddomain.TypedSchema
	InferenceRules       []shareddomain.TypedSchema
	ApprovalPolicy       shareddomain.TypedSchema
	SubmissionPolicy     shareddomain.TypedSchema
	DestinationPolicy    shareddomain.TypedSchema
	SupportedChannels    []string
	IsPublic             bool
	Status               servicedomain.FormSpecStatus
	Metadata             shareddomain.TypedSchema
	CreatedBy            string
}

type UpdateFormSpecParams struct {
	Name                 *string
	Slug                 *string
	PublicKey            *string
	DescriptionMarkdown  *string
	FieldSpec            *shareddomain.TypedSchema
	EvidenceRequirements *[]shareddomain.TypedSchema
	InferenceRules       *[]shareddomain.TypedSchema
	ApprovalPolicy       *shareddomain.TypedSchema
	SubmissionPolicy     *shareddomain.TypedSchema
	DestinationPolicy    *shareddomain.TypedSchema
	SupportedChannels    *[]string
	IsPublic             *bool
	Status               *servicedomain.FormSpecStatus
	Metadata             *shareddomain.TypedSchema
}

type CreateFormSubmissionParams struct {
	FormSpecID            string
	ConversationSessionID string
	CaseID                string
	ContactID             string
	Status                servicedomain.FormSubmissionStatus
	Channel               string
	SubmitterEmail        string
	SubmitterName         string
	CompletionToken       string
	CollectedFields       shareddomain.TypedSchema
	MissingFields         shareddomain.TypedSchema
	Evidence              []shareddomain.TypedSchema
	ValidationErrors      []string
	Metadata              shareddomain.TypedSchema
	SubmittedAt           *time.Time
}

func (s *FormSpecService) GetFormSpec(ctx context.Context, specID string) (*servicedomain.FormSpec, error) {
	if strings.TrimSpace(specID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("spec_id", "required"))
	}
	spec, err := s.formSpecStore.GetFormSpec(ctx, specID)
	if err != nil {
		return nil, apierrors.NotFoundError("form spec", specID)
	}
	return spec, nil
}

func (s *FormSpecService) GetFormSpecBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSpec, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	normalizedSlug := strings.TrimSpace(slug)
	if normalizedSlug == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("slug", "required"))
	}
	spec, err := s.formSpecStore.GetFormSpecBySlug(ctx, workspaceID, normalizedSlug)
	if err != nil {
		return nil, apierrors.NotFoundError("form spec", normalizedSlug)
	}
	return spec, nil
}

func (s *FormSpecService) ListWorkspaceFormSpecs(ctx context.Context, workspaceID string) ([]*servicedomain.FormSpec, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	specs, err := s.formSpecStore.ListWorkspaceFormSpecs(ctx, workspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list form specs", err)
	}
	return specs, nil
}

func (s *FormSpecService) CreateFormSpec(ctx context.Context, params CreateFormSpecParams) (*servicedomain.FormSpec, error) {
	if strings.TrimSpace(params.WorkspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("name", "required"))
	}
	if s.workspaceStore != nil {
		workspace, err := s.workspaceStore.GetWorkspace(ctx, params.WorkspaceID)
		if err != nil || workspace == nil {
			return nil, apierrors.NotFoundError("workspace", params.WorkspaceID)
		}
	}

	status := params.Status
	if status == "" {
		status = servicedomain.FormSpecStatusDraft
	}

	spec := servicedomain.NewFormSpec(params.WorkspaceID, params.Slug, params.Name)
	spec.PublicKey = strings.TrimSpace(params.PublicKey)
	spec.DescriptionMarkdown = strings.TrimSpace(params.DescriptionMarkdown)
	spec.FieldSpec = cloneTypedSchema(params.FieldSpec)
	spec.EvidenceRequirements = cloneTypedSchemaSlice(params.EvidenceRequirements)
	spec.InferenceRules = cloneTypedSchemaSlice(params.InferenceRules)
	spec.ApprovalPolicy = cloneTypedSchema(params.ApprovalPolicy)
	spec.SubmissionPolicy = cloneTypedSchema(params.SubmissionPolicy)
	spec.DestinationPolicy = cloneTypedSchema(params.DestinationPolicy)
	spec.SupportedChannels = normalizeStringSlice(params.SupportedChannels)
	spec.IsPublic = params.IsPublic
	spec.Status = status
	spec.Metadata = cloneTypedSchema(params.Metadata)
	spec.CreatedBy = strings.TrimSpace(params.CreatedBy)

	if err := spec.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "form spec validation failed")
	}
	if err := s.formSpecStore.CreateFormSpec(ctx, spec); err != nil {
		return nil, apierrors.DatabaseError("create form spec", err)
	}
	return spec, nil
}

func (s *FormSpecService) UpdateFormSpec(ctx context.Context, specID string, params UpdateFormSpecParams) (*servicedomain.FormSpec, error) {
	spec, err := s.GetFormSpec(ctx, specID)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		if err := spec.Rename(*params.Name); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "form spec validation failed")
		}
	}
	if params.Slug != nil {
		if err := spec.SetSlug(*params.Slug); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "form spec validation failed")
		}
	}
	if params.PublicKey != nil {
		spec.PublicKey = strings.TrimSpace(*params.PublicKey)
	}
	if params.DescriptionMarkdown != nil {
		spec.DescriptionMarkdown = strings.TrimSpace(*params.DescriptionMarkdown)
	}
	if params.FieldSpec != nil {
		spec.FieldSpec = cloneTypedSchema(*params.FieldSpec)
	}
	if params.EvidenceRequirements != nil {
		spec.EvidenceRequirements = cloneTypedSchemaSlice(*params.EvidenceRequirements)
	}
	if params.InferenceRules != nil {
		spec.InferenceRules = cloneTypedSchemaSlice(*params.InferenceRules)
	}
	if params.ApprovalPolicy != nil {
		spec.ApprovalPolicy = cloneTypedSchema(*params.ApprovalPolicy)
	}
	if params.SubmissionPolicy != nil {
		spec.SubmissionPolicy = cloneTypedSchema(*params.SubmissionPolicy)
	}
	if params.DestinationPolicy != nil {
		spec.DestinationPolicy = cloneTypedSchema(*params.DestinationPolicy)
	}
	if params.SupportedChannels != nil {
		spec.SupportedChannels = normalizeStringSlice(*params.SupportedChannels)
	}
	if params.IsPublic != nil {
		spec.IsPublic = *params.IsPublic
	}
	if params.Status != nil {
		spec.Status = *params.Status
	}
	if params.Metadata != nil {
		spec.Metadata = cloneTypedSchema(*params.Metadata)
	}
	spec.UpdatedAt = time.Now().UTC()

	if err := spec.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "form spec validation failed")
	}
	if err := s.formSpecStore.UpdateFormSpec(ctx, spec); err != nil {
		return nil, apierrors.DatabaseError("update form spec", err)
	}
	return spec, nil
}

func (s *FormSpecService) GetFormSubmission(ctx context.Context, submissionID string) (*servicedomain.FormSubmission, error) {
	if strings.TrimSpace(submissionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("submission_id", "required"))
	}
	submission, err := s.formSpecStore.GetFormSubmission(ctx, submissionID)
	if err != nil {
		return nil, apierrors.NotFoundError("form submission", submissionID)
	}
	return submission, nil
}

func (s *FormSpecService) ListFormSubmissions(ctx context.Context, workspaceID string, filter servicedomain.FormSubmissionFilter) ([]*servicedomain.FormSubmission, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	submissions, err := s.formSpecStore.ListFormSubmissions(ctx, workspaceID, filter)
	if err != nil {
		return nil, apierrors.DatabaseError("list form submissions", err)
	}
	return submissions, nil
}

func (s *FormSpecService) CreateFormSubmission(ctx context.Context, params CreateFormSubmissionParams) (*servicedomain.FormSubmission, error) {
	if strings.TrimSpace(params.FormSpecID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("form_spec_id", "required"))
	}

	spec, err := s.GetFormSpec(ctx, params.FormSpecID)
	if err != nil {
		return nil, err
	}

	status := params.Status
	if status == "" {
		status = servicedomain.FormSubmissionStatusSubmitted
	}

	submission := servicedomain.NewFormSubmission(spec.WorkspaceID, spec.ID)
	submission.ConversationSessionID = strings.TrimSpace(params.ConversationSessionID)
	submission.CaseID = strings.TrimSpace(params.CaseID)
	submission.ContactID = strings.TrimSpace(params.ContactID)
	submission.Status = status
	submission.Channel = normalizeFormChannel(params.Channel, spec.SupportedChannels)
	submission.SubmitterEmail = strings.TrimSpace(params.SubmitterEmail)
	submission.SubmitterName = strings.TrimSpace(params.SubmitterName)
	submission.CompletionToken = strings.TrimSpace(params.CompletionToken)
	submission.CollectedFields = cloneTypedSchema(params.CollectedFields)
	submission.MissingFields = cloneTypedSchema(params.MissingFields)
	submission.Evidence = cloneTypedSchemaSlice(params.Evidence)
	submission.ValidationErrors = cloneStringSlice(params.ValidationErrors)
	submission.Metadata = cloneTypedSchema(params.Metadata)
	if params.SubmittedAt != nil {
		submission.SubmittedAt = params.SubmittedAt
	} else if status != servicedomain.FormSubmissionStatusDraft {
		now := time.Now().UTC()
		submission.SubmittedAt = &now
	}
	submission.UpdatedAt = time.Now().UTC()

	if err := submission.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "form submission validation failed")
	}
	if err := s.formSpecStore.CreateFormSubmission(ctx, submission); err != nil {
		return nil, apierrors.DatabaseError("create form submission", err)
	}
	return submission, nil
}

func NormalizeFormSpecStatus(value string) (servicedomain.FormSpecStatus, error) {
	switch servicedomain.FormSpecStatus(strings.ToLower(strings.TrimSpace(value))) {
	case "", servicedomain.FormSpecStatusDraft:
		return servicedomain.FormSpecStatusDraft, nil
	case servicedomain.FormSpecStatusActive:
		return servicedomain.FormSpecStatusActive, nil
	case servicedomain.FormSpecStatusArchived:
		return servicedomain.FormSpecStatusArchived, nil
	default:
		return "", apierrors.NewValidationErrors(apierrors.NewValidationError("status", "must be one of: draft, active, archived"))
	}
}

func NormalizeFormSubmissionStatus(value string) (servicedomain.FormSubmissionStatus, error) {
	switch servicedomain.FormSubmissionStatus(strings.ToLower(strings.TrimSpace(value))) {
	case "", servicedomain.FormSubmissionStatusSubmitted:
		return servicedomain.FormSubmissionStatusSubmitted, nil
	case servicedomain.FormSubmissionStatusDraft:
		return servicedomain.FormSubmissionStatusDraft, nil
	case servicedomain.FormSubmissionStatusAccepted:
		return servicedomain.FormSubmissionStatusAccepted, nil
	case servicedomain.FormSubmissionStatusRejected:
		return servicedomain.FormSubmissionStatusRejected, nil
	default:
		return "", apierrors.NewValidationErrors(apierrors.NewValidationError("status", "must be one of: draft, submitted, accepted, rejected"))
	}
}

func cloneTypedSchema(schema shareddomain.TypedSchema) shareddomain.TypedSchema {
	return schema.Clone()
}

func cloneTypedSchemaSlice(items []shareddomain.TypedSchema) []shareddomain.TypedSchema {
	if len(items) == 0 {
		return []shareddomain.TypedSchema{}
	}
	result := make([]shareddomain.TypedSchema, 0, len(items))
	for _, item := range items {
		result = append(result, item.Clone())
	}
	return result
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func normalizeFormChannel(channel string, supported []string) string {
	normalized := strings.TrimSpace(channel)
	if normalized != "" {
		return normalized
	}
	if len(supported) > 0 && strings.TrimSpace(supported[0]) != "" {
		return strings.TrimSpace(supported[0])
	}
	return "operator_console"
}
