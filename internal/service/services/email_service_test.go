package serviceapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	emaildom "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/id"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEmailProvider struct{}

func (stubEmailProvider) SendEmail(ctx context.Context, email *emaildom.OutboundEmail) error {
	return nil
}

func (stubEmailProvider) ParseInboundEmail(ctx context.Context, rawEmail []byte) (*emaildom.InboundEmail, error) {
	return nil, nil
}

func (stubEmailProvider) ValidateConfig() error {
	return nil
}

func TestEmailProviderRegistry_NewProvider(t *testing.T) {
	registry := NewEmailProviderRegistry()

	provider, err := registry.NewProvider(EmailConfig{Provider: "mock"})
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, err = registry.NewProvider(EmailConfig{Provider: "not-a-provider"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported email backend")
}

func TestNewEmailServiceWithRegistry_UsesInjectedRegistry(t *testing.T) {
	registry := &EmailProviderRegistry{factories: map[string]EmailProviderFactory{
		"custom": func(config EmailConfig) (EmailProvider, error) {
			return stubEmailProvider{}, nil
		},
	}}

	service, err := NewEmailServiceWithRegistry(nil, EmailConfig{Provider: "custom"}, nil, registry)
	require.NoError(t, err)
	require.NotNil(t, service)

	_, err = NewEmailServiceWithRegistry(nil, EmailConfig{Provider: "missing"}, nil, registry)
	require.Error(t, err)
}

func TestEmailProviderRegistry_Register(t *testing.T) {
	registry := &EmailProviderRegistry{factories: map[string]EmailProviderFactory{}}
	registry.Register("custom", func(config EmailConfig) (EmailProvider, error) {
		return stubEmailProvider{}, nil
	})

	provider, err := registry.NewProvider(EmailConfig{Provider: "custom"})
	require.NoError(t, err)
	require.NotNil(t, provider)

	registry.Register("broken", func(config EmailConfig) (EmailProvider, error) {
		return nil, fmt.Errorf("boom")
	})
	_, err = registry.NewProvider(EmailConfig{Provider: "broken"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestEmailServiceCreateInboundEmailWithTenantContextSetsWorkspaceOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service, err := NewEmailService(store, EmailConfig{Provider: "mock"}, nil)
	require.NoError(t, err)

	inbound := emaildom.NewInboundEmail("", "<tenant-aware@example.com>", "customer@example.com", "Subject", "Body")
	inbound.ToEmails = []string{"support@example.com"}

	require.NoError(t, service.CreateInboundEmailWithTenantContext(ctx, workspace.ID, inbound))
	require.Equal(t, workspace.ID, inbound.WorkspaceID)

	stored, err := store.InboundEmails().GetInboundEmail(ctx, inbound.ID)
	require.NoError(t, err)
	require.Equal(t, workspace.ID, stored.WorkspaceID)
}

func TestEmailService_ProcessInboundEmailCreatesCaseAndCommunication(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := setupTestWorkspace(t, store, "email-process-new")
	caseService := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil, WithTransactionRunner(store))
	service, err := NewEmailService(store, EmailConfig{Provider: "mock"}, caseService)
	require.NoError(t, err)

	inbound := emaildom.NewInboundEmail(workspaceID, "<new-thread@example.com>", "customer@example.com", "Need help with my order", "I need an update on my order.")
	inbound.FromName = "Casey Customer"
	inbound.HTMLContent = "<p>I need an update on my order.</p>"
	inbound.ToEmails = []string{"support@example.com"}

	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, inbound))

	processed, err := service.ProcessInboundEmail(ctx, inbound.ID)
	require.NoError(t, err)

	require.Equal(t, emaildom.EmailProcessingStatusProcessed, processed.ProcessingStatus)
	require.NotNil(t, processed.ProcessedAt)
	require.NotEmpty(t, processed.CaseID)
	require.NotEmpty(t, processed.CommunicationID)
	assert.True(t, processed.IsThreadStart)

	caseObj, err := store.Cases().GetCase(ctx, processed.CaseID)
	require.NoError(t, err)
	assert.Equal(t, workspaceID, caseObj.WorkspaceID)
	assert.Equal(t, inbound.Subject, caseObj.Subject)
	assert.Equal(t, inbound.FromEmail, caseObj.ContactEmail)
	assert.Equal(t, inbound.FromName, caseObj.ContactName)
	assert.Equal(t, emaildom.CaseChannelEmail, caseObj.Channel)
	assert.Equal(t, 1, caseObj.MessageCount)
	assert.Equal(t, emaildom.CaseStatusNew, caseObj.Status)

	comms, err := store.Cases().ListCaseCommunications(ctx, caseObj.ID)
	require.NoError(t, err)
	require.Len(t, comms, 1)
	assert.Equal(t, processed.CommunicationID, comms[0].ID)
	assert.Equal(t, shareddomain.DirectionInbound, comms[0].Direction)
	assert.Equal(t, shareddomain.CommTypeEmail, comms[0].Type)
	assert.Equal(t, inbound.MessageID, comms[0].MessageID)
	assert.Equal(t, inbound.TextContent, comms[0].Body)
	assert.Equal(t, inbound.HTMLContent, comms[0].BodyHTML)
}

func TestEmailService_ProcessInboundEmailReopensOrOpensExistingCases(t *testing.T) {
	testCases := []struct {
		name        string
		initial     emaildom.CaseStatus
		setupStatus func(t *testing.T, ctx context.Context, svc *CaseService, caseID string)
		wantStatus  emaildom.CaseStatus
		wantReopen  int
	}{
		{
			name:    "pending to open",
			initial: emaildom.CaseStatusPending,
			setupStatus: func(t *testing.T, ctx context.Context, svc *CaseService, caseID string) {
				require.NoError(t, svc.SetCaseStatus(ctx, caseID, emaildom.CaseStatusPending))
			},
			wantStatus: emaildom.CaseStatusOpen,
			wantReopen: 0,
		},
		{
			name:    "resolved to open",
			initial: emaildom.CaseStatusResolved,
			setupStatus: func(t *testing.T, ctx context.Context, svc *CaseService, caseID string) {
				require.NoError(t, svc.MarkCaseResolved(ctx, caseID, time.Now().UTC()))
			},
			wantStatus: emaildom.CaseStatusOpen,
			wantReopen: 1,
		},
		{
			name:    "closed to open",
			initial: emaildom.CaseStatusClosed,
			setupStatus: func(t *testing.T, ctx context.Context, svc *CaseService, caseID string) {
				require.NoError(t, svc.MarkCaseResolved(ctx, caseID, time.Now().UTC()))
				require.NoError(t, svc.CloseCase(ctx, caseID))
			},
			wantStatus: emaildom.CaseStatusOpen,
			wantReopen: 1,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()

			ctx := context.Background()
			workspaceID := setupTestWorkspace(t, store, "email-process-existing")
			caseService := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil, WithTransactionRunner(store))
			service, err := NewEmailService(store, EmailConfig{Provider: "mock"}, caseService)
			require.NoError(t, err)

			caseObj, err := caseService.CreateCase(ctx, CreateCaseParams{
				WorkspaceID:  workspaceID,
				Subject:      "Billing follow-up",
				ContactEmail: "customer@example.com",
				ContactName:  "Casey Customer",
				Channel:      emaildom.CaseChannelEmail,
			})
			require.NoError(t, err)
			tt.setupStatus(t, ctx, caseService, caseObj.ID)

			anchor := emaildom.NewCommunication(caseObj.ID, workspaceID, shareddomain.CommTypeEmail, "Prior agent response")
			anchor.Direction = shareddomain.DirectionOutbound
			anchor.IsInternal = false
			anchor.FromUserID = id.New()
			anchor.FromEmail = "agent@example.com"
			anchor.ToEmails = []string{"customer@example.com"}
			anchor.Subject = "Re: Billing follow-up"
			anchor.MessageID = "<anchor@example.com>"
			require.NoError(t, caseService.AddCommunication(ctx, anchor))

			inbound := emaildom.NewInboundEmail(workspaceID, "<customer-reply@example.com>", "customer@example.com", "Re: Billing follow-up", "I still need help with this.")
			inbound.FromName = "Casey Customer"
			inbound.InReplyTo = anchor.MessageID
			inbound.References = []string{anchor.MessageID}
			inbound.ToEmails = []string{"support@example.com"}
			require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, inbound))

			processed, err := service.ProcessInboundEmail(ctx, inbound.ID)
			require.NoError(t, err)
			assert.Equal(t, caseObj.ID, processed.CaseID)
			assert.Equal(t, emaildom.EmailProcessingStatusProcessed, processed.ProcessingStatus)
			assert.False(t, processed.IsThreadStart)

			updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, updatedCase.Status)
			assert.Equal(t, tt.wantReopen, updatedCase.ReopenCount)
			assert.Equal(t, 1, updatedCase.MessageCount)

			comms, err := store.Cases().ListCaseCommunications(ctx, caseObj.ID)
			require.NoError(t, err)
			require.Len(t, comms, 2)
			reply := comms[1]
			assert.Equal(t, shareddomain.DirectionInbound, reply.Direction)
			assert.Equal(t, inbound.MessageID, reply.MessageID)
			assert.Equal(t, inbound.InReplyTo, reply.InReplyTo)
		})
	}
}

