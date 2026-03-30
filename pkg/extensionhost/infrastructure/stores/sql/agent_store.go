package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

// AgentStore implements shared.AgentStore using sqlx
type AgentStore struct {
	db *SqlxDB
}

// NewAgentStore creates a new agent store
func NewAgentStore(db *SqlxDB) *AgentStore {
	return &AgentStore{db: db}
}

// =============================================================================
// Agent Models
// =============================================================================

// AgentModel is the sqlx model for agents
type AgentModel struct {
	ID           string     `db:"id"`
	WorkspaceID  string     `db:"workspace_id"`
	Name         string     `db:"name"`
	Description  string     `db:"description"`
	OwnerID      string     `db:"owner_id"`
	Status       string     `db:"status"`
	StatusReason string     `db:"status_reason"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	CreatedByID  string     `db:"created_by_id"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

func (AgentModel) TableName() string {
	return "agents"
}

func (m *AgentModel) ToDomain() *platformdomain.Agent {
	return &platformdomain.Agent{
		ID:           m.ID,
		WorkspaceID:  m.WorkspaceID,
		Name:         m.Name,
		Description:  m.Description,
		OwnerID:      m.OwnerID,
		Status:       platformdomain.AgentStatus(m.Status),
		StatusReason: m.StatusReason,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		CreatedByID:  m.CreatedByID,
		DeletedAt:    m.DeletedAt,
	}
}

// AgentTokenModel is the sqlx model for agent tokens
type AgentTokenModel struct {
	ID          string     `db:"id"`
	AgentID     string     `db:"agent_id"`
	TokenHash   string     `db:"token_hash"`
	TokenPrefix string     `db:"token_prefix"`
	Name        string     `db:"name"`
	ExpiresAt   *time.Time `db:"expires_at"`
	RevokedAt   *time.Time `db:"revoked_at"`
	RevokedByID *string    `db:"revoked_by_id"`
	LastUsedAt  *time.Time `db:"last_used_at"`
	LastUsedIP  string     `db:"last_used_ip"`
	UseCount    int64      `db:"use_count"`
	CreatedAt   time.Time  `db:"created_at"`
	CreatedByID string     `db:"created_by_id"`
}

func (AgentTokenModel) TableName() string {
	return "agent_tokens"
}

func (m *AgentTokenModel) ToDomain() *platformdomain.AgentToken {
	return &platformdomain.AgentToken{
		ID:          m.ID,
		AgentID:     m.AgentID,
		TokenHash:   m.TokenHash,
		TokenPrefix: m.TokenPrefix,
		Name:        m.Name,
		ExpiresAt:   m.ExpiresAt,
		RevokedAt:   m.RevokedAt,
		RevokedByID: m.RevokedByID,
		LastUsedAt:  m.LastUsedAt,
		LastUsedIP:  m.LastUsedIP,
		UseCount:    m.UseCount,
		CreatedAt:   m.CreatedAt,
		CreatedByID: m.CreatedByID,
	}
}

// WorkspaceMembershipModel is the sqlx model for workspace memberships
type WorkspaceMembershipModel struct {
	ID            string     `db:"id"`
	WorkspaceID   string     `db:"workspace_id"`
	PrincipalID   string     `db:"principal_id"`
	PrincipalType string     `db:"principal_type"`
	Role          string     `db:"role"`
	Permissions   []byte     `db:"permissions"`
	Constraints   []byte     `db:"constraints"`
	GrantedByID   string     `db:"granted_by_id"`
	GrantedAt     time.Time  `db:"granted_at"`
	ExpiresAt     *time.Time `db:"expires_at"`
	RevokedAt     *time.Time `db:"revoked_at"`
	RevokedByID   *string    `db:"revoked_by_id"`
}

func (WorkspaceMembershipModel) TableName() string {
	return "workspace_memberships"
}

func (m *WorkspaceMembershipModel) ToDomain() *platformdomain.WorkspaceMembership {
	var permissions []string
	unmarshalJSONBytes(m.Permissions, &permissions, "workspace_memberships", "permissions")

	var constraints platformdomain.MembershipConstraints
	unmarshalJSONBytes(m.Constraints, &constraints, "workspace_memberships", "constraints")

	return &platformdomain.WorkspaceMembership{
		ID:            m.ID,
		WorkspaceID:   m.WorkspaceID,
		PrincipalID:   m.PrincipalID,
		PrincipalType: platformdomain.PrincipalType(m.PrincipalType),
		Role:          m.Role,
		Permissions:   permissions,
		Constraints:   constraints,
		GrantedByID:   m.GrantedByID,
		GrantedAt:     m.GrantedAt,
		ExpiresAt:     m.ExpiresAt,
		RevokedAt:     m.RevokedAt,
		RevokedByID:   m.RevokedByID,
	}
}

// =============================================================================
// Agent CRUD
// =============================================================================

func (s *AgentStore) CreateAgent(ctx context.Context, agent *platformdomain.Agent) error {
	normalizePersistedUUID(&agent.ID)
	query := `
		INSERT INTO core_identity.agents (
			id, workspace_id, name, description, owner_id, status, status_reason,
			created_at, updated_at, created_by_id, deleted_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		agent.ID, agent.WorkspaceID, agent.Name, agent.Description, agent.OwnerID,
		string(agent.Status), agent.StatusReason,
		agent.CreatedAt, agent.UpdatedAt, agent.CreatedByID, agent.DeletedAt,
	).Scan(&agent.ID)
	return TranslateSqlxError(err, "agents")
}

func (s *AgentStore) GetAgentByID(ctx context.Context, agentID string) (*platformdomain.Agent, error) {
	var model AgentModel
	query := `SELECT * FROM core_identity.agents WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &model, query, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "agents")
	}
	return model.ToDomain(), nil
}

