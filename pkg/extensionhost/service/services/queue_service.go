package serviceapp

import (
	"context"
	"strings"

	apierrors "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/errors"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

// QueueService manages workspace-scoped case queues.
type QueueService struct {
	queueStore     shared.QueueStore
	queueItemStore shared.QueueItemStore
	workspaceStore shared.WorkspaceStore
}

type CreateQueueParams struct {
	WorkspaceID string
	Name        string
	Slug        string
	Description string
}

type UpdateQueueParams struct {
	Name        *string
	Slug        *string
	Description *string
}

func NewQueueService(queueStore shared.QueueStore, queueItemStore shared.QueueItemStore, workspaceStore shared.WorkspaceStore) *QueueService {
	return &QueueService{
		queueStore:     queueStore,
		queueItemStore: queueItemStore,
		workspaceStore: workspaceStore,
	}
}

func (s *QueueService) CreateQueue(ctx context.Context, params CreateQueueParams) (*servicedomain.Queue, error) {
	if params.WorkspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("name", "required"))
	}

	if s.workspaceStore != nil {
		workspace, err := s.workspaceStore.GetWorkspace(ctx, params.WorkspaceID)
		if err != nil || workspace == nil {
			return nil, apierrors.NotFoundError("workspace", params.WorkspaceID)
		}
	}

	queue := servicedomain.NewQueue(params.WorkspaceID, params.Name, params.Slug, params.Description)
	if err := queue.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue validation failed")
	}
	if err := s.queueStore.CreateQueue(ctx, queue); err != nil {
		return nil, apierrors.DatabaseError("create queue", err)
	}
	return queue, nil
}

func (s *QueueService) GetQueue(ctx context.Context, queueID string) (*servicedomain.Queue, error) {
	if queueID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("queue_id", "required"))
	}
	queue, err := s.queueStore.GetQueue(ctx, queueID)
	if err != nil {
		return nil, apierrors.NotFoundError("queue", queueID)
	}
	return queue, nil
}

func (s *QueueService) ListWorkspaceQueues(ctx context.Context, workspaceID string) ([]*servicedomain.Queue, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	queues, err := s.queueStore.ListWorkspaceQueues(ctx, workspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list queues", err)
	}
	return queues, nil
}

func (s *QueueService) UpdateQueue(ctx context.Context, queueID string, params UpdateQueueParams) (*servicedomain.Queue, error) {
	queue, err := s.GetQueue(ctx, queueID)
	if err != nil {
		return nil, err
	}

	if params.Name != nil || params.Description != nil {
		name := queue.Name
		if params.Name != nil {
			name = *params.Name
		}
		description := queue.Description
		if params.Description != nil {
			description = *params.Description
		}
		if err := queue.Rename(name, description); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue validation failed")
		}
	}
	if params.Slug != nil {
		if err := queue.SetSlug(*params.Slug); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue validation failed")
		}
	}
	if err := queue.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue validation failed")
	}
	if err := s.queueStore.UpdateQueue(ctx, queue); err != nil {
		return nil, apierrors.DatabaseError("update queue", err)
	}
	return queue, nil
}

func (s *QueueService) DeleteQueue(ctx context.Context, workspaceID, queueID string) error {
	if workspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if queueID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("queue_id", "required"))
	}
	if err := s.queueStore.DeleteQueue(ctx, workspaceID, queueID); err != nil {
		return apierrors.DatabaseError("delete queue", err)
	}
	return nil
}

func (s *QueueService) ListQueueItems(ctx context.Context, queueID string) ([]*servicedomain.QueueItem, error) {
	if queueID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("queue_id", "required"))
	}
	if _, err := s.GetQueue(ctx, queueID); err != nil {
		return nil, err
	}
	if s.queueItemStore == nil {
		return []*servicedomain.QueueItem{}, nil
	}
	items, err := s.queueItemStore.ListQueueItems(ctx, queueID)
	if err != nil {
		return nil, apierrors.DatabaseError("list queue items", err)
	}
	return items, nil
}
