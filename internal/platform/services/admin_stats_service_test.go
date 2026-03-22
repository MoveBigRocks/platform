package platformservices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/movebigrocks/platform/pkg/logger"
)

type testAdminStatsExtensionGate struct {
	globalEnabled    bool
	workspaceEnabled map[string]bool
	err              error
}

type testAdminStatsIssueMetricsProvider struct {
	count int64
	err   error
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

func (p testAdminStatsIssueMetricsProvider) CountRecentIssues(_ context.Context, _ time.Time) (int64, error) {
	if p.err != nil {
		return 0, p.err
	}
	return p.count, nil
}

func TestAdminStatsServiceGetErrorStatsFromStoreSkipsDisabledSurface(t *testing.T) {
	svc := &AdminStatsService{
		logger: logger.New(),
		extensionGate: testAdminStatsExtensionGate{
			globalEnabled: false,
		},
	}

	recentIssues, errorRate := svc.getErrorStatsFromStore(context.Background())
	if recentIssues != 0 {
		t.Fatalf("expected zero recent issues, got %d", recentIssues)
	}
	if errorRate != 0 {
		t.Fatalf("expected zero error rate, got %f", errorRate)
	}
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

func TestAdminStatsServiceGetErrorStatsFromProvider(t *testing.T) {
	svc := &AdminStatsService{
		logger: logger.New(),
		extensionGate: testAdminStatsExtensionGate{
			globalEnabled: true,
		},
		issueMetricsProvider: testAdminStatsIssueMetricsProvider{
			count: 144,
		},
	}

	recentIssues, errorRate := svc.getErrorStatsFromStore(context.Background())
	if recentIssues != 144 {
		t.Fatalf("expected 144 recent issues, got %d", recentIssues)
	}
	if errorRate <= 0 {
		t.Fatalf("expected positive error rate, got %f", errorRate)
	}
}
