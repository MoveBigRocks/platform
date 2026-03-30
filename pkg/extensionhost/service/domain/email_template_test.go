package servicedomain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewEmailTemplate(t *testing.T) {
	template := NewEmailTemplate("ws-1", "Welcome", "Hello {{name}}", "user-1")
	require.Empty(t, template.ID)
	require.Equal(t, "ws-1", template.WorkspaceID)
	require.Equal(t, "Welcome", template.Name)
	require.Equal(t, "Hello {{name}}", template.Subject)
	require.True(t, template.IsActive)
	require.Equal(t, "en", template.Language)
	require.Equal(t, 1, template.Version)
	require.Empty(t, template.Variables)
	require.NotNil(t, template.SampleData)
	require.Equal(t, "user-1", template.CreatedByID)
	require.False(t, template.CreatedAt.IsZero())
	require.False(t, template.UpdatedAt.IsZero())
}
