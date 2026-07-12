package platformservices

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type extensionAuditSpy struct {
	activities []LogActivityRequest
}

func (s *extensionAuditSpy) LogActivity(_ context.Context, req LogActivityRequest) error {
	s.activities = append(s.activities, req)
	return nil
}

func TestExtensionServiceRecordsWorkspaceLifecycleAudit(t *testing.T) {
	audit := &extensionAuditSpy{}
	service := NewExtensionServiceWithOptions(nil, nil, nil, nil, nil, nil, WithExtensionAuditService(audit))
	extension := &platformdomain.InstalledExtension{
		ID:          "extension-1",
		WorkspaceID: "workspace-1",
		Slug:        "service-desk",
		Name:        "Service Desk",
		Version:     "1.2.3",
		Status:      platformdomain.ExtensionStatusActive,
	}

	service.recordExtensionLifecycle(
		context.Background(),
		extension,
		"user-1",
		platformdomain.AuditActionExtensionActivated,
		"",
	)

	require.Len(t, audit.activities, 1)
	activity := audit.activities[0]
	assert.Equal(t, "workspace-1", activity.WorkspaceID)
	assert.Equal(t, "user-1", activity.ActorID)
	assert.Equal(t, string(platformdomain.AuditActionExtensionActivated), activity.Action)
	assert.Equal(t, "extension", activity.ResourceType)
	assert.Equal(t, "1.2.3", activity.Details.GetString("version"))
	assert.Equal(t, string(platformdomain.ExtensionStatusActive), activity.Details.GetString("status"))
}

func TestExtensionServiceSkipsInstanceLifecycleAuditWithoutInstanceSchema(t *testing.T) {
	audit := &extensionAuditSpy{}
	service := NewExtensionServiceWithOptions(nil, nil, nil, nil, nil, nil, WithExtensionAuditService(audit))

	service.recordExtensionLifecycle(context.Background(), &platformdomain.InstalledExtension{
		ID:   "extension-1",
		Slug: "instance-extension",
	}, "user-1", platformdomain.AuditActionExtensionInstalled, "")

	assert.Empty(t, audit.activities)
}
