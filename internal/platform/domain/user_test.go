package platformdomain

import (
	"testing"
	"time"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shared "github.com/movebigrocks/platform/internal/shared/domain"

	"github.com/stretchr/testify/require"
)

func TestInstanceRoleNormalizationAndPermissions(t *testing.T) {
	require.Equal(t, InstanceRoleAdmin, CanonicalizeInstanceRole(" ADMIN "))
	require.True(t, IsValidInstanceRole(InstanceRoleOperator))
	require.False(t, IsValidInstanceRole("owner"))

	super := InstanceRoleSuperAdmin
	admin := InstanceRoleAdmin
	operator := InstanceRoleOperator

	require.True(t, super.IsSuperAdmin())
	require.True(t, admin.IsAdmin())
	require.True(t, operator.IsOperator())
}

func TestNewUserDefaults(t *testing.T) {
	user := NewUser("user@example.com", "User")
	require.Equal(t, "user@example.com", user.Email)
	require.Equal(t, "User", user.Name)
	require.True(t, user.IsActive)
	require.False(t, user.CreatedAt.IsZero())
	require.False(t, user.UpdatedAt.IsZero())
}

func TestUserAdminAndWorkspaceAccess(t *testing.T) {
	role := InstanceRole(" ADMIN ")
	user := &User{
		ID:            "user_1",
		Email:         "admin@example.com",
		IsActive:      true,
		EmailVerified: true,
		InstanceRole:  &role,
	}

	user.CanonicalizeRole()
	require.NotNil(t, user.InstanceRole)
	require.Equal(t, InstanceRoleAdmin, *user.InstanceRole)
	require.True(t, user.CanAccessAdminPanel())
	require.True(t, user.CanManageUsers())
	require.True(t, user.IsInstanceAdmin())

	roles := []*UserWorkspaceRole{
		{UserID: "user_1", WorkspaceID: "ws_1", Role: WorkspaceRoleOwner},
		{UserID: "user_1", WorkspaceID: "ws_2", Role: WorkspaceRoleViewer, RevokedAt: ptr(time.Now())},
	}

	require.True(t, user.HasWorkspaceRole("ws_1", WorkspaceRoleOwner, roles))
	require.False(t, user.HasWorkspaceRole("ws_2", WorkspaceRoleViewer, roles))
	require.Equal(t, WorkspaceRoleOwner, *user.GetWorkspaceRole("ws_1", roles))
	require.Equal(t, []string{"ws_1"}, user.GetWorkspaces(roles))
	require.True(t, user.CanAccessWorkspace("ws_1", roles))
	require.True(t, user.CanAccessWorkspace("ws_any", roles))

	lockedUntil := time.Now().Add(time.Hour)
	user.LockedUntil = &lockedUntil
	require.True(t, user.IsLocked())
	require.False(t, user.CanAccessAdminPanel())

	invalid := InstanceRole("unknown")
	user.InstanceRole = &invalid
	user.CanonicalizeRole()
	require.Nil(t, user.InstanceRole)
}

func TestSessionAndWorkspaceSettingsHelpers(t *testing.T) {
	workspaceID := "ws_1"
	session := &Session{
		ExpiresAt:      time.Now().Add(time.Hour),
		LastActivityAt: time.Now().Add(-2 * time.Hour),
		CurrentContext: Context{Type: ContextTypeWorkspace, WorkspaceID: &workspaceID},
		AvailableContexts: []Context{
			{Type: ContextTypeWorkspace, WorkspaceID: &workspaceID},
			{Type: ContextTypeInstance},
		},
	}

	require.True(t, session.IsValid())
	require.True(t, session.IsIdle(time.Hour))
	require.True(t, session.IsWorkspaceContext())
	require.False(t, session.IsInstanceContext())
	require.Equal(t, &workspaceID, session.GetCurrentWorkspaceID())
	require.True(t, session.HasInstanceAccess())

	session.UpdateActivity()
	require.False(t, session.IsIdle(time.Hour))

	settings := NewWorkspaceSettings("ws_1")
	require.Equal(t, "ws_1", settings.WorkspaceID)

	settings.UpdateSetting("workspace_name", shared.StringValue("Acme"), "user_1")
	settings.UpdateSetting("default_sla_hours", shared.IntValue(12), "user_1")
	settings.UpdateSetting("portal_enabled", shared.BoolValue(true), "user_1")
	require.Equal(t, "Acme", settings.WorkspaceName)
	require.Equal(t, 12, settings.DefaultSLAHours)
	require.True(t, settings.PortalEnabled)
	require.Equal(t, "user_1", settings.UpdatedByID)

	settings.SetBusinessHours("Monday", BusinessHours{IsBusinessDay: true})
	settings.AddHoliday(Holiday{Name: "King's Day", Date: time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)})
	settings.SetFeatureFlag("beta_dashboard", true)
	settings.AddBetaFeature("ai_triage")
	settings.AddBetaFeature("ai_triage")
	require.True(t, settings.IsFeatureEnabled("beta_dashboard"))
	require.True(t, settings.IsBetaFeatureEnabled("ai_triage"))

	settings.RemoveBetaFeature("ai_triage")
	require.False(t, settings.IsBetaFeatureEnabled("ai_triage"))

	settings.DefaultSLAHours = 24
	settings.SLAByPriority = map[string]int{string(servicedomain.CasePriorityHigh): 4}
	require.Equal(t, 4, settings.GetSLAForPriority(servicedomain.CasePriorityHigh))
	require.Equal(t, 24, settings.GetSLAForPriority(servicedomain.CasePriorityLow))

	settings.AllowedFileTypes = []string{"image/png"}
	settings.BlockedFileTypes = []string{"application/x-msdownload"}
	settings.MaxFileSize = 10
	require.True(t, settings.IsFileTypeAllowed("image/png"))
	require.False(t, settings.IsFileTypeAllowed("application/x-msdownload"))
	require.False(t, settings.IsFileSizeAllowed(11))

	settings.NotificationChannels = []string{"email"}
	settings.DefaultCaseStatus = servicedomain.CaseStatusNew
	settings.DefaultCasePriority = servicedomain.CasePriorityMedium
	require.Equal(t, "email", settings.Notifications().Channels[0])
	require.Equal(t, servicedomain.CaseStatusNew, settings.Case().DefaultStatus)
	require.Equal(t, 24, settings.SLA().DefaultHours)
}

