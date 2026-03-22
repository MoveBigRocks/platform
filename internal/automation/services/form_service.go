package automationservices

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/graph/model"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// RateLimitStore is the interface needed for rate limiting operations.
type RateLimitStore interface {
	CheckRateLimit(ctx context.Context, key string, maxAttempts int, window, blockDuration time.Duration) (bool, time.Duration, error)
}

// TenantContextSetter sets tenant context for RLS policies.
type TenantContextSetter interface {
	SetTenantContext(ctx context.Context, workspaceID string) error
}

// FormService handles all form-related business logic
type FormService struct {
	formStore           shared.FormStore
	rateLimitStore      RateLimitStore
	txRunner            contracts.TransactionRunner
	tenantContextSetter TenantContextSetter
	outbox              contracts.OutboxPublisher
	logger              *logger.Logger
}

// NewFormServiceWithDeps creates a form service with all dependencies for public form handling.
func NewFormServiceWithDeps(
	formStore shared.FormStore,
	rateLimitStore RateLimitStore,
	txRunner contracts.TransactionRunner,
	tenantContextSetter TenantContextSetter,
	outbox contracts.OutboxPublisher,
) *FormService {
	return &FormService{
		formStore:           formStore,
		rateLimitStore:      rateLimitStore,
		txRunner:            txRunner,
		tenantContextSetter: tenantContextSetter,
		outbox:              outbox,
		logger:              logger.New().WithField("service", "form"),
	}
}

// GetForm retrieves a form definition by ID.
func (s *FormService) GetForm(ctx context.Context, formID string) (*servicedomain.FormSchema, error) {
	if formID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("form_id", "required"))
	}

	form, err := s.formStore.GetFormSchema(ctx, formID)
	if err != nil {
		return nil, apierrors.NotFoundError("form", formID)
	}
	return form, nil
}

// GetFormBySlug retrieves a form by workspace and slug
func (s *FormService) GetFormBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSchema, error) {
	if slug == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("slug", "required"))
	}

	form, err := s.formStore.GetFormBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, apierrors.NotFoundError("form", slug)
	}
	return form, nil
}

// GetFormByCryptoID retrieves a form by its crypto ID (for public forms)
func (s *FormService) GetFormByCryptoID(ctx context.Context, cryptoID string) (*servicedomain.FormSchema, error) {
	if cryptoID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("crypto_id", "required"))
	}

	form, err := s.formStore.GetFormByCryptoID(ctx, cryptoID)
	if err != nil {
		return nil, apierrors.NotFoundError("form", cryptoID)
	}
	return form, nil
}

// ListWorkspaceForms lists all forms for a workspace
func (s *FormService) ListWorkspaceForms(ctx context.Context, workspaceID string) ([]*servicedomain.FormSchema, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	return s.formStore.ListWorkspaceFormSchemas(ctx, workspaceID)
}

// ListAllForms lists all forms across workspaces (requires admin context)
func (s *FormService) ListAllForms(ctx context.Context) ([]*servicedomain.FormSchema, error) {
	return s.formStore.ListAllFormSchemas(ctx)
}

// ListAllFormsFiltered lists admin forms with GraphQL filter translation kept out of resolvers.
func (s *FormService) ListAllFormsFiltered(ctx context.Context, filter *model.AdminFormFilterInput) ([]*servicedomain.FormSchema, error) {
	forms, err := s.ListAllForms(ctx)
	if err != nil {
		return nil, err
	}
	if filter == nil {
		return forms, nil
	}

	result := make([]*servicedomain.FormSchema, 0, len(forms))
	for _, form := range forms {
		if filter.WorkspaceID != nil && form.WorkspaceID != *filter.WorkspaceID {
			continue
		}
		if filter.Status != nil && string(form.Status) != *filter.Status {
			continue
		}
		if filter.IsPublic != nil && form.IsPublic != *filter.IsPublic {
			continue
		}
		result = append(result, form)
	}
	if filter.First != nil && len(result) > int(*filter.First) {
		result = result[:int(*filter.First)]
	}
	return result, nil
}

// ListPublicForms lists all public forms
func (s *FormService) ListPublicForms(ctx context.Context) ([]*servicedomain.FormSchema, error) {
	return s.formStore.ListPublicForms(ctx)
}

// CreateForm creates a new form definition.
func (s *FormService) CreateForm(ctx context.Context, form *servicedomain.FormSchema) error {
	if form.WorkspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if form.Name == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("name", "required"))
	}

	if err := s.formStore.CreateFormSchema(ctx, form); err != nil {
		return apierrors.DatabaseError("create form", err)
	}
	return nil
}

// UpdateForm updates an existing form definition.
func (s *FormService) UpdateForm(ctx context.Context, form *servicedomain.FormSchema) error {
	if form.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}

	if err := s.formStore.UpdateFormSchema(ctx, form); err != nil {
		return apierrors.DatabaseError("update form", err)
	}
	return nil
}

