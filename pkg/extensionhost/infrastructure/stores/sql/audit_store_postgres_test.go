package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestAuditStorePersistsAppendOnlyGovernanceRecords(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()
	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := platformservices.NewAuditService(store.Audits(), store)
	require.NoError(t, service.LogActivity(ctx, platformservices.LogActivityRequest{
		WorkspaceID:  workspace.ID,
		Action:       string(platformdomain.AuditActionLogin),
		ResourceType: "session",
		Outcome:      "success",
	}))
	require.NoError(t, service.LogSecurityEvent(ctx, platformservices.LogSecurityEventRequest{
		WorkspaceID: workspace.ID,
		EventType:   string(platformdomain.SecurityEventTypeAuthenticationFailure),
		Severity:    string(platformdomain.SecuritySeverityLow),
		Description: "Authentication failed",
	}))

	db, err := store.GetSQLDB()
	require.NoError(t, err)
	var auditCount, securityCount int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM core_governance.audit_logs WHERE workspace_id = $1`, workspace.ID,
	).Scan(&auditCount))
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM core_governance.security_events WHERE workspace_id = $1`, workspace.ID,
	).Scan(&securityCount))
	assert.Equal(t, 1, auditCount)
	assert.Equal(t, 1, securityCount)
}
