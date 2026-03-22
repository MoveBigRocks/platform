package models

import "time"

// Queue represents a workspace-scoped case queue.
type Queue struct {
	ID          string     `db:"id"`
	WorkspaceID string     `db:"workspace_id"`
	Slug        string     `db:"slug"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
	Metadata    string     `db:"metadata"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (Queue) TableName() string {
	return "case_queues"
}
