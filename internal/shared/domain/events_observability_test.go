package shareddomain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

func TestObservabilityEventConstructorsAndValidation(t *testing.T) {
	created := NewIssueCreatedEvent(
		"issue_1",
		"project_1",
		"ws_1",
		"TypeError: boom",
		"error",
		"fingerprint_1",
		"event_1",
		"go",
		"handler.go",
	)
	require.Equal(t, eventbus.TypeIssueCreated, created.EventType)
	require.NotEmpty(t, created.EventID)
	require.False(t, created.CreatedAt.IsZero())
	require.NoError(t, created.Validate())

	updated := NewIssueUpdatedEventWithUserFlag(
		"issue_1",
		"project_1",
		"ws_1",
		"event_2",
		time.Now().UTC(),
		true,
	)
	require.Equal(t, eventbus.TypeIssueUpdated, updated.EventType)
	require.NotEmpty(t, updated.EventID)
	require.True(t, updated.HasNewUser)
	require.NoError(t, updated.Validate())
}
