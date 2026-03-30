package shareddomain

import (
	"testing"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"

	"github.com/stretchr/testify/require"
)

func TestKnowledgeCreatedValidate(t *testing.T) {
	t.Parallel()

	event := KnowledgeCreated{
		BaseEvent:           eventbus.BaseEvent{EventID: "evt-1", EventType: eventbus.TypeKnowledgeCreated, Timestamp: time.Now()},
		KnowledgeResourceID: "kr_1",
		WorkspaceID:         "ws_1",
		OwnerTeamID:         "team_1",
		Slug:                "queue-routing",
		Title:               "Queue Routing RFC",
		Kind:                "decision",
		CreatedAt:           time.Now(),
	}
	require.NoError(t, event.Validate())
}

func TestKnowledgeReviewRequestedValidate(t *testing.T) {
	t.Parallel()

	event := KnowledgeReviewRequested{
		BaseEvent:           eventbus.BaseEvent{EventID: "evt-1", EventType: eventbus.TypeKnowledgeReviewRequested, Timestamp: time.Now()},
		KnowledgeResourceID: "kr_1",
		WorkspaceID:         "ws_1",
		OwnerTeamID:         "team_1",
		Slug:                "queue-routing",
		Title:               "Queue Routing RFC",
		Kind:                "decision",
		RequestedAt:         time.Now(),
	}
	require.NoError(t, event.Validate())
}
