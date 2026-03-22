package servicehandlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// MockStore implements a minimal shared.Store for testing
// It implements all required methods but only WithAdminContext does real work
type MockStore struct {
	mock.Mock
}

// Verify MockStore implements shared.Store
var _ shared.Store = (*MockStore)(nil)

func (m *MockStore) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *MockStore) WithAdminContext(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *MockStore) SetTenantContext(ctx context.Context, workspaceID string) error {
	return nil
}

func (m *MockStore) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockStore) Close() error {
	return nil
}

func (m *MockStore) GetSQLDB() (*sql.DB, error) {
	return nil, nil
}

// Stub methods for all sub-stores - these return nil since they aren't used in handler tests
func (m *MockStore) Users() shared.UserStore                                { return nil }
func (m *MockStore) Workspaces() shared.WorkspaceStore                      { return nil }
func (m *MockStore) Queues() shared.QueueStore                              { return nil }
func (m *MockStore) QueueItems() shared.QueueItemStore                      { return nil }
func (m *MockStore) Extensions() shared.ExtensionStore                      { return nil }
func (m *MockStore) ExtensionRuntime() shared.ExtensionRuntimeStore         { return nil }
func (m *MockStore) Cases() shared.CaseStore                                { return nil }
func (m *MockStore) Contacts() shared.ContactStore                          { return nil }
func (m *MockStore) EmailTemplates() shared.EmailTemplateStore              { return nil }
func (m *MockStore) OutboundEmails() shared.OutboundEmailStore              { return nil }
func (m *MockStore) InboundEmails() shared.InboundEmailStore                { return nil }
func (m *MockStore) EmailThreads() shared.EmailThreadStore                  { return nil }
func (m *MockStore) EmailSecurity() shared.EmailSecurityStore               { return nil }
func (m *MockStore) Projects() shared.ProjectStore                          { return nil }
func (m *MockStore) Issues() shared.IssueStore                              { return nil }
func (m *MockStore) ErrorEvents() shared.ErrorEventStore                    { return nil }
func (m *MockStore) IssueCaseIntegration() shared.IssueCaseIntegrationStore { return nil }
func (m *MockStore) ErrorAlerts() shared.ErrorAlertStore                    { return nil }
func (m *MockStore) Jobs() shared.JobStore                                  { return nil }
func (m *MockStore) ServiceCatalog() shared.ServiceCatalogStore             { return nil }
func (m *MockStore) Conversations() shared.ConversationStore                { return nil }
func (m *MockStore) FormSpecs() shared.FormSpecStore                             { return nil }
func (m *MockStore) ConceptSpecs() shared.ConceptSpecStore                  { return nil }
func (m *MockStore) KnowledgeResources() shared.KnowledgeResourceStore      { return nil }
func (m *MockStore) Sandboxes() shared.SandboxStore                         { return nil }
func (m *MockStore) Forms() shared.FormStore                                { return nil }
func (m *MockStore) Rules() shared.RuleStore                                { return nil }
func (m *MockStore) Outbox() shared.OutboxStore                             { return nil }
func (m *MockStore) Agents() shared.AgentStore                              { return nil }
func (m *MockStore) Idempotency() shared.IdempotencyStore                   { return nil }

// MockCaseService implements CaseServiceInterface for testing
type MockCaseService struct {
	mock.Mock
}

// Verify MockCaseService implements CaseServiceInterface
var _ CaseServiceInterface = (*MockCaseService)(nil)

func (m *MockCaseService) LinkIssueToCase(ctx context.Context, caseID, issueID, projectID string) error {
	args := m.Called(ctx, caseID, issueID, projectID)
	return args.Error(0)
}

func (m *MockCaseService) UnlinkIssueFromCase(ctx context.Context, caseID, issueID string) error {
	args := m.Called(ctx, caseID, issueID)
	return args.Error(0)
}

func (m *MockCaseService) CreateCaseForIssue(ctx context.Context, params observabilityservices.CreateCaseForIssueParams) (*servicedomain.Case, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*servicedomain.Case), args.Error(1)
}

