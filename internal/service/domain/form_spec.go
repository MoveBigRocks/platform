package servicedomain

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

type FormSpecStatus string

const (
	FormSpecStatusDraft    FormSpecStatus = "draft"
	FormSpecStatusActive   FormSpecStatus = "active"
	FormSpecStatusArchived FormSpecStatus = "archived"
)

type FormSubmissionStatus string

const (
	FormSubmissionStatusDraft     FormSubmissionStatus = "draft"
	FormSubmissionStatusSubmitted FormSubmissionStatus = "submitted"
	FormSubmissionStatusAccepted  FormSubmissionStatus = "accepted"
	FormSubmissionStatusRejected  FormSubmissionStatus = "rejected"
)

type FormSpec struct {
	ID          string
	WorkspaceID string

	Name                string
	Slug                string
	PublicKey           string
	DescriptionMarkdown string

	FieldSpec            shareddomain.TypedSchema
	EvidenceRequirements []shareddomain.TypedSchema
	InferenceRules       []shareddomain.TypedSchema
	ApprovalPolicy       shareddomain.TypedSchema
	SubmissionPolicy     shareddomain.TypedSchema
	DestinationPolicy    shareddomain.TypedSchema

	SupportedChannels []string
	IsPublic          bool
	Status            FormSpecStatus
	Metadata          shareddomain.TypedSchema

	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewFormSpec(workspaceID, slug, name string) *FormSpec {
	now := time.Now().UTC()
	return &FormSpec{
		WorkspaceID:          workspaceID,
		Name:                 strings.TrimSpace(name),
		Slug:                 NormalizeFormSpecSlug(slug, name),
		FieldSpec:            shareddomain.NewTypedSchema(),
		EvidenceRequirements: []shareddomain.TypedSchema{},
		InferenceRules:       []shareddomain.TypedSchema{},
		ApprovalPolicy:       shareddomain.NewTypedSchema(),
		SubmissionPolicy:     shareddomain.NewTypedSchema(),
		DestinationPolicy:    shareddomain.NewTypedSchema(),
		SupportedChannels:    []string{},
		Status:               FormSpecStatusDraft,
		Metadata:             shareddomain.NewTypedSchema(),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func (s *FormSpec) Validate() error {
	if strings.TrimSpace(s.WorkspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(s.Slug) == "" {
		return fmt.Errorf("slug is required")
	}
	if s.Slug != NormalizeFormSpecSlug(s.Slug, "") {
		return fmt.Errorf("slug must contain only lowercase letters, numbers, and hyphens")
	}
	switch s.Status {
	case "", FormSpecStatusDraft, FormSpecStatusActive, FormSpecStatusArchived:
	default:
		return fmt.Errorf("status must be one of: draft, active, archived")
	}
	return nil
}

func (s *FormSpec) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	s.Name = name
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *FormSpec) SetSlug(slug string) error {
	normalized := NormalizeFormSpecSlug(slug, s.Name)
	if normalized == "" {
		return fmt.Errorf("slug is required")
	}
	s.Slug = normalized
	s.UpdatedAt = time.Now().UTC()
	return nil
}

type FormSubmission struct {
	ID          string
	WorkspaceID string

	FormSpecID            string
	ConversationSessionID string
	CaseID                string
	ContactID             string

	Status          FormSubmissionStatus
	Channel         string
	SubmitterEmail  string
	SubmitterName   string
	CompletionToken string

	CollectedFields  shareddomain.TypedSchema
	MissingFields    shareddomain.TypedSchema
	Evidence         []shareddomain.TypedSchema
	ValidationErrors []string
	Metadata         shareddomain.TypedSchema

	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewFormSubmission(workspaceID, formSpecID string) *FormSubmission {
	now := time.Now().UTC()
	return &FormSubmission{
		WorkspaceID:      workspaceID,
		FormSpecID:       formSpecID,
		Status:           FormSubmissionStatusDraft,
		Channel:          "operator_console",
		CollectedFields:  shareddomain.NewTypedSchema(),
		MissingFields:    shareddomain.NewTypedSchema(),
		Evidence:         []shareddomain.TypedSchema{},
		ValidationErrors: []string{},
		Metadata:         shareddomain.NewTypedSchema(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func (s *FormSubmission) Validate() error {
	if strings.TrimSpace(s.WorkspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(s.FormSpecID) == "" {
		return fmt.Errorf("form_spec_id is required")
	}
	if strings.TrimSpace(s.Channel) == "" {
		return fmt.Errorf("channel is required")
	}
	switch s.Status {
	case "", FormSubmissionStatusDraft, FormSubmissionStatusSubmitted, FormSubmissionStatusAccepted, FormSubmissionStatusRejected:
	default:
		return fmt.Errorf("status must be one of: draft, submitted, accepted, rejected")
	}
	return nil
}

type FormSubmissionFilter struct {
	FormSpecID            string
	ConversationSessionID string
	CaseID                string
	ContactID             string
	Status                FormSubmissionStatus
	Limit                 int
	Offset                int
}

func NormalizeFormSpecSlug(slug, fallbackName string) string {
	source := strings.TrimSpace(slug)
	if source == "" {
		source = strings.TrimSpace(fallbackName)
	}
	source = strings.ToLower(source)

	var b strings.Builder
	lastDash := false
	for _, r := range source {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "form"
	}
	return result
}
