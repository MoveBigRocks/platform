package platformdomain

import (
	"strings"
	"testing"
	"time"
)

func TestAgentLifecycleAndPrincipalHelpers(t *testing.T) {
	user := &User{ID: "user_1", Name: "Ada"}
	if user.GetID() != "user_1" || user.GetName() != "Ada" || user.GetPrincipalType() != PrincipalTypeUser {
		t.Fatalf("unexpected user principal helpers: %#v", user)
	}

	agent := NewAgent("ws_1", "triage-bot", "Handles triage", "owner_1", "creator_1")
	if agent.GetID() != "" || agent.GetName() != "triage-bot" || agent.GetPrincipalType() != PrincipalTypeAgent {
		t.Fatalf("unexpected agent principal helpers: %#v", agent)
	}
	if !agent.IsActive() {
		t.Fatal("expected new agent to be active")
	}

	agent.Suspend("maintenance")
	if agent.Status != AgentStatusSuspended || agent.StatusReason != "maintenance" {
		t.Fatalf("expected suspended agent, got %#v", agent)
	}

	agent.Activate()
	if agent.Status != AgentStatusActive || agent.StatusReason != "" {
		t.Fatalf("expected active agent, got %#v", agent)
	}

	agent.Revoke("retired")
	if agent.Status != AgentStatusRevoked || agent.StatusReason != "retired" || agent.IsActive() {
		t.Fatalf("expected revoked agent, got %#v", agent)
	}
}

func TestAgentTokenLifecycle(t *testing.T) {
	plaintext, hash, prefix, err := GenerateAgentToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if !strings.HasPrefix(plaintext, "hat_") {
		t.Fatalf("expected Move Big Rocks token prefix, got %q", plaintext)
	}
	if HashAgentToken(plaintext) != hash {
		t.Fatalf("expected hash helper to match generated hash")
	}
	if prefix == "" || !strings.HasPrefix(plaintext, prefix) {
		t.Fatalf("expected prefix %q to be derived from plaintext %q", prefix, plaintext)
	}

	expiresAt := time.Now().Add(time.Hour)
	token, shownPlaintext, err := NewAgentToken("agent_1", "CLI", "creator_1", &expiresAt)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if shownPlaintext == "" || token.AgentID != "agent_1" || token.Name != "CLI" || !token.IsValid() {
		t.Fatalf("unexpected token state: %#v plaintext=%q", token, shownPlaintext)
	}

	token.RecordUsage("127.0.0.1")
	if token.UseCount != 1 || token.LastUsedAt == nil || token.LastUsedIP != "127.0.0.1" {
		t.Fatalf("expected usage tracking to update token, got %#v", token)
	}

	token.Revoke("owner_1")
	if token.IsValid() || token.RevokedAt == nil || token.RevokedByID == nil || *token.RevokedByID != "owner_1" {
		t.Fatalf("expected token to be revoked, got %#v", token)
	}
}

func TestAuthContextHelpers(t *testing.T) {
	role := InstanceRoleAdmin
	user := &User{ID: "user_1", Name: "Ada"}
	ctx := &AuthContext{
		Principal:     user,
		PrincipalType: PrincipalTypeUser,
		WorkspaceID:   "ws_2",
		WorkspaceIDs:  []string{"ws_1", "ws_2", "ws_1"},
		Permissions:   []string{PermissionCaseRead, PermissionContactWrite},
		InstanceRole:  &role,
	}

	ids := ctx.WorkspaceIDSet()
	if len(ids) != 2 || ids[0] != "ws_1" || ids[1] != "ws_2" {
		t.Fatalf("expected sorted unique workspace IDs, got %#v", ids)
	}
	if !ctx.IsHuman() || ctx.IsAgent() {
		t.Fatalf("expected human auth context, got %#v", ctx)
	}
	if !ctx.HasWorkspaceAccess("ws_1") || !ctx.IsInstanceAdmin() || !ctx.CanManageUsers() || !ctx.CanAccessInstancePanel() {
		t.Fatalf("expected admin auth context helpers to be true, got %#v", ctx)
	}
	if !ctx.HasPermission(PermissionCaseRead) || !ctx.HasResourcePermission("contact", "write") {
		t.Fatalf("expected permission helpers to be true, got %#v", ctx)
	}
	if ctx.GetHuman() != user || ctx.GetAgent() != nil {
		t.Fatalf("unexpected principal extraction from auth context: %#v", ctx)
	}

	agent := NewAgent("ws_1", "agent", "", "owner_1", "creator_1")
	agentCtx := &AuthContext{
		Principal:     agent,
		PrincipalType: PrincipalTypeAgent,
		WorkspaceIDs:  []string{"ws_1"},
		Membership: &WorkspaceMembership{
			Constraints: MembershipConstraints{
				AllowedTeamIDs:          []string{"team_support", "team_billing"},
				AllowDelegatedRouting:   true,
				DelegatedRoutingTeamIDs: []string{"team_billing"},
			},
		},
	}
	if !agentCtx.IsAgent() || agentCtx.IsHuman() || agentCtx.GetAgent() != agent || agentCtx.GetHuman() != nil {
		t.Fatalf("unexpected agent auth context helpers: %#v", agentCtx)
	}
	if agentCtx.IsInstanceAdmin() || agentCtx.CanManageUsers() || agentCtx.CanAccessInstancePanel() {
		t.Fatalf("agent context should not have instance admin capabilities: %#v", agentCtx)
	}
	if !agentCtx.AllowsDelegatedRouting() || !agentCtx.CanDelegateRoutingToTeam("team_billing") || agentCtx.CanDelegateRoutingToTeam("team_support") {
		t.Fatalf("expected delegated routing scope to apply, got %#v", agentCtx)
	}
}

func TestAuthContextWorkspaceAccessEdgeCases(t *testing.T) {
	var nilContext *AuthContext
	if nilContext.HasWorkspaceAccess("ws_1") {
		t.Fatal("nil auth context should not have workspace access")
	}

	role := InstanceRoleOperator
	adminCtx := &AuthContext{
		Principal:     &User{ID: "user_1"},
		PrincipalType: PrincipalTypeUser,
		InstanceRole:  &role,
	}
	if !adminCtx.HasWorkspaceAccess("any-workspace") {
		t.Fatal("instance admin should have workspace access")
	}

	workspaceCtx := &AuthContext{
		PrincipalType: PrincipalTypeUser,
		WorkspaceID:   "ws_2",
		WorkspaceIDs:  []string{"ws_1", "ws_2"},
	}
	if workspaceCtx.HasWorkspaceAccess("") {
		t.Fatal("empty workspace id should not be allowed")
	}
	if !workspaceCtx.HasWorkspaceAccess("ws_1") {
		t.Fatal("expected explicit workspace membership to grant access")
	}
	if workspaceCtx.HasWorkspaceAccess("ws_3") {
		t.Fatal("unexpected access to unrelated workspace")
	}
}