// DeleteForm deletes a form definition.
func (s *FormService) DeleteForm(ctx context.Context, workspaceID, formID string) error {
	if workspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if formID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("form_id", "required"))
	}

	if err := s.formStore.DeleteFormSchema(ctx, workspaceID, formID); err != nil {
		return apierrors.DatabaseError("delete form", err)
	}
	return nil
}

// CreateSubmission creates a new form submission
func (s *FormService) CreateSubmission(ctx context.Context, submission *servicedomain.PublicFormSubmission) error {
	if submission.FormID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("form_id", "required"))
	}

	if err := s.formStore.CreateFormSubmission(ctx, submission); err != nil {
		return apierrors.DatabaseError("create form submission", err)
	}
	return nil
}

// GetSubmission retrieves a form submission by ID
func (s *FormService) GetSubmission(ctx context.Context, submissionID string) (*servicedomain.PublicFormSubmission, error) {
	if submissionID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("submission_id", "required"))
	}

	submission, err := s.formStore.GetFormSubmission(ctx, submissionID)
	if err != nil {
		return nil, apierrors.NotFoundError("form_submission", submissionID)
	}
	return submission, nil
}

// UpdateSubmission updates an existing form submission
func (s *FormService) UpdateSubmission(ctx context.Context, submission *servicedomain.PublicFormSubmission) error {
	if submission.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}

	if err := s.formStore.UpdateFormSubmission(ctx, submission); err != nil {
		return apierrors.DatabaseError("update form submission", err)
	}
	return nil
}

// ListFormSubmissions lists all submissions for a form
func (s *FormService) ListFormSubmissions(ctx context.Context, formID string) ([]*servicedomain.PublicFormSubmission, error) {
	if formID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("form_id", "required"))
	}
	return s.formStore.ListFormSubmissions(ctx, formID)
}

// GetFormAnalytics retrieves analytics for a form
func (s *FormService) GetFormAnalytics(ctx context.Context, formID string) (*servicedomain.FormAnalytics, error) {
	if formID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("form_id", "required"))
	}
	return s.formStore.GetFormAnalytics(ctx, formID)
}

// =============================================================================
// Public Form Operations
// =============================================================================

// CheckSubmissionRateLimit checks if a form submission is rate limited by IP.
// Returns (allowed bool, retryAfter time.Duration, error).
// If rate limiting is not configured, returns (true, 0, nil).
func (s *FormService) CheckSubmissionRateLimit(ctx context.Context, clientIP string, maxAttempts int, window, blockDuration time.Duration) (bool, time.Duration, error) {
	if s.rateLimitStore == nil {
		// Rate limiting not configured, allow all
		return true, 0, nil
	}

	key := fmt.Sprintf("form_submission:%s", clientIP)
	return s.rateLimitStore.CheckRateLimit(ctx, key, maxAttempts, window, blockDuration)
}

// GetFormAPIToken retrieves a form API token by its token value
func (s *FormService) GetFormAPIToken(ctx context.Context, token string) (*servicedomain.FormAPIToken, error) {
	if token == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("token", "required"))
	}

	apiToken, err := s.formStore.GetFormAPIToken(ctx, token)
	if err != nil {
		return nil, apierrors.NotFoundError("form_api_token", token)
	}
	return apiToken, nil
}

// CreatePublicSubmission creates a form submission with tenant context.
// This is used for public form endpoints that bypass normal RLS middleware.
// It handles setting up the tenant context and transaction internally.
func (s *FormService) CreatePublicSubmission(ctx context.Context, workspaceID string, submission *servicedomain.PublicFormSubmission, event *contracts.FormSubmittedEvent) error {
	if workspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if submission.FormID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("form_id", "required"))
	}

	// If we have transaction runner and tenant context setter, use them
	if s.txRunner != nil && s.tenantContextSetter != nil {
		return s.txRunner.WithTransaction(ctx, func(txCtx context.Context) error {
			// Set tenant context for RLS policies
			if err := s.tenantContextSetter.SetTenantContext(txCtx, workspaceID); err != nil {
				return fmt.Errorf("set tenant context: %w", err)
			}
			// Save submission within tenant-scoped transaction
			if err := s.formStore.CreateFormSubmission(txCtx, submission); err != nil {
				return fmt.Errorf("create form submission: %w", err)
			}
			if event != nil {
				event.SubmissionID = submission.ID
			}
			if err := s.publishSubmissionEvent(txCtx, event); err != nil {
				return err
			}
			return nil
		})
	}

	// Fallback to simple creation (for non-public contexts)
	if err := s.formStore.CreateFormSubmission(ctx, submission); err != nil {
		return err
	}
	if event != nil {
		event.SubmissionID = submission.ID
	}
	return s.publishSubmissionEvent(ctx, event)
}

func (s *FormService) publishSubmissionEvent(ctx context.Context, event *contracts.FormSubmittedEvent) error {
	if event == nil || s.outbox == nil {
		return nil
	}
	if err := s.outbox.PublishEvent(ctx, eventbus.StreamFormEvents, *event); err != nil {
		return fmt.Errorf("publish form submission event: %w", err)
	}
	return nil
}