func (m *MockCaseService) MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error {
	args := m.Called(ctx, caseID, resolvedAt)
	return args.Error(0)
}

func (m *MockCaseService) GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error) {
	args := m.Called(ctx, caseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*servicedomain.Case), args.Error(1)
}

func (m *MockCaseService) UpdateCase(ctx context.Context, c *servicedomain.Case) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

// MockContactService implements ContactServiceInterface for testing
type MockContactService struct {
	mock.Mock
}

// Verify MockContactService implements ContactServiceInterface
var _ ContactServiceInterface = (*MockContactService)(nil)

func (m *MockContactService) CreateContact(ctx context.Context, params platformservices.CreateContactParams) (*platformdomain.Contact, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*platformdomain.Contact), args.Error(1)
}

func TestNewCaseEventHandler(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, caseService, handler.caseService)
	assert.Equal(t, contactService, handler.contactService)
	assert.NotNil(t, handler.logger)
}

func TestHandleIssueCaseLinked_Success(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.IssueCaseLinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseLinked),
		IssueID:     "issue-1",
		CaseID:      "case-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		LinkedBy:    "user-1",
		LinkReason:  "related issue",
		LinkedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	caseService.On("LinkIssueToCase", mock.Anything, "case-1", "issue-1", "project-1").Return(nil)

	err = handler.HandleIssueCaseLinked(context.Background(), eventData)
	assert.NoError(t, err)

	caseService.AssertExpectations(t)
}

func TestHandleIssueCaseLinked_InvalidJSON(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	invalidJSON := []byte(`{invalid json}`)

	err := handler.HandleIssueCaseLinked(context.Background(), invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestHandleIssueCaseLinked_MissingLinkedBy(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	// Event without LinkedBy field should be silently ignored
	event := shareddomain.IssueCaseLinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseLinked),
		IssueID:     "issue-1",
		CaseID:      "case-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		LinkedBy:    "", // Empty LinkedBy
		LinkedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	err = handler.HandleIssueCaseLinked(context.Background(), eventData)
	assert.NoError(t, err)

	// Service should not be called
	caseService.AssertNotCalled(t, "LinkIssueToCase")
}

func TestHandleIssueCaseLinked_ServiceError(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.IssueCaseLinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseLinked),
		IssueID:     "issue-1",
		CaseID:      "case-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		LinkedBy:    "user-1",
		LinkedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	expectedErr := assert.AnError
	caseService.On("LinkIssueToCase", mock.Anything, "case-1", "issue-1", "project-1").Return(expectedErr)

	err = handler.HandleIssueCaseLinked(context.Background(), eventData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to link issue to case")

	caseService.AssertExpectations(t)
}

func TestHandleCaseEvent_IgnoresIssueLinkEvents(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	log := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, log)

	event := shareddomain.IssueCaseLinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseLinked),
		IssueID:     "issue-1",
		CaseID:      "case-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		LinkedBy:    "user-1",
		LinkedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	err = handler.HandleCaseEvent(context.Background(), eventData)
	require.NoError(t, err)
	caseService.AssertNotCalled(t, "LinkIssueToCase")
}

func TestHandleCaseEvent_IgnoresCaseCreatedForContact(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	log := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, log)

	event := shareddomain.CaseCreatedForContact{
		BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCaseCreatedForContact),
		ContactID:    "contact-1",
		ContactEmail: "person@example.com",
		IssueID:      "issue-1",
		ProjectID:    "project-1",
		WorkspaceID:  "workspace-1",
		IssueTitle:   "Broken checkout",
		IssueLevel:   "error",
		Priority:     string(servicedomain.CasePriorityHigh),
		CreatedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	err = handler.HandleCaseEvent(context.Background(), eventData)
	require.NoError(t, err)
	caseService.AssertNotCalled(t, "CreateCaseForIssue")
}

