package models

import (
	"time"
)

// Project (Application)
type Project struct {
	ID                 string     `db:"id"`
	WorkspaceID        string     `db:"workspace_id"`
	ExtensionInstallID string     `db:"extension_install_id"`
	TeamID             *string    `db:"team_id"`
	Name               string     `db:"name"`
	Slug               string     `db:"slug"`
	Repository         string     `db:"repository"`
	Platform           string     `db:"platform"`
	Environment        string     `db:"environment"`
	DSN                string     `db:"dsn"`
	PublicKey          string     `db:"public_key"`
	SecretKey          string     `db:"secret_key"`
	AppKey             string     `db:"app_key"`
	ProjectNumber      int64      `db:"project_number"`
	EventsPerHour      int        `db:"events_per_hour"`
	StorageQuotaMB     int        `db:"storage_quota_mb"`
	RetentionDays      int        `db:"retention_days"`
	Status             string     `db:"status"`
	EventCount         int64      `db:"event_count"`
	LastEventAt        *time.Time `db:"last_event_at"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
	DeletedAt          *time.Time `db:"deleted_at"`
}

func (Project) TableName() string {
	return "projects"
}

// Issue
type Issue struct {
	ID                 string     `db:"id"`
	WorkspaceID        string     `db:"workspace_id"`
	ExtensionInstallID string     `db:"extension_install_id"`
	ProjectID          string     `db:"project_id"`
	Title              string     `db:"title"`
	Culprit            string     `db:"culprit"`
	Fingerprint        string     `db:"fingerprint"`
	Status             string     `db:"status"`
	Level              string     `db:"level"`
	Type               string     `db:"type"`
	FirstSeen          time.Time  `db:"first_seen"`
	LastSeen           time.Time  `db:"last_seen"`
	EventCount         int64      `db:"event_count"`
	UserCount          int64      `db:"user_count"`
	AssignedTo         *string    `db:"assigned_to"`
	ResolvedAt         *time.Time `db:"resolved_at"`
	ResolvedBy         *string    `db:"resolved_by"`
	Resolution         string     `db:"resolution"`
	ResolutionNotes    string     `db:"resolution_notes"`
	ResolvedInCommit   string     `db:"resolved_in_commit"`
	ResolvedInVersion  string     `db:"resolved_in_version"`
	HasRelatedCase     bool       `db:"has_related_case"`
	RelatedCaseIDs     string     `db:"related_case_ids"` // JSON array
	Tags               string     `db:"tags"`             // JSON object
	Permalink          string     `db:"permalink"`
	ShortID            string     `db:"short_id"`
	Logger             string     `db:"logger"`
	Platform           string     `db:"platform"`
	LastEventID        string     `db:"last_event_id"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
}

func (Issue) TableName() string {
	return "issues"
}

// ErrorEvent
type ErrorEvent struct {
	ID                 string     `db:"id"`
	WorkspaceID        string     `db:"workspace_id"`
	ExtensionInstallID string     `db:"extension_install_id"`
	EventID            string     `db:"event_id"`
	ProjectID          string     `db:"project_id"`
	IssueID            *string    `db:"issue_id"`
	Timestamp          time.Time  `db:"timestamp"`
	Received           time.Time  `db:"received"`
	Message            string     `db:"message"`
	Level              string     `db:"level"`
	Logger             string     `db:"logger"`
	Platform           string     `db:"platform"`
	Environment        string     `db:"environment"`
	Release            string     `db:"release"`
	Dist               string     `db:"dist"`
	Exception          string     `db:"exception"`   // JSON array
	Stacktrace         string     `db:"stacktrace"`  // JSON object
	User               string     `db:"user"`        // JSON object
	Request            string     `db:"request"`     // JSON object
	Tags               string     `db:"tags"`        // JSON object
	Extra              string     `db:"extra"`       // JSON object
	Contexts           string     `db:"contexts"`    // JSON object
	Breadcrumbs        string     `db:"breadcrumbs"` // JSON array
	Fingerprint        string     `db:"fingerprint"` // JSON array
	DataURL            string     `db:"data_url"`
	Size               int64      `db:"size"`
	ProcessedAt        *time.Time `db:"processed_at"`
	GroupedAt          *time.Time `db:"grouped_at"`
}

func (ErrorEvent) TableName() string {
	return "error_events"
}

// Alert
type Alert struct {
	ID                 string     `db:"id"`
	WorkspaceID        string     `db:"workspace_id"`
	ExtensionInstallID string     `db:"extension_install_id"`
	ProjectID          string     `db:"project_id"`
	Name               string     `db:"name"`
	Conditions         string     `db:"conditions"` // JSON array
	Frequency          int64      `db:"frequency"`  // Duration in nanoseconds
	Actions            string     `db:"actions"`    // JSON array
	Enabled            bool       `db:"enabled"`
	CooldownMinutes    int        `db:"cooldown_minutes"`
	LastTriggered      *time.Time `db:"last_triggered"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
}

func (Alert) TableName() string {
	return "alerts"
}

// ProjectStats
type ProjectStats struct {
	ID                 string    `db:"id"`
	WorkspaceID        string    `db:"workspace_id"`
	ExtensionInstallID string    `db:"extension_install_id"`
	ProjectID          string    `db:"project_id"`
	Date               time.Time `db:"date"`
	EventCount         int64     `db:"event_count"`
	IssueCount         int64     `db:"issue_count"`
	UserCount          int64     `db:"user_count"`
	ErrorRate          float64   `db:"error_rate"`
	NewIssues          int64     `db:"new_issues"`
	ResolvedIssues     int64     `db:"resolved_issues"`
}

// GitRepo
type GitRepo struct {
	ID                 string    `db:"id"`
	ApplicationID      string    `db:"application_id"`
	WorkspaceID        string    `db:"workspace_id"`
	ExtensionInstallID string    `db:"extension_install_id"`
	RepoURL            string    `db:"repo_url"`
	DefaultBranch      string    `db:"default_branch"`
	AccessToken        string    `db:"access_token"`
	PathPrefix         string    `db:"path_prefix"`
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

func (GitRepo) TableName() string {
	return "git_repos"
}

// IssueStats
type IssueStats struct {
	ID                 string    `db:"id"`
	WorkspaceID        string    `db:"workspace_id"`
	ExtensionInstallID string    `db:"extension_install_id"`
	IssueID            string    `db:"issue_id"`
	Date               time.Time `db:"date"`
	EventCount         int64     `db:"event_count"`
	UserCount          int64     `db:"user_count"`
	FirstOccurrence    time.Time `db:"first_occurrence"`
	LastOccurrence     time.Time `db:"last_occurrence"`
}
