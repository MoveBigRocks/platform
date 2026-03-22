package platformdomain

import (
	"testing"
	"time"
)

func TestWorkspaceMembershipLifecycleAndConstraints(t *testing.T) {
	membership := NewWorkspaceMembership(
		"ws_1",
		"user_1",
		PrincipalTypeUser,
		"owner",
		[]string{PermissionCaseRead, PermissionContactWrite},
		"admin_1",
	)
	if !membership.IsActive() || !membership.HasPermission(PermissionCaseRead) || !membership.HasResourcePermission("contact", "write") {
		t.Fatalf("expected membership helpers to pass, got %#v", membership)
	}

	ratePerMinute := 60
	start := "09:00"
	membership.Constraints.RateLimitPerMinute = &ratePerMinute
	membership.Constraints.ActiveHoursStart = &start
	membership.Constraints.AllowedIPs = []string{"127.0.0.1"}
	membership.Constraints.AllowedProjectIDs = []string{"proj_1"}
	membership.Constraints.AllowDelegatedRouting = true
	membership.Constraints.DelegatedRoutingTeamIDs = []string{"team_1"}
	if !membership.IsRateLimited() || !membership.HasRateLimit() || !membership.HasTimeRestrictions() || !membership.IsIPRestricted() || !membership.IsProjectRestricted() {
		t.Fatalf("expected constraints helpers to pass, got %#v", membership.Constraints)
	}
	if !membership.CanAccessProject("proj_1") || membership.CanAccessProject("proj_2") {
		t.Fatalf("expected project restriction helper to apply, got %#v", membership.Constraints)
	}
	if !membership.AllowsDelegatedRouting() || !membership.CanDelegateRoutingToTeam("team_1") || membership.CanDelegateRoutingToTeam("team_2") {
		t.Fatalf("expected delegated routing helper to apply, got %#v", membership.Constraints)
	}

	membership.Revoke("admin_2")
	if membership.IsActive() || membership.RevokedAt == nil || membership.RevokedByID == nil || *membership.RevokedByID != "admin_2" {
		t.Fatalf("expected revoked membership, got %#v", membership)
	}

	expired := NewWorkspaceMembership("ws_1", "user_2", PrincipalTypeUser, "member", nil, "admin_1")
	expiresAt := time.Now().Add(-time.Hour)
	expired.ExpiresAt = &expiresAt
	if expired.IsActive() {
		t.Fatalf("expected expired membership to be inactive, got %#v", expired)
	}

	agentMembership := NewAgentMembership("ws_1", "agent_1", "agent", []string{PermissionCaseWrite}, "admin_1")
	if agentMembership.PrincipalType != PrincipalTypeAgent || agentMembership.PrincipalID != "agent_1" {
		t.Fatalf("expected agent membership defaults, got %#v", agentMembership)
	}
}
