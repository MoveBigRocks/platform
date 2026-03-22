package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// FormStore provides the web-form surface on top of form specs and submissions.
type FormStore struct {
	db          *SqlxDB
	logger      *logger.Logger
	formSpecStore *FormSpecStore
}

func NewFormStore(db *SqlxDB) *FormStore {
	return &FormStore{
		db:          db,
		logger:      logger.New(),
		formSpecStore: NewFormSpecStore(db),
	}
}

type formSurfaceConfig struct {
	UISchema               shareddomain.TypedSchema               `json:"ui_schema"`
	ValidationRules        shareddomain.TypedSchema               `json:"validation_rules"`
	RequiresAuth           bool                                   `json:"requires_auth"`
	AllowMultiple          bool                                   `json:"allow_multiple"`
	CollectEmail           bool                                   `json:"collect_email"`
	AutoCreateCase         bool                                   `json:"auto_create_case"`
	AutoAssignTeamID       string                                 `json:"auto_assign_team_id,omitempty"`
	AutoAssignUserID       string                                 `json:"auto_assign_user_id,omitempty"`
	AutoCasePriority       string                                 `json:"auto_case_priority,omitempty"`
	AutoCaseType           string                                 `json:"auto_case_type,omitempty"`
	AutoTags               []string                               `json:"auto_tags,omitempty"`
	NotifyOnSubmission     bool                                   `json:"notify_on_submission"`
	NotificationEmails     []string                               `json:"notification_emails,omitempty"`
	NotificationWebhookURL string                                 `json:"notification_webhook_url,omitempty"`
	Theme                  string                                 `json:"theme,omitempty"`
	CustomCSS              string                                 `json:"custom_css,omitempty"`
	SubmissionMessage      string                                 `json:"submission_message,omitempty"`
	RedirectURL            string                                 `json:"redirect_url,omitempty"`
	AllowedDomains         []string                               `json:"allowed_domains,omitempty"`
	BlockedDomains         []string                               `json:"blocked_domains,omitempty"`
	RequiresCaptcha        bool                                   `json:"requires_captcha"`
	MaxSubmissionsPerDay   int                                    `json:"max_submissions_per_day,omitempty"`
	AllowEmbed             bool                                   `json:"allow_embed"`
	EmbedDomains           []string                               `json:"embed_domains,omitempty"`
	TriggerRuleIDs         []string                               `json:"trigger_rule_ids,omitempty"`
	HasWorkflow            bool                                   `json:"has_workflow"`
	WorkflowStates         []servicedomain.FormWorkflowState      `json:"workflow_states,omitempty"`
	Transitions            []servicedomain.FormWorkflowTransition `json:"transitions,omitempty"`
}

type formSubmissionEnvelope struct {
	RawData         string                          `json:"raw_data,omitempty"`
	ProcessedData   map[string]interface{}          `json:"processed_data,omitempty"`
	SubmitterIP     string                          `json:"submitter_ip,omitempty"`
	UserAgent       string                          `json:"user_agent,omitempty"`
	Referrer        string                          `json:"referrer,omitempty"`
	ProcessingError string                          `json:"processing_error,omitempty"`
	ProcessingNotes string                          `json:"processing_notes,omitempty"`
	IsValid         bool                            `json:"is_valid"`
	SpamScore       float64                         `json:"spam_score,omitempty"`
	IsSpam          bool                            `json:"is_spam"`
	SpamReasons     []string                        `json:"spam_reasons,omitempty"`
	ProcessedAt     *time.Time                      `json:"processed_at,omitempty"`
	ProcessedByID   string                          `json:"processed_by_id,omitempty"`
	ProcessingTime  int64                           `json:"processing_time,omitempty"`
	AttachmentIDs   []string                        `json:"attachment_ids,omitempty"`
	CurrentStateID  string                          `json:"current_state_id,omitempty"`
	StateHistory    []servicedomain.StateTransition `json:"state_history,omitempty"`
}

