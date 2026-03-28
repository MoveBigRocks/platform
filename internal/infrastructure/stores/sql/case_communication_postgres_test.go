package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestCaseStoreCommunicationPreservesAgentAuthorOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	owner := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, owner))

	caseObj := testutil.NewIsolatedCase(t, workspace.ID)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	agent := platformdomain.NewAgent(workspace.ID, "triage-bot", "Agent author", owner.ID, owner.ID)
	agent.ID = testutil.UniqueUserID(t)
	require.NoError(t, store.Agents().CreateAgent(ctx, agent))

	comm := servicedomain.NewAgentCommunication(caseObj.ID, workspace.ID, agent.ID, shareddomain.CommTypeNote, "Escalating the issue.")
	comm.FromName = agent.Name
	require.NoError(t, store.Cases().CreateCommunication(ctx, comm))

	comm.Body = "Escalating the issue with additional context."
	require.NoError(t, store.Cases().UpdateCommunication(ctx, comm))

	stored, err := store.Cases().GetCommunication(ctx, workspace.ID, comm.ID)
	require.NoError(t, err)
	assert.Equal(t, agent.ID, stored.FromAgentID)
	assert.Empty(t, stored.FromUserID)

	comms, err := store.Cases().ListCaseCommunications(ctx, caseObj.ID)
	require.NoError(t, err)
	require.Len(t, comms, 1)
	assert.Equal(t, agent.ID, comms[0].FromAgentID)
	assert.Equal(t, agent.Name, comms[0].FromName)
}