func TestHandleIssueLinkEvent_DispatchesIssueCaseLinked(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	log := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, log)

	event := shareddomain.IssueCaseLinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseLinked),
		IssueID:     "issue-1",
		CaseID:      "case-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		LinkedBy:    "user-1",
		LinkedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	caseService.On("LinkIssueToCase", mock.Anything, "case-1", "issue-1", "project-1").Return(nil)

	err = handler.HandleIssueLinkEvent(context.Background(), eventData)
	require.NoError(t, err)
	caseService.AssertExpectations(t)
}

func TestHandleErrorTrackingCaseEvent_DispatchesCaseCreatedForContact(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	log := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, log)

	event := shareddomain.CaseCreatedForContact{
		BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCaseCreatedForContact),
		ContactID:    "contact-1",
		ContactEmail: "person@example.com",
		IssueID:      "issue-1",
		ProjectID:    "project-1",
		WorkspaceID:  "workspace-1",
		IssueTitle:   "Broken checkout",
		IssueLevel:   "error",
		Priority:     string(servicedomain.CasePriorityHigh),
		CreatedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	caseService.On("CreateCaseForIssue", mock.Anything, mock.MatchedBy(func(params observabilityservices.CreateCaseForIssueParams) bool {
		return params.WorkspaceID == "workspace-1" &&
			params.IssueID == "issue-1" &&
			params.ProjectID == "project-1" &&
			params.ContactID == "contact-1" &&
			params.ContactEmail == "person@example.com"
	})).Return(func() *servicedomain.Case {
		c := servicedomain.NewCase("workspace-1", "Broken checkout", "person@example.com")
		return c
	}(), nil)

	err = handler.HandleErrorTrackingCaseEvent(context.Background(), eventData)
	require.NoError(t, err)
	caseService.AssertExpectations(t)
}