type formAccessTokenRow struct {
	ID           string     `db:"id"`
	WorkspaceID  string     `db:"workspace_id"`
	FormSpecID string     `db:"form_spec_id"`
	Token        string     `db:"token"`
	Name         string     `db:"name"`
	IsActive     bool       `db:"is_active"`
	ExpiresAt    *time.Time `db:"expires_at"`
	AllowedHosts string     `db:"allowed_hosts"`
	LastUsedAt   *time.Time `db:"last_used_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

func (s *FormStore) CreateFormSchema(ctx context.Context, form *servicedomain.FormSchema) error {
	spec := mapFormSchemaToFormSpec(form)
	if err := s.formSpecStore.CreateFormSpec(ctx, spec); err != nil {
		return err
	}
	form.ID = spec.ID
	return nil
}

func (s *FormStore) GetFormSchema(ctx context.Context, formID string) (*servicedomain.FormSchema, error) {
	spec, err := s.formSpecStore.GetFormSpec(ctx, formID)
	if err != nil {
		return nil, err
	}
	return mapFormSpecToFormSchema(spec), nil
}

func (s *FormStore) GetFormSchemaBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSchema, error) {
	spec, err := s.formSpecStore.GetFormSpecBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, err
	}
	return mapFormSpecToFormSchema(spec), nil
}

func (s *FormStore) GetFormByCryptoID(ctx context.Context, cryptoID string) (*servicedomain.FormSchema, error) {
	spec, err := s.formSpecStore.GetFormSpecByPublicKey(ctx, cryptoID)
	if err != nil {
		return nil, err
	}
	return mapFormSpecToFormSchema(spec), nil
}

func (s *FormStore) GetFormBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSchema, error) {
	return s.GetFormSchemaBySlug(ctx, workspaceID, slug)
}

func (s *FormStore) UpdateFormSchema(ctx context.Context, form *servicedomain.FormSchema) error {
	return s.formSpecStore.UpdateFormSpec(ctx, mapFormSchemaToFormSpec(form))
}

func (s *FormStore) ListWorkspaceFormSchemas(ctx context.Context, workspaceID string) ([]*servicedomain.FormSchema, error) {
	specs, err := s.formSpecStore.ListWorkspaceFormSpecs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	forms := make([]*servicedomain.FormSchema, len(specs))
	for i := range specs {
		forms[i] = mapFormSpecToFormSchema(specs[i])
	}
	return forms, nil
}

func (s *FormStore) ListPublicForms(ctx context.Context) ([]*servicedomain.FormSchema, error) {
	var specs []models.FormSpec
	query := `SELECT * FROM core_service.form_specs WHERE is_public = TRUE AND deleted_at IS NULL ORDER BY name ASC`
	if err := s.db.Get(ctx).SelectContext(ctx, &specs, query); err != nil {
		return nil, TranslateSqlxError(err, "form_specs")
	}
	return mapFormSpecModelsToForms(specs), nil
}

func (s *FormStore) ListAllFormSchemas(ctx context.Context) ([]*servicedomain.FormSchema, error) {
	var specs []models.FormSpec
	query := `SELECT * FROM core_service.form_specs WHERE deleted_at IS NULL ORDER BY workspace_id, name`
	if err := s.db.Get(ctx).SelectContext(ctx, &specs, query); err != nil {
		return nil, TranslateSqlxError(err, "form_specs")
	}
	return mapFormSpecModelsToForms(specs), nil
}

func (s *FormStore) DeleteFormSchema(ctx context.Context, workspaceID, formID string) error {
	return s.formSpecStore.DeleteFormSpec(ctx, workspaceID, formID)
}

func (s *FormStore) CreateFormSubmission(ctx context.Context, submission *servicedomain.PublicFormSubmission) error {
	formSubmission := mapPublicFormSubmissionToFormSubmission(submission)
	if err := s.formSpecStore.CreateFormSubmission(ctx, formSubmission); err != nil {
		return err
	}
	submission.ID = formSubmission.ID
	return nil
}

func (s *FormStore) GetFormSubmission(ctx context.Context, submissionID string) (*servicedomain.PublicFormSubmission, error) {
	formSubmission, err := s.formSpecStore.GetFormSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	return mapFormSubmissionToPublicFormSubmission(formSubmission), nil
}

func (s *FormStore) UpdateFormSubmission(ctx context.Context, submission *servicedomain.PublicFormSubmission) error {
	return s.formSpecStore.UpdateFormSubmission(ctx, mapPublicFormSubmissionToFormSubmission(submission))
}

func (s *FormStore) ListFormSubmissions(ctx context.Context, formID string) ([]*servicedomain.PublicFormSubmission, error) {
	var modelsList []models.FormSubmission
	query := `SELECT * FROM core_service.form_submissions WHERE form_spec_id = ? ORDER BY created_at DESC, id DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, formID); err != nil {
		return nil, TranslateSqlxError(err, "form_submissions")
	}

	submissions := make([]*servicedomain.PublicFormSubmission, len(modelsList))
	for i := range modelsList {
		submissions[i] = mapFormSubmissionToPublicFormSubmission(mapFormSubmissionToDomain(&modelsList[i]))
	}
	return submissions, nil
}

