package servicedomain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionLifecycleAndValidation(t *testing.T) {
	queue := NewQueue("ws-1", " Primary Queue ", "primary_queue", " urgent work ")
	require.Equal(t, "primary-queue", queue.Slug)
	require.Equal(t, "Primary Queue", queue.Name)
	require.Equal(t, "urgent work", queue.Description)
	require.NoError(t, queue.Validate())

	beforeRename := queue.UpdatedAt
	require.NoError(t, queue.Rename("Escalations", " high touch "))
	require.Equal(t, "Escalations", queue.Name)
	require.Equal(t, "high touch", queue.Description)
	require.False(t, queue.UpdatedAt.Before(beforeRename))

	beforeSlug := queue.UpdatedAt
	require.NoError(t, queue.SetSlug("Critical Incidents"))
	require.Equal(t, "critical-incidents", queue.Slug)
	require.False(t, queue.UpdatedAt.Before(beforeSlug))
}

func TestCollectionValidationFailuresAndSlugNormalization(t *testing.T) {
	invalid := &Queue{}
	require.EqualError(t, invalid.Validate(), "workspace_id is required")

	invalid.WorkspaceID = "ws-1"
	require.EqualError(t, invalid.Validate(), "name is required")

	invalid.Name = "Queue"
	require.EqualError(t, invalid.Validate(), "slug is required")

	invalid.Slug = "bad slug"
	require.EqualError(t, invalid.Validate(), "slug must contain only lowercase letters, numbers, and hyphens")

	cases := map[string]string{
		"Primary Queue":  "primary-queue",
		"ops_support":    "ops-support",
		"!!!":            "queue",
		" release 2026 ": "release-2026",
	}
	for input, expected := range cases {
		assert.Equal(t, expected, NormalizeQueueSlug(input, ""), "slug for %q", input)
	}

	assert.Equal(t, "fallback-name", NormalizeQueueSlug("", "Fallback Name"))
}
