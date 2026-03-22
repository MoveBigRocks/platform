package resolvers

import (
	"testing"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

func TestValidateDelegatedRouting(t *testing.T) {
	agent := platformdomain.NewAgent("ws_1", "Router", "", "user_1", "admin_1")
	authCtx := &platformdomain.AuthContext{
		Principal:     agent,
		PrincipalType: platformdomain.PrincipalTypeAgent,
		Membership: &platformdomain.WorkspaceMembership{
			Constraints: platformdomain.MembershipConstraints{
				AllowedTeamIDs:          []string{"team_support", "team_billing"},
				AllowDelegatedRouting:   true,
				DelegatedRoutingTeamIDs: []string{"team_billing"},
			},
		},
	}

	if err := validateDelegatedRouting(authCtx, "team_billing"); err != nil {
		t.Fatalf("expected delegated routing to billing to be allowed, got %v", err)
	}
	if err := validateDelegatedRouting(authCtx, "team_support"); err == nil {
		t.Fatal("expected delegated routing outside explicit target scope to fail")
	}
}

func TestValidateDelegatedRoutingRejectsAgentWithoutPolicy(t *testing.T) {
	agent := platformdomain.NewAgent("ws_1", "Router", "", "user_1", "admin_1")
	authCtx := &platformdomain.AuthContext{
		Principal:     agent,
		PrincipalType: platformdomain.PrincipalTypeAgent,
		Membership:    &platformdomain.WorkspaceMembership{},
	}

	if err := validateDelegatedRouting(authCtx, "team_support"); err == nil {
		t.Fatal("expected delegated routing without membership policy to fail")
	}
}

func TestValidateSourceTeamAccess(t *testing.T) {
	authCtx := &platformdomain.AuthContext{
		PrincipalType: platformdomain.PrincipalTypeUser,
		Membership: &platformdomain.WorkspaceMembership{
			Constraints: platformdomain.MembershipConstraints{
				AllowedTeamIDs: []string{"team_support"},
			},
		},
	}

	if err := validateSourceTeamAccess(authCtx, "team_support", "not found"); err != nil {
		t.Fatalf("expected source team access to succeed, got %v", err)
	}
	if err := validateSourceTeamAccess(authCtx, "team_billing", "not found"); err == nil {
		t.Fatal("expected source team access restriction to fail")
	}
}
