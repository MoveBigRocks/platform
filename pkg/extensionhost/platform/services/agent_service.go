package platformservices

import (
	"context"

	apierrors "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/errors"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// AgentService handles all agent-related business logic
type AgentService struct {
	agentStore shared.AgentStore
	logger     *logger.Logger
}

// NewAgentService creates a new agent service
func NewAgentService(agentStore shared.AgentStore) *AgentService {
	return &AgentService{
		agentStore: agentStore,
		logger:     logger.New().WithField("service", "agent"),
	}
}

// GetAgent retrieves an agent by ID
func (s *AgentService) GetAgent(ctx context.Context, agentID string) (*platformdomain.Agent, error) {
	if agentID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("agent_id", "required"))
	}

	agent, err := s.agentStore.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, apierrors.NotFoundError("agent", agentID)
	}
	return agent, nil
}

// GetAgentByName retrieves an agent by workspace and name
func (s *AgentService) GetAgentByName(ctx context.Context, workspaceID, name string) (*platformdomain.Agent, error) {
	if workspaceID == "" || name == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("name", "workspace_id and name required"))
	}

	agent, err := s.agentStore.GetAgentByName(ctx, workspaceID, name)
	if err != nil {
		return nil, apierrors.NotFoundError("agent", name)
	}
	return agent, nil
}

// ListAgents lists all agents for a workspace
func (s *AgentService) ListAgents(ctx context.Context, workspaceID string) ([]*platformdomain.Agent, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	return s.agentStore.ListAgents(ctx, workspaceID)
}

// CreateAgent creates a new agent
func (s *AgentService) CreateAgent(ctx context.Context, agent *platformdomain.Agent) error {
	if agent.WorkspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if agent.Name == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("name", "required"))
	}

	if err := s.agentStore.CreateAgent(ctx, agent); err != nil {
		return apierrors.DatabaseError("create agent", err)
	}
	return nil
}

// UpdateAgent updates an existing agent
func (s *AgentService) UpdateAgent(ctx context.Context, agent *platformdomain.Agent) error {
	if agent.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}

	if err := s.agentStore.UpdateAgent(ctx, agent); err != nil {
		return apierrors.DatabaseError("update agent", err)
	}
	return nil
}

// DeleteAgent deletes an agent
func (s *AgentService) DeleteAgent(ctx context.Context, agentID string) error {
	if agentID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("agent_id", "required"))
	}

	if err := s.agentStore.DeleteAgent(ctx, agentID); err != nil {
		return apierrors.DatabaseError("delete agent", err)
	}
	return nil
}

// CreateAgentToken creates a new token for an agent
func (s *AgentService) CreateAgentToken(ctx context.Context, token *platformdomain.AgentToken) error {
	if token.AgentID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("agent_id", "required"))
	}

	if err := s.agentStore.CreateAgentToken(ctx, token); err != nil {
		return apierrors.DatabaseError("create agent token", err)
	}
	return nil
}

// GetAgentTokenByHash retrieves a token by its hash
func (s *AgentService) GetAgentTokenByHash(ctx context.Context, tokenHash string) (*platformdomain.AgentToken, error) {
	if tokenHash == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("token_hash", "required"))
	}

	token, err := s.agentStore.GetAgentTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, apierrors.NotFoundError("agent_token", "")
	}
	return token, nil
}

// GetAgentToken retrieves a token by ID.
func (s *AgentService) GetAgentToken(ctx context.Context, tokenID string) (*platformdomain.AgentToken, error) {
	if tokenID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("token_id", "required"))
	}

	token, err := s.agentStore.GetAgentTokenByID(ctx, tokenID)
	if err != nil {
		return nil, apierrors.NotFoundError("agent_token", tokenID)
	}
	return token, nil
}

// ListAgentTokens lists all tokens for an agent
func (s *AgentService) ListAgentTokens(ctx context.Context, agentID string) ([]*platformdomain.AgentToken, error) {
	if agentID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("agent_id", "required"))
	}
	return s.agentStore.ListAgentTokens(ctx, agentID)
}

// RevokeAgentToken revokes an agent token
func (s *AgentService) RevokeAgentToken(ctx context.Context, tokenID, revokedByID string) error {
	if tokenID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("token_id", "required"))
	}

	if err := s.agentStore.RevokeAgentToken(ctx, tokenID, revokedByID); err != nil {
		return apierrors.DatabaseError("revoke agent token", err)
	}
	return nil
}

// GetWorkspaceMembership retrieves a workspace membership for a principal.
func (s *AgentService) GetWorkspaceMembership(ctx context.Context, workspaceID, principalID string, principalType platformdomain.PrincipalType) (*platformdomain.WorkspaceMembership, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if principalID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("principal_id", "required"))
	}

	membership, err := s.agentStore.GetWorkspaceMembership(ctx, workspaceID, principalID, principalType)
	if err != nil {
		return nil, apierrors.NotFoundError("workspace_membership", principalID)
	}
	return membership, nil
}

// GetWorkspaceMembershipByID retrieves a workspace membership by ID.
func (s *AgentService) GetWorkspaceMembershipByID(ctx context.Context, membershipID string) (*platformdomain.WorkspaceMembership, error) {
	if membershipID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("membership_id", "required"))
	}

	membership, err := s.agentStore.GetWorkspaceMembershipByID(ctx, membershipID)
	if err != nil {
		return nil, apierrors.NotFoundError("workspace_membership", membershipID)
	}
	return membership, nil
}

// GrantWorkspaceMembership creates a workspace membership.
func (s *AgentService) GrantWorkspaceMembership(ctx context.Context, membership *platformdomain.WorkspaceMembership) error {
	if membership == nil {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("membership", "required"))
	}
	if membership.WorkspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if membership.PrincipalID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("principal_id", "required"))
	}

	if err := s.agentStore.CreateWorkspaceMembership(ctx, membership); err != nil {
		return apierrors.DatabaseError("create workspace membership", err)
	}
	return nil
}

// RevokeWorkspaceMembership revokes a workspace membership.
func (s *AgentService) RevokeWorkspaceMembership(ctx context.Context, membershipID, revokedByID string) error {
	if membershipID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("membership_id", "required"))
	}

	if err := s.agentStore.RevokeWorkspaceMembership(ctx, membershipID, revokedByID); err != nil {
		return apierrors.DatabaseError("revoke workspace membership", err)
	}
	return nil
}