func TestHandleIssueCaseUnlinked_Success(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.IssueCaseUnlinked{
		IssueID:     "issue-1",
		CaseID:      "case-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		UnlinkedBy:  "user-1",
		UnlinkedAt:  time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	caseService.On("UnlinkIssueFromCase", mock.Anything, "case-1", "issue-1").Return(nil)

	err = handler.HandleIssueCaseUnlinked(context.Background(), eventData)
	assert.NoError(t, err)

	caseService.AssertExpectations(t)
}

func TestHandleIssueCaseUnlinked_MissingUnlinkedBy(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.IssueCaseUnlinked{
		IssueID:     "issue-1",
		CaseID:      "case-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		UnlinkedBy:  "", // Empty UnlinkedBy
		UnlinkedAt:  time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	err = handler.HandleIssueCaseUnlinked(context.Background(), eventData)
	assert.NoError(t, err)

	// Service should not be called
	caseService.AssertNotCalled(t, "UnlinkIssueFromCase")
}

func TestHandleIssueCaseUnlinked_InvalidJSON(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	invalidJSON := []byte(`{not valid json`)

	err := handler.HandleIssueCaseUnlinked(context.Background(), invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestHandleCaseCreatedForContact_Success(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.CaseCreatedForContact{
		ContactID:    "contact-1",
		ContactEmail: "user@example.com",
		IssueID:      "issue-1",
		ProjectID:    "project-1",
		WorkspaceID:  "workspace-1",
		IssueTitle:   "Application Error",
		IssueLevel:   "error",
		Priority:     "high",
		CreatedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	expectedCase := &servicedomain.Case{
		CaseIdentity: servicedomain.CaseIdentity{
			ID:          "case-1",
			WorkspaceID: "workspace-1",
		},
		Subject: "Application Error",
	}

	caseService.On("CreateCaseForIssue", mock.Anything, mock.MatchedBy(func(params observabilityservices.CreateCaseForIssueParams) bool {
		return params.WorkspaceID == "workspace-1" &&
			params.IssueID == "issue-1" &&
			params.ContactEmail == "user@example.com" &&
			params.Priority == servicedomain.CasePriorityHigh
	})).Return(expectedCase, nil)

	err = handler.HandleCaseCreatedForContact(context.Background(), eventData)
	assert.NoError(t, err)

	caseService.AssertExpectations(t)
}

func TestHandleCaseCreatedForContact_MissingEmail(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.CaseCreatedForContact{
		ContactID:    "contact-1",
		ContactEmail: "", // Empty email
		IssueID:      "issue-1",
		ProjectID:    "project-1",
		WorkspaceID:  "workspace-1",
		IssueTitle:   "Application Error",
		IssueLevel:   "error",
		Priority:     "high",
		CreatedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	err = handler.HandleCaseCreatedForContact(context.Background(), eventData)
	assert.NoError(t, err)

	// Service should not be called
	caseService.AssertNotCalled(t, "CreateCaseForIssue")
}

func TestHandleCaseCreatedForContact_InvalidJSON(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	invalidJSON := []byte(`{"invalid": json`)

	err := handler.HandleCaseCreatedForContact(context.Background(), invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestHandleCaseCreatedForContact_ServiceError(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.CaseCreatedForContact{
		ContactID:    "contact-1",
		ContactEmail: "user@example.com",
		IssueID:      "issue-1",
		ProjectID:    "project-1",
		WorkspaceID:  "workspace-1",
		IssueTitle:   "Application Error",
		IssueLevel:   "error",
		Priority:     "high",
		CreatedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	expectedErr := assert.AnError
	caseService.On("CreateCaseForIssue", mock.Anything, mock.Anything).Return(nil, expectedErr)

	err = handler.HandleCaseCreatedForContact(context.Background(), eventData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create case for issue")

	caseService.AssertExpectations(t)
}

func TestHandleCasesBulkResolved_Success(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	// Use Truncate to remove monotonic clock (which is lost during JSON serialization)
	resolvedAt := time.Now().Truncate(time.Millisecond)
	event := shareddomain.CasesBulkResolved{
		IssueID:     "issue-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		CaseIDs:     []string{"case-1", "case-2", "case-3"},
		Resolution:  "fixed in v1.2.3",
		ResolvedAt:  resolvedAt,
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	// Mock case retrieval and updates for all cases
	for _, caseID := range []string{"case-1", "case-2", "case-3"} {
		mockCase := &servicedomain.Case{
			CaseIdentity: servicedomain.CaseIdentity{
				ID:          caseID,
				WorkspaceID: "workspace-1",
			},
		}
		caseService.On("MarkCaseResolved", mock.Anything, caseID, mock.AnythingOfType("time.Time")).Return(nil)
		caseService.On("GetCase", mock.Anything, caseID).Return(mockCase, nil)
		caseService.On("UpdateCase", mock.Anything, mockCase).Return(nil)
	}

	err = handler.HandleCasesBulkResolved(context.Background(), eventData)
	assert.NoError(t, err)

	caseService.AssertExpectations(t)
}

func TestHandleCasesBulkResolved_NoCases(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.CasesBulkResolved{
		IssueID:     "issue-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		Resolution:  "fixed",
		ResolvedAt:  time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	err = handler.HandleCasesBulkResolved(context.Background(), eventData)
	assert.NoError(t, err)

	// No service calls should be made
	caseService.AssertNotCalled(t, "MarkCaseResolved")
}

func TestHandleCasesBulkResolved_InvalidJSON(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	invalidJSON := []byte(`not json at all`)

	err := handler.HandleCasesBulkResolved(context.Background(), invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestHandleCasesBulkResolved_PartialFailure(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	resolvedAt := time.Now().Truncate(time.Millisecond)
	event := shareddomain.CasesBulkResolved{
		IssueID:     "issue-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		CaseIDs:     []string{"case-1", "case-2"},
		Resolution:  "fixed",
		ResolvedAt:  resolvedAt,
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	// First case succeeds
	mockCase1 := &servicedomain.Case{CaseIdentity: servicedomain.CaseIdentity{ID: "case-1", WorkspaceID: "workspace-1"}}
	caseService.On("MarkCaseResolved", mock.Anything, "case-1", mock.AnythingOfType("time.Time")).Return(nil)
	caseService.On("GetCase", mock.Anything, "case-1").Return(mockCase1, nil)
	caseService.On("UpdateCase", mock.Anything, mockCase1).Return(nil)

	// Second case fails
	caseService.On("MarkCaseResolved", mock.Anything, "case-2", mock.AnythingOfType("time.Time")).Return(assert.AnError)

	// Should not fail the entire batch
	err = handler.HandleCasesBulkResolved(context.Background(), eventData)
	assert.NoError(t, err)

	caseService.AssertExpectations(t)
}

func TestHandleContactCreatedFromEmail_Success(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.ContactCreatedFromEmail{
		ContactID:    "contact-1",
		WorkspaceID:  "workspace-1",
		EmailID:      "email-1",
		Email:        "user@example.com",
		Name:         "John Doe",
		Organization: "ACME Corp",
		CreatedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	expectedContact := &platformdomain.Contact{
		ID:    "contact-1",
		Email: "user@example.com",
	}

	contactService.On("CreateContact", mock.Anything, mock.MatchedBy(func(params platformservices.CreateContactParams) bool {
		return params.WorkspaceID == "workspace-1" &&
			params.Email == "user@example.com" &&
			params.Name == "John Doe" &&
			params.Company == "ACME Corp" &&
			params.Source == "email"
	})).Return(expectedContact, nil)

	err = handler.HandleContactCreatedFromEmail(context.Background(), eventData)
	assert.NoError(t, err)

	contactService.AssertExpectations(t)
}

func TestHandleContactCreatedFromEmail_MissingEmail(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.ContactCreatedFromEmail{
		ContactID:    "contact-1",
		WorkspaceID:  "workspace-1",
		EmailID:      "email-1",
		Email:        "", // Empty email
		Name:         "John Doe",
		Organization: "ACME Corp",
		CreatedAt:    time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	err = handler.HandleContactCreatedFromEmail(context.Background(), eventData)
	assert.NoError(t, err)

	// Service should not be called
	contactService.AssertNotCalled(t, "CreateContact")
}

func TestHandleContactCreatedFromEmail_InvalidJSON(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	invalidJSON := []byte(`{broken json`)

	err := handler.HandleContactCreatedFromEmail(context.Background(), invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestHandleContactCreatedFromEmail_ServiceError(t *testing.T) {
	caseService := &MockCaseService{}
	contactService := &MockContactService{}
	store := &MockStore{}
	logger := logger.New()

	handler := NewCaseEventHandler(caseService, contactService, nil, store, logger)

	event := shareddomain.ContactCreatedFromEmail{
		ContactID:   "contact-1",
		WorkspaceID: "workspace-1",
		EmailID:     "email-1",
		Email:       "user@example.com",
		Name:        "John Doe",
		CreatedAt:   time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	expectedErr := assert.AnError
	contactService.On("CreateContact", mock.Anything, mock.Anything).Return(nil, expectedErr)

	err = handler.HandleContactCreatedFromEmail(context.Background(), eventData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create contact")

	contactService.AssertExpectations(t)
}

func TestEventHandlerMiddleware(t *testing.T) {
	logger := logger.New()
	ctx := context.Background()

	callCount := 0
	testHandler := func(ctx context.Context, data []byte) error {
		callCount++
		return nil
	}

	middleware := EventHandlerMiddleware(logger, testHandler)

	err := middleware(ctx, []byte(`{"test": "data"}`))
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestEventHandlerMiddleware_Error(t *testing.T) {
	logger := logger.New()
	ctx := context.Background()

	expectedErr := assert.AnError
	testHandler := func(ctx context.Context, data []byte) error {
		return expectedErr
	}

	middleware := EventHandlerMiddleware(logger, testHandler)

	err := middleware(ctx, []byte(`{"test": "data"}`))
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}
