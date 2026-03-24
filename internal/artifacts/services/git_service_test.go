package artifactservices

import (
	"context"
	"strings"
	"testing"
)

func TestGitServiceWriteHistoryAndDiff(t *testing.T) {
	service := NewGitService(t.TempDir())
	ctx := context.Background()
	repository := WorkspaceRepository("ws_123")

	first, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		Content:       []byte("# Playbook\n\nDraft\n"),
		CommitMessage: "knowledge create playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if !first.Changed || strings.TrimSpace(first.Ref) == "" {
		t.Fatalf("expected first write to create a revision, got %#v", first)
	}

	second, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		Content:       []byte("# Playbook\n\nPublished\n"),
		CommitMessage: "knowledge update playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !second.Changed || second.Ref == first.Ref {
		t.Fatalf("expected a second distinct revision, got %#v", second)
	}

	history, err := service.History(ctx, repository, "knowledge/teams/team_123/private/playbook.md", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(history))
	}
	if history[0].Ref != second.Ref || history[1].Ref != first.Ref {
		t.Fatalf("unexpected history order: %#v", history)
	}

	fromRef, toRef, patch, err := service.Diff(ctx, repository, "knowledge/teams/team_123/private/playbook.md", first.Ref, second.Ref)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if fromRef != first.Ref || toRef != second.Ref {
		t.Fatalf("unexpected diff refs: from=%s to=%s", fromRef, toRef)
	}
	if !strings.Contains(patch, "-Draft") || !strings.Contains(patch, "+Published") {
		t.Fatalf("unexpected patch: %s", patch)
	}

	files, err := service.List(ctx, repository, "knowledge/teams/team_123/private")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 || files[0] != "knowledge/teams/team_123/private/playbook.md" {
		t.Fatalf("unexpected file list: %#v", files)
	}

	content, err := service.Read(ctx, ReadParams{
		Repository:   repository,
		RelativePath: "knowledge/teams/team_123/private/playbook.md",
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) != "# Playbook\n\nPublished\n" {
		t.Fatalf("unexpected content: %s", string(content))
	}
}

func TestGitServiceDelete(t *testing.T) {
	service := NewGitService(t.TempDir())
	ctx := context.Background()
	repository := WorkspaceRepository("ws_123")

	first, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		Content:       []byte("# Playbook\n\nPublished\n"),
		CommitMessage: "knowledge create playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	deleted, err := service.Delete(ctx, DeleteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		CommitMessage: "knowledge delete playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted.Changed || deleted.Ref == "" || deleted.Ref == first.Ref {
		t.Fatalf("expected delete to create a distinct revision, got %#v", deleted)
	}

	files, err := service.List(ctx, repository, "knowledge/teams/team_123/private")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected deleted file to be absent, got %#v", files)
	}

	if _, err := service.Read(ctx, ReadParams{
		Repository:   repository,
		RelativePath: "knowledge/teams/team_123/private/playbook.md",
	}); err == nil {
		t.Fatalf("expected read to fail after delete")
	}

	history, err := service.History(ctx, repository, "knowledge/teams/team_123/private/playbook.md", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected delete to remain visible in history, got %#v", history)
	}
	if history[0].Ref != deleted.Ref || history[1].Ref != first.Ref {
		t.Fatalf("unexpected history order after delete: %#v", history)
	}
}