func (s *FormStore) GetFormAnalytics(ctx context.Context, formID string) (*servicedomain.FormAnalytics, error) {
	var analytics servicedomain.FormAnalytics
	query := `
		SELECT
			COALESCE(COUNT(*), 0) AS total_submissions,
			COALESCE(COUNT(*) FILTER (WHERE jsonb_array_length(validation_errors_json) = 0), 0) AS valid_submissions,
			COALESCE(COUNT(*) FILTER (WHERE jsonb_array_length(validation_errors_json) > 0), 0) AS invalid_submissions,
			COALESCE(COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE), 0) AS submissions_today,
			COALESCE(COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '7 days'), 0) AS submissions_week,
			COALESCE(COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '30 days'), 0) AS submissions_month,
			COALESCE(MAX(updated_at), NOW()) AS last_updated
		FROM core_service.form_submissions
		WHERE form_spec_id = ?`

	var lastUpdated time.Time
	err := s.db.Get(ctx).QueryRowxContext(ctx, query, formID).Scan(
		&analytics.TotalSubmissions,
		&analytics.ValidSubmissions,
		&analytics.InvalidSubmissions,
		&analytics.SubmissionsToday,
		&analytics.SubmissionsWeek,
		&analytics.SubmissionsMonth,
		&lastUpdated,
	)
	if err != nil {
		return nil, TranslateSqlxError(err, "form_submissions")
	}

	analytics.FormID = formID
	analytics.SpamSubmissions = 0
	analytics.FieldCompletionRates = map[string]float64{}
	analytics.FieldErrorRates = map[string]float64{}
	analytics.ReferrerStats = map[string]int{}
	analytics.DeviceStats = map[string]int{}
	analytics.GeographicStats = map[string]int{}
	analytics.LastUpdated = lastUpdated
	if analytics.TotalSubmissions > 0 {
		analytics.ConversionRate = 1
	}
	return &analytics, nil
}