func TestEmailService_ProcessInboundEmailMatchesReplySubjectByContact(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := setupTestWorkspace(t, store, "email-process-subject-match")
	caseService := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil, WithTransactionRunner(store))
	service, err := NewEmailService(store, EmailConfig{Provider: "mock"}, caseService)
	require.NoError(t, err)

	matchingCase, err := caseService.CreateCase(ctx, CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Refund request",
		ContactEmail: "customer@example.com",
		ContactName:  "Casey Customer",
		Channel:      emaildom.CaseChannelEmail,
	})
	require.NoError(t, err)
	require.NoError(t, caseService.SetCaseStatus(ctx, matchingCase.ID, emaildom.CaseStatusPending))

	otherCase, err := caseService.CreateCase(ctx, CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Refund request",
		ContactEmail: "someone-else@example.com",
		ContactName:  "Another Customer",
		Channel:      emaildom.CaseChannelEmail,
	})
	require.NoError(t, err)

	inbound := emaildom.NewInboundEmail(workspaceID, "<subject-match@example.com>", "customer@example.com", "Re: Refund request", "Any update on my refund?")
	inbound.FromName = "Casey Customer"
	inbound.ToEmails = []string{"support@example.com"}
	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, inbound))

	processed, err := service.ProcessInboundEmail(ctx, inbound.ID)
	require.NoError(t, err)
	assert.Equal(t, matchingCase.ID, processed.CaseID)
	assert.NotEqual(t, otherCase.ID, processed.CaseID)

	updatedCase, err := store.Cases().GetCase(ctx, matchingCase.ID)
	require.NoError(t, err)
	assert.Equal(t, emaildom.CaseStatusOpen, updatedCase.Status)
}

func TestEmailService_ProcessInboundEmailDoesNotSubjectMatchFreshEmail(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := setupTestWorkspace(t, store, "email-process-no-subject-match")
	caseService := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil, WithTransactionRunner(store))
	service, err := NewEmailService(store, EmailConfig{Provider: "mock"}, caseService)
	require.NoError(t, err)

	existingCase, err := caseService.CreateCase(ctx, CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Subscription issue",
		ContactEmail: "customer@example.com",
		Channel:      emaildom.CaseChannelEmail,
	})
	require.NoError(t, err)

	inbound := emaildom.NewInboundEmail(workspaceID, "<fresh-thread@example.com>", "customer@example.com", "Subscription issue", "This is a brand new thread with the same subject.")
	inbound.ToEmails = []string{"support@example.com"}
	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, inbound))

	processed, err := service.ProcessInboundEmail(ctx, inbound.ID)
	require.NoError(t, err)
	assert.NotEqual(t, existingCase.ID, processed.CaseID)

	newCase, err := store.Cases().GetCase(ctx, processed.CaseID)
	require.NoError(t, err)
	assert.Equal(t, inbound.Subject, newCase.Subject)
	assert.Equal(t, inbound.FromEmail, newCase.ContactEmail)
}
