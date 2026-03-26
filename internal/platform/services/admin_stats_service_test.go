package platformservices

import (
	"context"
	"errors"
	"testing"

	"github.com/movebigrocks/platform/pkg/logger"
)

type testAdminStatsExtensionGate struct {
	globalEnabled    bool
	workspaceEnabled map[string]bool
	err              error
}

func (g testAdminStatsExtensionGate) HasActiveExtension(_ context.Context, _ string) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return g.globalEnabled, nil
}

func (g testAdminStatsExtensionGate) HasActiveExtensionInWorkspace(_ context.Context, workspaceID, _ string) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return g.workspaceEnabled[workspaceID], nil
}

func TestAdminStatsServiceSurfaceGateReturnsFalseOnLookupError(t *testing.T) {
	svc := &AdminStatsService{
		logger: logger.New(),
		extensionGate: testAdminStatsExtensionGate{
			err: errors.New("boom"),
		},
	}

	if svc.isSurfaceEnabled(context.Background(), "", "error-tracking") {
		t.Fatal("expected surface to be treated as disabled when lookup fails")
	}
}