func (s *FormStore) CreateFormAPIToken(ctx context.Context, token *servicedomain.FormAPIToken) error {
	normalizePersistedUUID(&token.ID)
	query := `
		INSERT INTO core_service.form_access_tokens (
			id, workspace_id, form_spec_id, token, name, is_active, expires_at,
			allowed_hosts, last_used_at, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		token.ID,
		token.WorkspaceID,
		nullableUUIDValue(token.FormID),
		token.Token,
		token.Name,
		token.IsActive,
		token.ExpiresAt,
		pq.Array(token.AllowedHosts),
		token.LastUsedAt,
		token.CreatedAt,
		token.UpdatedAt,
	).Scan(&token.ID)
	return TranslateSqlxError(err, "form_access_tokens")
}

func (s *FormStore) GetFormAPIToken(ctx context.Context, tokenValue string) (*servicedomain.FormAPIToken, error) {
	var row formAccessTokenRow
	query := `SELECT * FROM core_service.form_access_tokens WHERE token = ? AND is_active = TRUE`
	if err := s.db.Get(ctx).GetContext(ctx, &row, query, tokenValue); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "form_access_tokens")
	}

	return &servicedomain.FormAPIToken{
		ID:           row.ID,
		WorkspaceID:  row.WorkspaceID,
		FormID:       row.FormSpecID,
		Token:        row.Token,
		Name:         row.Name,
		IsActive:     row.IsActive,
		ExpiresAt:    row.ExpiresAt,
		AllowedHosts: unmarshalStringArrayField(row.AllowedHosts, "form_access_tokens", "allowed_hosts"),
		LastUsedAt:   row.LastUsedAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func mapFormSchemaToFormSpec(form *servicedomain.FormSchema) *servicedomain.FormSpec {
	metadata := shareddomain.NewTypedSchema()
	metadata.Set("form_surface", formSurfaceConfig{
		UISchema:               form.UISchema,
		ValidationRules:        form.ValidationRules,
		RequiresAuth:           form.RequiresAuth,
		AllowMultiple:          form.AllowMultiple,
		CollectEmail:           form.CollectEmail,
		AutoCreateCase:         form.AutoCreateCase,
		AutoAssignTeamID:       form.AutoAssignTeamID,
		AutoAssignUserID:       form.AutoAssignUserID,
		AutoCasePriority:       form.AutoCasePriority,
		AutoCaseType:           form.AutoCaseType,
		AutoTags:               form.AutoTags,
		NotifyOnSubmission:     form.NotifyOnSubmission,
		NotificationEmails:     form.NotificationEmails,
		NotificationWebhookURL: form.NotificationWebhookURL,
		Theme:                  form.Theme,
		CustomCSS:              form.CustomCSS,
		SubmissionMessage:      form.SubmissionMessage,
		RedirectURL:            form.RedirectURL,
		AllowedDomains:         form.AllowedDomains,
		BlockedDomains:         form.BlockedDomains,
		RequiresCaptcha:        form.RequiresCaptcha,
		MaxSubmissionsPerDay:   form.MaxSubmissionsPerDay,
		AllowEmbed:             form.AllowEmbed,
		EmbedDomains:           form.EmbedDomains,
		TriggerRuleIDs:         form.TriggerRuleIDs,
		HasWorkflow:            form.HasWorkflow,
		WorkflowStates:         form.WorkflowStates,
		Transitions:            form.Transitions,
	})

	return &servicedomain.FormSpec{
		ID:                   form.ID,
		WorkspaceID:          form.WorkspaceID,
		Name:                 form.Name,
		Slug:                 form.Slug,
		PublicKey:            form.CryptoID,
		DescriptionMarkdown:  form.Description,
		FieldSpec:            form.SchemaData,
		SubmissionPolicy:     form.ValidationRules,
		SupportedChannels:    formSupportedChannels(form),
		IsPublic:             form.IsPublic,
		Status:               servicedomain.FormSpecStatus(form.Status),
		Metadata:             metadata,
		CreatedBy:            form.CreatedByID,
		CreatedAt:            form.CreatedAt,
		UpdatedAt:            form.UpdatedAt,
		DeletedAt:            form.DeletedAt,
		EvidenceRequirements: []shareddomain.TypedSchema{},
		InferenceRules:       []shareddomain.TypedSchema{},
		ApprovalPolicy:       shareddomain.NewTypedSchema(),
		DestinationPolicy:    shareddomain.NewTypedSchema(),
	}
}

func mapFormSpecToFormSchema(spec *servicedomain.FormSpec) *servicedomain.FormSchema {
	surface := decodeFormSurfaceConfig(spec.Metadata)
	form := &servicedomain.FormSchema{
		ID:                     spec.ID,
		WorkspaceID:            spec.WorkspaceID,
		Name:                   spec.Name,
		Description:            spec.DescriptionMarkdown,
		Slug:                   spec.Slug,
		SchemaData:             spec.FieldSpec,
		UISchema:               surface.UISchema,
		ValidationRules:        surface.ValidationRules,
		Status:                 servicedomain.FormStatus(spec.Status),
		IsPublic:               spec.IsPublic,
		RequiresAuth:           surface.RequiresAuth,
		AllowMultiple:          surface.AllowMultiple,
		CollectEmail:           surface.CollectEmail,
		AutoCreateCase:         surface.AutoCreateCase,
		AutoAssignTeamID:       surface.AutoAssignTeamID,
		AutoAssignUserID:       surface.AutoAssignUserID,
		AutoCasePriority:       surface.AutoCasePriority,
		AutoCaseType:           surface.AutoCaseType,
		AutoTags:               surface.AutoTags,
		NotifyOnSubmission:     surface.NotifyOnSubmission,
		NotificationEmails:     surface.NotificationEmails,
		NotificationWebhookURL: surface.NotificationWebhookURL,
		Theme:                  surface.Theme,
		CustomCSS:              surface.CustomCSS,
		SubmissionMessage:      surface.SubmissionMessage,
		RedirectURL:            surface.RedirectURL,
		AllowedDomains:         surface.AllowedDomains,
		BlockedDomains:         surface.BlockedDomains,
		RequiresCaptcha:        surface.RequiresCaptcha,
		MaxSubmissionsPerDay:   surface.MaxSubmissionsPerDay,
		AllowEmbed:             surface.AllowEmbed,
		EmbedDomains:           surface.EmbedDomains,
		CryptoID:               spec.PublicKey,
		TriggerRuleIDs:         surface.TriggerRuleIDs,
		HasWorkflow:            surface.HasWorkflow,
		WorkflowStates:         surface.WorkflowStates,
		Transitions:            surface.Transitions,
		CreatedByID:            spec.CreatedBy,
		CreatedAt:              spec.CreatedAt,
		UpdatedAt:              spec.UpdatedAt,
		DeletedAt:              spec.DeletedAt,
	}
	return form
}

func mapPublicFormSubmissionToFormSubmission(submission *servicedomain.PublicFormSubmission) *servicedomain.FormSubmission {
	metadata := shareddomain.NewTypedSchema()
	metadata.Set("form_submission", formSubmissionEnvelope{
		RawData:         submission.RawData,
		ProcessedData:   submission.ProcessedData.ToInterfaceMap(),
		SubmitterIP:     submission.SubmitterIP,
		UserAgent:       submission.UserAgent,
		Referrer:        submission.Referrer,
		ProcessingError: submission.ProcessingError,
		ProcessingNotes: submission.ProcessingNotes,
		IsValid:         submission.IsValid,
		SpamScore:       submission.SpamScore,
		IsSpam:          submission.IsSpam,
		SpamReasons:     submission.SpamReasons,
		ProcessedAt:     submission.ProcessedAt,
		ProcessedByID:   submission.ProcessedByID,
		ProcessingTime:  submission.ProcessingTime,
		AttachmentIDs:   submission.AttachmentIDs,
		CurrentStateID:  submission.CurrentStateID,
		StateHistory:    submission.StateHistory,
	})

	submittedAt := submission.CreatedAt
	return &servicedomain.FormSubmission{
		ID:               submission.ID,
		WorkspaceID:      submission.WorkspaceID,
		FormSpecID:     submission.FormID,
		CaseID:           submission.CaseID,
		ContactID:        submission.ContactID,
		Status:           servicedomain.FormSubmissionStatus(submission.Status),
		Channel:          "public_form",
		SubmitterEmail:   submission.SubmitterEmail,
		SubmitterName:    submission.SubmitterName,
		CompletionToken:  submission.CompletionToken,
		CollectedFields:  shareddomain.TypedSchemaFromMap(submission.Data.ToInterfaceMap()),
		MissingFields:    shareddomain.NewTypedSchema(),
		ValidationErrors: submission.ValidationErrors,
		Metadata:         metadata,
		Evidence:         []shareddomain.TypedSchema{},
		SubmittedAt:      &submittedAt,
		CreatedAt:        submission.CreatedAt,
		UpdatedAt:        submission.UpdatedAt,
	}
}

func mapFormSubmissionToPublicFormSubmission(submission *servicedomain.FormSubmission) *servicedomain.PublicFormSubmission {
	envelope := decodeFormSubmissionEnvelope(submission.Metadata)
	data := shareddomain.MetadataFromMap(submission.CollectedFields.ToMap())
	processedData := shareddomain.NewMetadata()
	if len(envelope.ProcessedData) > 0 {
		processedData = shareddomain.MetadataFromMap(envelope.ProcessedData)
	}

	rawData := envelope.RawData
	if rawData == "" {
		if bytes, err := json.Marshal(submission.CollectedFields.ToMap()); err == nil {
			rawData = string(bytes)
		}
	}

	return &servicedomain.PublicFormSubmission{
		ID:               submission.ID,
		WorkspaceID:      submission.WorkspaceID,
		FormID:           submission.FormSpecID,
		Data:             data,
		RawData:          rawData,
		ProcessedData:    processedData,
		SubmitterEmail:   submission.SubmitterEmail,
		SubmitterName:    submission.SubmitterName,
		SubmitterIP:      envelope.SubmitterIP,
		UserAgent:        envelope.UserAgent,
		Referrer:         envelope.Referrer,
		Status:           servicedomain.SubmissionStatus(submission.Status),
		ProcessingError:  envelope.ProcessingError,
		ProcessingNotes:  envelope.ProcessingNotes,
		CaseID:           submission.CaseID,
		ContactID:        submission.ContactID,
		IsValid:          envelope.IsValid || len(submission.ValidationErrors) == 0,
		ValidationErrors: submission.ValidationErrors,
		SpamScore:        envelope.SpamScore,
		IsSpam:           envelope.IsSpam,
		SpamReasons:      envelope.SpamReasons,
		ProcessedAt:      envelope.ProcessedAt,
		ProcessedByID:    envelope.ProcessedByID,
		ProcessingTime:   envelope.ProcessingTime,
		AttachmentIDs:    envelope.AttachmentIDs,
		CurrentStateID:   envelope.CurrentStateID,
		StateHistory:     envelope.StateHistory,
		CompletionToken:  submission.CompletionToken,
		CreatedAt:        submission.CreatedAt,
		UpdatedAt:        submission.UpdatedAt,
	}
}

func mapFormSpecModelsToForms(specs []models.FormSpec) []*servicedomain.FormSchema {
	forms := make([]*servicedomain.FormSchema, len(specs))
	for i := range specs {
		forms[i] = mapFormSpecToFormSchema(mapFormSpecToDomain(&specs[i]))
	}
	return forms
}

func decodeFormSurfaceConfig(metadata shareddomain.TypedSchema) formSurfaceConfig {
	var config formSurfaceConfig
	decodeTypedSchemaField(metadata, "form_surface", &config)
	return config
}

func decodeFormSubmissionEnvelope(metadata shareddomain.TypedSchema) formSubmissionEnvelope {
	var envelope formSubmissionEnvelope
	decodeTypedSchemaField(metadata, "form_submission", &envelope)
	return envelope
}

func decodeTypedSchemaField(schema shareddomain.TypedSchema, key string, target interface{}) {
	raw, ok := schema.Get(key)
	if !ok {
		return
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return
	}
	_ = json.Unmarshal(bytes, target)
}

func formSupportedChannels(form *servicedomain.FormSchema) []string {
	channels := []string{"operator_console", "agent"}
	if form.IsPublic || form.AllowEmbed {
		channels = append(channels, "public_web")
	}
	return channels
}
