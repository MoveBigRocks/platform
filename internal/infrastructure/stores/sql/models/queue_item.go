package models

import "time"

type QueueItem struct {
	ID                    string     `db:"id"`
	WorkspaceID           string     `db:"workspace_id"`
	QueueID               string     `db:"queue_id"`
	ItemKind              string     `db:"item_kind"`
	CaseID                *string    `db:"case_id"`
	ConversationSessionID *string    `db:"conversation_session_id"`
	CreatedAt             time.Time  `db:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"`
	DeletedAt             *time.Time `db:"deleted_at"`
}

func (QueueItem) TableName() string {
	return "queue_items"
}