func TestManagedUserPolicies(t *testing.T) {
	role := InstanceRole(" ADMIN ")
	user, err := NewManagedUser("user@example.com", "User", &role, time.Unix(10, 0))
	require.NoError(t, err)
	require.NotNil(t, user.InstanceRole)
	require.Equal(t, InstanceRoleAdmin, *user.InstanceRole)
	require.True(t, user.IsActive)

	super := InstanceRoleSuperAdmin
	err = user.UpdateManagedProfile("next@example.com", "Next", &super, false, true, time.Unix(20, 0))
	require.NoError(t, err)
	require.Equal(t, "next@example.com", user.Email)
	require.Equal(t, "Next", user.Name)
	require.NotNil(t, user.InstanceRole)
	require.Equal(t, InstanceRoleSuperAdmin, *user.InstanceRole)
	require.False(t, user.IsActive)
	require.True(t, user.EmailVerified)
	require.Equal(t, time.Unix(20, 0), user.UpdatedAt)

	invalid := InstanceRole("owner")
	_, err = NewManagedUser("bad@example.com", "Bad", &invalid, time.Unix(30, 0))
	require.EqualError(t, err, "invalid instance role: owner")
}

func TestEnsureAnotherActiveSuperAdmin(t *testing.T) {
	err := EnsureAnotherActiveSuperAdmin([]*User{
		{ID: "u1", InstanceRole: ptr(InstanceRoleSuperAdmin), IsActive: true},
	}, "u1", "delete")
	require.EqualError(t, err, "cannot delete the last active super admin")

	err = EnsureAnotherActiveSuperAdmin([]*User{
		{ID: "u1", InstanceRole: ptr(InstanceRoleSuperAdmin), IsActive: true},
		{ID: "u2", InstanceRole: ptr(InstanceRoleSuperAdmin), IsActive: true},
	}, "u1", "deactivate")
	require.NoError(t, err)
}

func TestSessionContextPolicies(t *testing.T) {
	super := InstanceRoleSuperAdmin
	user := &User{
		ID:           "user_1",
		InstanceRole: &super,
	}
	allWorkspaces := []*Workspace{
		{ID: "ws_1", Name: "One", Slug: "one"},
		{ID: "ws_2", Name: "Two", Slug: "two"},
	}

	contexts, err := user.BuildAvailableContexts(allWorkspaces, nil, nil)
	require.NoError(t, err)
	require.Len(t, contexts, 3)
	require.Equal(t, ContextTypeInstance, contexts[0].Type)

	defaultContext := user.DefaultContext(contexts)
	require.Equal(t, ContextTypeInstance, defaultContext.Type)

	workspaceContext, ok := FindContext(contexts, ContextTypeWorkspace, ptr("ws_2"))
	require.True(t, ok)
	require.Equal(t, "ws_2", *workspaceContext.WorkspaceID)
	require.Equal(t, string(InstanceRoleSuperAdmin), workspaceContext.Role)

	session := &Session{
		CurrentContext: Context{Type: ContextTypeWorkspace, WorkspaceID: ptr("missing")},
	}
	session.ReconcileContexts(user, contexts)
	require.Equal(t, ContextTypeInstance, session.CurrentContext.Type)
	require.True(t, HasContext(contexts, session.CurrentContext))
}

func ptr[T any](v T) *T {
	return &v
}
