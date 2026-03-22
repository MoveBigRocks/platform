package observabilityservices

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

type CreateCaseForIssueParams struct {
	WorkspaceID  string
	IssueID      string
	ProjectID    string
	IssueTitle   string
	IssueLevel   string
	Priority     servicedomain.CasePriority
	ContactID    string
	ContactEmail string
}

type issueCaseWriter interface {
	CreateCase(ctx context.Context, params serviceapp.CreateCaseParams) (*servicedomain.Case, error)
	UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error
	LinkIssueToCase(ctx context.Context, caseID, issueID, projectID string) error
	UnlinkIssueFromCase(ctx context.Context, caseID, issueID string) error
	MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error
	GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error)
}

// IssueCaseService owns the issue-to-case helper behavior for the error-tracking pack.
// It wraps generic case primitives instead of embedding that behavior in the shared support service.
type IssueCaseService struct {
	caseStore shared.CaseStore
	writer    issueCaseWriter
	logger    *logger.Logger
}

func NewIssueCaseService(caseStore shared.CaseStore, writer issueCaseWriter) *IssueCaseService {
	return &IssueCaseService{
		caseStore: caseStore,
		writer:    writer,
		logger:    logger.New().WithField("service", "issue-case"),
	}
}

func (s *IssueCaseService) LinkIssueToCase(ctx context.Context, caseID, issueID, projectID string) error {
	return s.writer.LinkIssueToCase(ctx, caseID, issueID, projectID)
}

func (s *IssueCaseService) UnlinkIssueFromCase(ctx context.Context, caseID, issueID string) error {
	return s.writer.UnlinkIssueFromCase(ctx, caseID, issueID)
}

func (s *IssueCaseService) MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error {
	return s.writer.MarkCaseResolved(ctx, caseID, resolvedAt)
}

func (s *IssueCaseService) GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error) {
	return s.writer.GetCase(ctx, caseID)
}

func (s *IssueCaseService) UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error {
	return s.writer.UpdateCase(ctx, caseObj)
}

func (s *IssueCaseService) CreateCaseForIssue(ctx context.Context, params CreateCaseForIssueParams) (*servicedomain.Case, error) {
	if s == nil || s.writer == nil || s.caseStore == nil {
		return nil, fmt.Errorf("issue case service is not configured")
	}

	if params.ContactID != "" {
		existing, err := s.caseStore.GetCaseByIssueAndContact(ctx, params.WorkspaceID, params.IssueID, params.ContactID)
		if err == nil && existing != nil {
			s.logger.WithFields(map[string]interface{}{
				"case_id":    existing.ID,
				"issue_id":   params.IssueID,
				"contact_id": params.ContactID,
			}).Debug("Returning existing case for issue/contact (idempotency)")
			return existing, nil
		}
	}

	customFields := shareddomain.NewTypedCustomFields()
	customFields.SetString("linked_issue_id", params.IssueID)
	customFields.SetString("linked_project_id", params.ProjectID)
	customFields.SetString("issue_level", params.IssueLevel)
	customFields.SetString("source", "auto_monitoring")
	customFields.SetBool("auto_created", true)

	caseObj, err := s.writer.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  params.WorkspaceID,
		Subject:      formatIssueSubject(params.IssueTitle),
		Description:  formatIssueDescription(params.IssueTitle, params.IssueLevel),
		Priority:     params.Priority,
		Channel:      servicedomain.CaseChannelInternal,
		ContactID:    params.ContactID,
		ContactEmail: params.ContactEmail,
		CustomFields: customFields,
	})
	if err != nil {
		return nil, err
	}

	if err := s.writer.LinkIssueToCase(ctx, caseObj.ID, params.IssueID, params.ProjectID); err != nil {
		s.logger.Warn("Failed to link issue to case", "case_id", caseObj.ID, "issue_id", params.IssueID, "error", err)
		return caseObj, nil
	}

	stored, err := s.writer.GetCase(ctx, caseObj.ID)
	if err != nil {
		s.logger.Warn("Failed to reload case after issue link", "case_id", caseObj.ID, "error", err)
		return caseObj, nil
	}
	stored.MarkAsAutoCreated("auto_monitoring", params.IssueID)
	if err := s.writer.UpdateCase(ctx, stored); err != nil {
		s.logger.Warn("Failed to persist issue link metadata", "case_id", caseObj.ID, "error", err)
		return stored, nil
	}
	return stored, nil
}

func formatIssueSubject(issueTitle string) string {
	return fmt.Sprintf("Error affecting you: %s", strings.TrimSpace(issueTitle))
}

func formatIssueDescription(issueTitle, _ string) string {
	return fmt.Sprintf("We've detected an error that may be affecting your experience: %s", strings.TrimSpace(issueTitle))
}
