package servicedomain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationSessionHandoff(t *testing.T) {
	t.Parallel()

	session := NewConversationSession("ws_123", ConversationChannelWebChat)

	err := session.Handoff("team_support", "user_123")
	require.NoError(t, err)
	assert.Equal(t, "team_support", session.HandlingTeamID)
	assert.Equal(t, "user_123", session.AssignedOperatorUserID)
	assert.Equal(t, ConversationStatusWaiting, session.Status)
	assert.False(t, session.UpdatedAt.IsZero())
}

func TestConversationSessionHandoffRequiresTarget(t *testing.T) {
	t.Parallel()

	session := NewConversationSession("ws_123", ConversationChannelWebChat)

	err := session.Handoff("", "")
	require.EqualError(t, err, "team_id or operator_user_id is required")
}

func TestConversationSessionEscalateLinksCase(t *testing.T) {
	t.Parallel()

	session := NewConversationSession("ws_123", ConversationChannelEmail)

	err := session.Escalate("case_123", "team_billing", "user_456")
	require.NoError(t, err)
	assert.Equal(t, "case_123", session.LinkedCaseID)
	assert.Equal(t, "team_billing", session.HandlingTeamID)
	assert.Equal(t, "user_456", session.AssignedOperatorUserID)
	assert.Equal(t, ConversationStatusEscalated, session.Status)
}

func TestConversationSessionEscalateRejectsClosedConversation(t *testing.T) {
	t.Parallel()

	session := NewConversationSession("ws_123", ConversationChannelWebChat)
	session.Status = ConversationStatusClosed

	err := session.Escalate("case_123", "team_support", "")
	require.EqualError(t, err, "cannot escalate a closed conversation")
}
