package knowledgeservices

import (
	"context"
	"strings"
	"testing"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
)

func TestConceptSpecService_HistoryAndDiff(t *testing.T) {
	t.Parallel()

	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "concept-history")
	teamID := setupTestTeam(t, store, workspaceID, "Strategy")
	service := NewConceptSpecService(store.ConceptSpecs(), store.Workspaces(), artifactservices.NewGitService(t.TempDir()))
	ctx := context.Background()

	spec, err := service.RegisterConceptSpec(ctx, RegisterConceptSpecParams{
		WorkspaceID:  workspaceID,
		OwnerTeamID:  teamID,
		Key:          "strategy/campaign-brief",
		Version:      "1",
		Name:         "Campaign Brief",
		InstanceKind: knowledgedomain.KnowledgeResourceKindContext,
		CreatedBy:    "user_123",
	})
	if err != nil {
		t.Fatalf("register concept spec: %v", err)
	}

	history, err := service.ConceptSpecHistory(ctx, workspaceID, spec.Key, spec.Version, 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 concept revision, got %#v", history)
	}
	if strings.TrimSpace(history[0].Ref) == "" {
		t.Fatalf("expected revision ref in history")
	}

	diff, err := service.ConceptSpecDiff(ctx, workspaceID, spec.Key, spec.Version, "", "")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff.ToRevision == "" {
		t.Fatalf("expected diff to include target revision")
	}
	if !strings.Contains(diff.Patch, "+key: strategy/campaign-brief") {
		t.Fatalf("unexpected concept diff patch: %s", diff.Patch)
	}
}
