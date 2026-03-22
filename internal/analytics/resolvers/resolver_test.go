package analyticsresolvers

import (
	"context"
	"strings"
	"testing"

	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

type testExtensionChecker struct {
	workspaceEnabled map[string]bool
	globalEnabled    bool
}

func (t testExtensionChecker) HasActiveExtension(_ context.Context, _ string) (bool, error) {
	return t.globalEnabled, nil
}

func (t testExtensionChecker) HasActiveExtensionInWorkspace(_ context.Context, workspaceID, _ string) (bool, error) {
	return t.workspaceEnabled[workspaceID], nil
}

func TestAnalyticsPropertiesRequiresActiveExtension(t *testing.T) {
	resolver := NewResolver(Config{
		ExtensionChecker: testExtensionChecker{
			workspaceEnabled: map[string]bool{"ws_disabled": false},
		},
	})

	ctx := graphshared.SetAuthContext(context.Background(), &platformdomain.AuthContext{
		WorkspaceID:  "ws_disabled",
		WorkspaceIDs: []string{"ws_disabled"},
	})

	_, err := resolver.AnalyticsProperties(ctx)
	if err == nil || !strings.Contains(err.Error(), "web-analytics is not active") {
		t.Fatalf("expected inactive extension error, got %v", err)
	}
}
