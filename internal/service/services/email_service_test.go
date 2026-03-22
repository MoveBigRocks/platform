package serviceapp

import (
	"context"
	"fmt"
	"testing"

	emaildom "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/testutil"

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