func (s *AgentStore) GetAgentByName(ctx context.Context, workspaceID, name string) (*platformdomain.Agent, error) {
	var model AgentModel
	query := `SELECT * FROM core_identity.agents WHERE workspace_id = ? AND name = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "agents")
	}
	return model.ToDomain(), nil
}

func (s *AgentStore) ListAgents(ctx context.Context, workspaceID string) ([]*platformdomain.Agent, error) {
	var models []AgentModel
	query := `SELECT * FROM core_identity.agents WHERE workspace_id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).SelectContext(ctx, &models, query, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "agents")
	}

	agents := make([]*platformdomain.Agent, len(models))
	for i, m := range models {
		agents[i] = m.ToDomain()
	}
	return agents, nil
}

func (s *AgentStore) UpdateAgent(ctx context.Context, agent *platformdomain.Agent) error {
	query := `
		UPDATE core_identity.agents SET
			name = ?, description = ?, owner_id = ?, status = ?, status_reason = ?,
			updated_at = ?, deleted_at = ?
		WHERE id = ?`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		agent.Name, agent.Description, agent.OwnerID,
		string(agent.Status), agent.StatusReason, agent.UpdatedAt, agent.DeletedAt,
		agent.ID,
	)
	if err != nil {
		return TranslateSqlxError(err, "agents")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *AgentStore) DeleteAgent(ctx context.Context, agentID string) error {
	query := `UPDATE core_identity.agents SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), agentID)
	if err != nil {
		return TranslateSqlxError(err, "agents")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

// =============================================================================
// Agent Token Operations
// =============================================================================

func (s *AgentStore) CreateAgentToken(ctx context.Context, token *platformdomain.AgentToken) error {
	normalizePersistedUUID(&token.ID)
	query := `
		INSERT INTO core_identity.agent_tokens (
			id, agent_id, token_hash, token_prefix, name, expires_at,
			revoked_at, revoked_by_id, last_used_at, last_used_ip,
			use_count, created_at, created_by_id
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		token.ID, token.AgentID, token.TokenHash, token.TokenPrefix, token.Name,
		token.ExpiresAt, token.RevokedAt, token.RevokedByID, token.LastUsedAt,
		token.LastUsedIP, token.UseCount, token.CreatedAt, token.CreatedByID,
	).Scan(&token.ID)
	return TranslateSqlxError(err, "agent_tokens")
}

