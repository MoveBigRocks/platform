package servicedomain

import (
	"fmt"
	"time"
)

type QueueItemKind string

const (
	QueueItemKindCase                QueueItemKind = "case"
	QueueItemKindConversationSession QueueItemKind = "conversation_session"
)

// QueueItem is the concrete work object currently visible in a queue.
// It points to either a case or a conversation session.
type QueueItem struct {
	ID                    string
	WorkspaceID           string
	QueueID               string
	ItemKind              QueueItemKind
	CaseID                string
	ConversationSessionID string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func NewCaseQueueItem(workspaceID, queueID, caseID string) *QueueItem {
	now := time.Now().UTC()
	return &QueueItem{
		WorkspaceID: workspaceID,
		QueueID:     queueID,
		ItemKind:    QueueItemKindCase,
		CaseID:      caseID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func NewConversationQueueItem(workspaceID, queueID, sessionID string) *QueueItem {
	now := time.Now().UTC()
	return &QueueItem{
		WorkspaceID:           workspaceID,
		QueueID:               queueID,
		ItemKind:              QueueItemKindConversationSession,
		ConversationSessionID: sessionID,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

func (i *QueueItem) Validate() error {
	if i.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if i.QueueID == "" {
		return fmt.Errorf("queue_id is required")
	}
	switch i.ItemKind {
	case QueueItemKindCase:
		if i.CaseID == "" {
			return fmt.Errorf("case_id is required")
		}
		if i.ConversationSessionID != "" {
			return fmt.Errorf("conversation_session_id must be empty for case queue items")
		}
	case QueueItemKindConversationSession:
		if i.ConversationSessionID == "" {
			return fmt.Errorf("conversation_session_id is required")
		}
		if i.CaseID != "" {
			return fmt.Errorf("case_id must be empty for conversation queue items")
		}
	default:
		return fmt.Errorf("item_kind is required")
	}
	return nil
}

func (i *QueueItem) MoveToQueue(queueID string) error {
	if queueID == "" {
		return fmt.Errorf("queue_id is required")
	}
	i.QueueID = queueID
	i.UpdatedAt = time.Now().UTC()
	return nil
}
