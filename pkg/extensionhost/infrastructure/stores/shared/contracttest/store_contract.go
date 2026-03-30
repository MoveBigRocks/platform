package contracttest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

type StoreFactory func(t testing.TB) (shared.Store, func())

// RunStoreContractTests exercises cross-implementation guarantees for shared.Store.
func RunStoreContractTests(t *testing.T, newStore StoreFactory) {
	t.Helper()

	t.Run("health_check", func(t *testing.T) {
		t.Parallel()

		store, cleanup := newStore(t)
		defer cleanup()

		if err := store.HealthCheck(context.Background()); err != nil {
			t.Fatalf("health check failed: %v", err)
		}
	})

	t.Run("transaction_commit_persists_workspace", func(t *testing.T) {
		t.Parallel()

		store, cleanup := newStore(t)
		defer cleanup()

		ctx := context.Background()
		workspace := newWorkspace(t, "commit")

		if err := store.WithTransaction(ctx, func(txCtx context.Context) error {
			return store.Workspaces().CreateWorkspace(txCtx, workspace)
		}); err != nil {
			t.Fatalf("commit transaction: %v", err)
		}

		got, err := store.Workspaces().GetWorkspace(ctx, workspace.ID)
		if err != nil {
			t.Fatalf("get workspace after commit: %v", err)
		}
		if got.ID != workspace.ID {
			t.Fatalf("workspace id mismatch: got %q want %q", got.ID, workspace.ID)
		}
	})

	t.Run("transaction_rollback_discards_workspace", func(t *testing.T) {
		t.Parallel()

		store, cleanup := newStore(t)
		defer cleanup()

		ctx := context.Background()
		workspace := newWorkspace(t, "rollback")
		sentinel := errors.New("rollback")

		err := store.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := store.Workspaces().CreateWorkspace(txCtx, workspace); err != nil {
				return err
			}
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("rollback error mismatch: got %v", err)
		}

		_, err = store.Workspaces().GetWorkspace(ctx, workspace.ID)
		if !errors.Is(err, shared.ErrNotFound) {
			t.Fatalf("expected not found after rollback, got %v", err)
		}
	})
}

func newWorkspace(t testing.TB, prefix string) *platformdomain.Workspace {
	t.Helper()

	now := time.Now().UTC()
	slug := strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(t.Name())
	if slug == "" {
		slug = "store-contract" //nolint:ineffassign // fallback value used in Workspace below
	}

	return &platformdomain.Workspace{
		ID:        id.New(),
		Name:      prefix + " workspace",
		Slug:      fmt.Sprintf("%s-%d", prefix, now.UnixNano()),
		ShortCode: "TST",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