func (s *AgentStore) GetAgentTokenByHash(ctx context.Context, tokenHash string) (*platformdomain.AgentToken, error) {
	var model AgentTokenModel
	query := `SELECT * FROM core_identity.agent_tokens WHERE token_hash = ?`
	err := s.db.Get(ctx).GetContext(ctx, &model, query, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "agent_tokens")
	}
	return model.ToDomain(), nil
}

func (s *AgentStore) GetAgentTokenByID(ctx context.Context, tokenID string) (*platformdomain.AgentToken, error) {
	var model AgentTokenModel
	query := `SELECT * FROM core_identity.agent_tokens WHERE id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &model, query, tokenID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "agent_tokens")
	}
	return model.ToDomain(), nil
}

func (s *AgentStore) ListAgentTokens(ctx context.Context, agentID string) ([]*platformdomain.AgentToken, error) {
	var models []AgentTokenModel
	query := `SELECT * FROM core_identity.agent_tokens WHERE agent_id = ? AND revoked_at IS NULL`
	err := s.db.Get(ctx).SelectContext(ctx, &models, query, agentID)
	if err != nil {
		return nil, TranslateSqlxError(err, "agent_tokens")
	}

	tokens := make([]*platformdomain.AgentToken, len(models))
	for i, m := range models {
		tokens[i] = m.ToDomain()
	}
	return tokens, nil
}

func (s *AgentStore) UpdateAgentTokenUsage(ctx context.Context, tokenID, ip string) error {
	query := `UPDATE core_identity.agent_tokens SET last_used_at = ?, last_used_ip = ?, use_count = use_count + 1 WHERE id = ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), ip, tokenID)
	return TranslateSqlxError(err, "agent_tokens")
}

func (s *AgentStore) RevokeAgentToken(ctx context.Context, tokenID, revokedByID string) error {
	query := `UPDATE core_identity.agent_tokens SET revoked_at = ?, revoked_by_id = ? WHERE id = ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), revokedByID, tokenID)
	return TranslateSqlxError(err, "agent_tokens")
}

// =============================================================================
// Workspace Membership Operations
// =============================================================================

func (s *AgentStore) CreateWorkspaceMembership(ctx context.Context, membership *platformdomain.WorkspaceMembership) error {
	normalizePersistedUUID(&membership.ID)
	permissions, _ := json.Marshal(membership.Permissions)
	constraints, _ := json.Marshal(membership.Constraints)

	query := `
		INSERT INTO core_identity.workspace_memberships (
			id, workspace_id, principal_id, principal_type, role,
			permissions, constraints, granted_by_id, granted_at,
			expires_at, revoked_at, revoked_by_id
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		membership.ID, membership.WorkspaceID, membership.PrincipalID,
		string(membership.PrincipalType), membership.Role, permissions, constraints,
		membership.GrantedByID, membership.GrantedAt,
		membership.ExpiresAt, membership.RevokedAt, membership.RevokedByID,
	).Scan(&membership.ID)
	return TranslateSqlxError(err, "workspace_memberships")
}

func (s *AgentStore) GetWorkspaceMembership(ctx context.Context, workspaceID, principalID string, principalType platformdomain.PrincipalType) (*platformdomain.WorkspaceMembership, error) {
	var model WorkspaceMembershipModel
	query := `SELECT * FROM core_identity.workspace_memberships WHERE workspace_id = ? AND principal_id = ? AND principal_type = ?`
	err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, principalID, string(principalType))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "workspace_memberships")
	}
	return model.ToDomain(), nil
}

func (s *AgentStore) GetWorkspaceMembershipByID(ctx context.Context, membershipID string) (*platformdomain.WorkspaceMembership, error) {
	var model WorkspaceMembershipModel
	query := `SELECT * FROM core_identity.workspace_memberships WHERE id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &model, query, membershipID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "workspace_memberships")
	}
	return model.ToDomain(), nil
}

func (s *AgentStore) RevokeWorkspaceMembership(ctx context.Context, membershipID, revokedByID string) error {
	query := `UPDATE core_identity.workspace_memberships SET revoked_at = ?, revoked_by_id = ? WHERE id = ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), revokedByID, membershipID)
	return TranslateSqlxError(err, "workspace_memberships")
}
