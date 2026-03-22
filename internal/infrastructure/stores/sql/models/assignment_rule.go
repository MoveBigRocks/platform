package models

import (
	"time"
)

// AssignmentRule represents an automatic case assignment rule
type AssignmentRule struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Rule details
	Name        string `db:"name"`
	Description string `db:"description"`
	IsActive    bool   `db:"is_active"`
	Priority    int    `db:"priority"`

	// Conditions (JSON array)
	Conditions string `db:"conditions"`

	// Assignment strategy: "round_robin", "load_balancing", "skills", "availability"
	Strategy string `db:"strategy"`

	// Target configuration (JSON arrays)
	TargetUsers string `db:"target_users"`
	TargetTeams string `db:"target_teams"`

	// Assignment criteria (JSON arrays for skills)
	RequiredSkills      string `db:"required_skills"`
	PreferredSkills     string `db:"preferred_skills"`
	MaxWorkload         int    `db:"max_workload"`
	RequireAvailability bool   `db:"require_availability"`

	// Business hours
	BusinessHoursOnly bool   `db:"business_hours_only"`
	Timezone          string `db:"timezone"`
	BusinessHours     string `db:"business_hours"`

	// Escalation
	AutoEscalate      bool   `db:"auto_escalate"`
	EscalationDelay   int    `db:"escalation_delay"`
	EscalationTargets string `db:"escalation_targets"`

	// Fallback
	FallbackStrategy string `db:"fallback_strategy"`
	FallbackTargets  string `db:"fallback_targets"`

	// Usage tracking
	TimesUsed   int        `db:"times_used"`
	LastUsedAt  *time.Time `db:"last_used_at"`
	SuccessRate float64    `db:"success_rate"`

	// Metadata
	CreatedByID string     `db:"created_by_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}
