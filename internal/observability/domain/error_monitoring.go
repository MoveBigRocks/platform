package observabilitydomain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	shared "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// Application represents a monitored application/service
// This is Move Big Rocks' internal concept. The Sentry protocol uses "project"
// but internally we use "application" for clarity
type Application struct {
	ID          string
	Name        string
	Slug        string // Indexed for GetApplicationBySlug (public URLs)
	WorkspaceID string // Indexed for ListWorkspaceApplications
	TeamID      string // Indexed for ListTeamApplications
	Repository  string // Git repo URL
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Configuration
	Platform    string // javascript, python, go, etc.
	Environment string // production, staging, development

	// Sentry protocol compatibility (DSN components)
	// DSN format: https://{public_key}@{host}/{project_number}
	// The project_number is used in the DSN path for SDK compatibility (must be numeric)
	// The public_key is used for actual authentication
	DSN           string // Full DSN for SDK
	PublicKey     string
	SecretKey     string
	AppKey        string // Legacy - used in some internal URLs
	ProjectNumber int64  // Numeric ID for Sentry SDK DSN compatibility

	// Rate limiting and quotas
	EventsPerHour  int
	StorageQuotaMB int
	RetentionDays  int

	// Status and health
	Status      string // active, paused, disabled
	LastEventAt *time.Time
	EventCount  int64
}

// Project is the Sentry-facing alias for Application.
// Use Application internally and Project only at protocol boundaries.
type Project = Application

// Issue represents a group of similar error events
type Issue struct {
	ID            string
	WorkspaceID   string // Indexed for ListWorkspaceIssues (denormalized for fast queries)
	ProjectID     string // Indexed for ListProjectIssues (kept for Sentry compatibility)
	ApplicationID string // Same as ProjectID, clearer name (no separate index needed)
	Title         string
	Culprit       string // Function/file where error occurred
	Fingerprint   string // Indexed for GetIssueByFingerprint (error deduplication - CRITICAL)

	// Status and lifecycle
	Status string // Indexed for filtering (unresolved, resolved, ignored, muted)
	Level  string // fatal, error, warning, info, debug (not indexed - filtered in-memory)
	Type   string // error, csp, default (not indexed)

	// Occurrence tracking
	FirstSeen  time.Time
	LastSeen   time.Time
	EventCount int64
	UserCount  int64

	// Assignment and resolution
	AssignedTo        string
	ResolvedAt        *time.Time
	ResolvedBy        string
	Resolution        string // fixed, wont_fix, duplicate
	ResolutionNotes   string
	ResolvedInCommit  string // Git SHA
	ResolvedInVersion string // v1.2.3

	// Case integration
	HasRelatedCase bool
	RelatedCaseIDs []string

	// Metadata
	Tags      map[string]string
	Permalink string
	ShortID   string

	// Additional context
	Logger      string
	Platform    string
	LastEventID string
}

// ErrorEvent represents an individual error occurrence
type ErrorEvent struct {
	ID        string
	EventID   string // Indexed for event lookup by Sentry-compatible event ID
	ProjectID string // Indexed for ListProjectEvents
	IssueID   string // Indexed for ListIssueEvents (issue detail timeline)

	// Timing
	Timestamp time.Time
	Received  time.Time

	// Error details
	Message  string
	Level    string
	Logger   string
	Platform string

	// Context and environment
	Environment string
	Release     string
	Dist        string

	// Exception details
	Exception  []ExceptionData
	Stacktrace *StacktraceData

	// User and request context
	User    *UserContext
	Request *RequestContext

	// Additional context
	Tags        map[string]string
	Extra       shared.Metadata
	Contexts    shared.Metadata
	Breadcrumbs []Breadcrumb

	// Fingerprinting
	Fingerprint []string

	// Storage references
	DataURL string // S3/storage reference for large events
	Size    int64

	// Processing metadata
	ProcessedAt *time.Time
	GroupedAt   *time.Time
}

// Supporting data structures for error events
type ExceptionData struct {
	Type       string
	Value      string
	Module     string
	Stacktrace *StacktraceData
}

type StacktraceData struct {
	Frames []FrameData
}

type FrameData struct {
	Filename    string
	Function    string
	Module      string
	LineNumber  int
	ColNumber   int
	AbsPath     string
	ContextLine string
	PreContext  []string
	PostContext []string
	InApp       bool
	Vars        shared.Metadata
}

type UserContext struct {
	ID       string
	Email    string
	Username string
	IPAddr   string
}

type RequestContext struct {
	URL         string
	Method      string
	Headers     map[string]string
	Data        shared.Metadata
	QueryString string
	Cookies     map[string]string
}

type Breadcrumb struct {
	Timestamp time.Time
	Message   string
	Category  string
	Level     string
	Type      string
	Data      shared.Metadata
}

// Alert represents an alerting rule for error monitoring
type Alert struct {
	ID        string
	ProjectID string
	Name      string

	// Conditions
	Conditions []AlertCondition
	Frequency  time.Duration // How often to check

	// Actions
	Actions []AlertAction

	// State
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time

	// Cooldown to prevent spam
	CooldownMinutes int
	LastTriggered   *time.Time
}

type AlertCondition struct {
	Type       string // "event_frequency", "new_issue", "error_level"
	Operator   string // "greater_than", "equals"
	Value      shared.Value
	TimeWindow time.Duration
}

type AlertAction struct {
	Type   string // "slack", "email", "webhook"
	Config shared.Metadata
}

// IsInCooldownPeriod checks if the alert is currently in its cooldown period
// DOMAIN BUSINESS RULE: An alert is in cooldown if it was triggered within the last CooldownMinutes
func (a *Alert) IsInCooldownPeriod() bool {
	if a.CooldownMinutes <= 0 {
		return false // No cooldown configured
	}

	if a.LastTriggered == nil {
		return false // Never triggered before
	}

	cooldownDuration := time.Duration(a.CooldownMinutes) * time.Minute
	cooldownUntil := a.LastTriggered.Add(cooldownDuration)
	return time.Now().Before(cooldownUntil)
}

// CanEvaluate returns true if the alert can be evaluated (not in cooldown)
// DOMAIN BUSINESS RULE: Alerts can only be evaluated if they're not in cooldown period
func (a *Alert) CanEvaluate() bool {
	return !a.IsInCooldownPeriod()
}

// ProjectStats represents aggregated statistics for a project
type ProjectStats struct {
	ProjectID      string
	Date           time.Time
	EventCount     int64
	IssueCount     int64
	UserCount      int64
	ErrorRate      float64
	NewIssues      int64
	ResolvedIssues int64
}

// IssueStats represents aggregated statistics for an issue
type IssueStats struct {
	IssueID         string
	Date            time.Time
	EventCount      int64
	UserCount       int64
	FirstOccurrence time.Time
	LastOccurrence  time.Time
}

// Constructor functions
func NewProject(workspaceID, teamID, name, slug, platform string) *Project {
	publicKey := generateRandomKey(32)
	secretKey := generateRandomKey(64)
	appKey := generateRandomKey(32)
	projectNumber := generateProjectNumber()

	return &Project{
		WorkspaceID:    workspaceID,
		TeamID:         teamID,
		Name:           name,
		Slug:           slug,
		Platform:       platform,
		Environment:    "production",
		Status:         "active",
		EventsPerHour:  1000,
		StorageQuotaMB: 1000,
		RetentionDays:  30,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		PublicKey:      publicKey,
		SecretKey:      secretKey,
		AppKey:         appKey,
		ProjectNumber:  projectNumber,
		// DSN format: https://{public_key}@{host}/{project_number}
		// Note: secret_key is optional in modern Sentry SDKs
		DSN: fmt.Sprintf("https://%s@movebigrocks.com/%d", publicKey, projectNumber),
	}
}

func NewIssue(projectID, title, culprit string, event *ErrorEvent) *Issue {
	shortID := generateShortID()
	fingerprint := GenerateFingerprint(event)

	return &Issue{
		ProjectID:     projectID, // Sentry compatibility
		ApplicationID: projectID, // Same value, clearer name
		Title:         title,
		Culprit:       culprit,
		Fingerprint:   fingerprint,
		Status:        "unresolved",
		Level:         event.Level,
		Type:          "error",
		FirstSeen:     event.Timestamp,
		LastSeen:      event.Timestamp,
		EventCount:    1,
		UserCount:     1,
		Tags:          make(map[string]string),
		ShortID:       shortID,
		Permalink:     fmt.Sprintf("/admin/extensions/error-tracking/issues/%s", shortID),
		Logger:        event.Logger,
		Platform:      event.Platform,
		LastEventID:   event.EventID,
	}
}

func NewErrorEvent(projectID, eventID string) *ErrorEvent {
	now := time.Now()

	return &ErrorEvent{
		EventID:   eventID,
		ProjectID: projectID,
		Timestamp: now,
		Received:  now,
		Tags:      make(map[string]string),
		Extra:     shared.NewMetadata(),
		Contexts:  shared.NewMetadata(),
	}
}

// Utility functions
func GenerateFingerprint(event *ErrorEvent) string {
	var parts []string

	// Use exception information if available
	if len(event.Exception) > 0 {
		exc := event.Exception[0]
		parts = append(parts, exc.Type)
		if exc.Stacktrace != nil && len(exc.Stacktrace.Frames) > 0 {
			frame := exc.Stacktrace.Frames[len(exc.Stacktrace.Frames)-1]
			parts = append(parts, frame.Filename, frame.Function)
		}
	}

	// Fallback to message
	if len(parts) == 0 {
		parts = append(parts, event.Message)
	}

	// Add platform and logger
	parts = append(parts, event.Platform, event.Logger)

	// Create hash
	sort.Strings(parts)
	content := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func generateRandomKey(length int) string {
	newID := id.New()
	hash := sha256.Sum256([]byte(newID))
	return hex.EncodeToString(hash[:])[:length]
}

func generateShortID() string {
	newID := id.New()
	hash := sha256.Sum256([]byte(newID))
	return hex.EncodeToString(hash[:])[:8]
}

func generateProjectNumber() int64 {
	// Generate a numeric project ID for Sentry SDK compatibility
	// Use timestamp microseconds + hash to ensure uniqueness
	timestamp := time.Now().UnixMicro()
	newID := id.New()
	hash := sha256.Sum256([]byte(newID))
	// Use first 4 bytes of hash as additional entropy (up to ~4 billion)
	hashPart := int64(hash[0])<<24 | int64(hash[1])<<16 | int64(hash[2])<<8 | int64(hash[3])
	// Combine: take lower bits from both to stay in safe int64 range
	return (timestamp % 1000000000) + (hashPart % 1000000)
}

// Validation methods
func (p *Project) IsActive() bool {
	return p.Status == "active"
}

func (p *Project) WithinQuota(currentEvents int) bool {
	return currentEvents < p.EventsPerHour
}

func (i *Issue) IsResolved() bool {
	return i.Status == "resolved"
}

func (i *Issue) IsMuted() bool {
	return i.Status == "muted" || i.Status == "ignored"
}

// MarkResolved transitions the issue to resolved status
func (i *Issue) MarkResolved(resolvedAt time.Time, resolvedBy, resolution string) error {
	if i.Status == IssueStatusResolved {
		return fmt.Errorf("issue is already resolved")
	}

	i.Status = IssueStatusResolved
	i.ResolvedAt = &resolvedAt
	i.ResolvedBy = resolvedBy

	if resolution != "" {
		i.Resolution = resolution
	} else {
		i.Resolution = "fixed"
	}

	return nil
}

// MarkResolvedInVersion marks the issue as resolved in a specific version
func (i *Issue) MarkResolvedInVersion(resolvedAt time.Time, resolvedBy, version, commit string) error {
	if err := i.MarkResolved(resolvedAt, resolvedBy, "fixed"); err != nil {
		return err
	}

	i.ResolvedInVersion = version
	i.ResolvedInCommit = commit

	return nil
}

// Reopen reopens a resolved issue
func (i *Issue) Reopen() error {
	if !i.IsResolved() {
		return fmt.Errorf("can only reopen resolved issues (current status: %s)", i.Status)
	}

	i.Status = IssueStatusUnresolved
	i.ResolvedAt = nil
	i.ResolvedBy = ""
	i.Resolution = ""
	i.ResolutionNotes = ""

	return nil
}

// MarkIgnored marks the issue as ignored
func (i *Issue) MarkIgnored() {
	i.Status = IssueStatusIgnored
}

// MarkMuted marks the issue as muted
func (i *Issue) MarkMuted() {
	i.Status = IssueStatusMuted
}

// Unmute removes mute/ignore status
func (i *Issue) Unmute() error {
	if !i.IsMuted() {
		return fmt.Errorf("issue is not muted (current status: %s)", i.Status)
	}

	i.Status = IssueStatusUnresolved
	return nil
}

// Assign assigns the issue to a user
func (i *Issue) Assign(userID string) {
	i.AssignedTo = userID
}

// Unassign removes the assignment
func (i *Issue) Unassign() {
	i.AssignedTo = ""
}

// RecordEvent records a new occurrence of this issue
func (i *Issue) RecordEvent(timestamp time.Time, eventID string, userID string) {
	i.EventCount++
	i.LastSeen = timestamp
	i.LastEventID = eventID

	// Track unique users
	if userID != "" {
		// Note: In a real implementation, you'd need to maintain a set of seen users
		// This is simplified - actual user count tracking would need additional logic
		i.UserCount++
	}
}

// UpdateLastSeen updates the last seen timestamp
func (i *Issue) UpdateLastSeen(timestamp time.Time, eventID string) {
	i.LastSeen = timestamp
	if eventID != "" {
		i.LastEventID = eventID
	}
}

// LinkCase links a support case to this issue
func (i *Issue) LinkCase(caseID string) {
	// Check if already linked
	for _, id := range i.RelatedCaseIDs {
		if id == caseID {
			return // Already linked
		}
	}

	i.RelatedCaseIDs = append(i.RelatedCaseIDs, caseID)
	i.HasRelatedCase = true
}

// UnlinkCase removes a case link
func (i *Issue) UnlinkCase(caseID string) {
	newCases := make([]string, 0, len(i.RelatedCaseIDs))
	for _, id := range i.RelatedCaseIDs {
		if id != caseID {
			newCases = append(newCases, id)
		}
	}

	i.RelatedCaseIDs = newCases
	i.HasRelatedCase = len(newCases) > 0
}

// ClearCaseLinks removes all linked support cases from this issue.
func (i *Issue) ClearCaseLinks() {
	i.RelatedCaseIDs = nil
	i.HasRelatedCase = false
}

// SetStatus applies a validated lifecycle transition to the issue.
func (i *Issue) SetStatus(status string, changedAt time.Time, changedBy string) error {
	switch status {
	case "", IssueStatusUnresolved:
		if i.IsResolved() {
			return i.Reopen()
		}
		if i.IsMuted() {
			return i.Unmute()
		}
		i.Status = IssueStatusUnresolved
		return nil
	case IssueStatusResolved:
		return i.MarkResolved(changedAt, changedBy, "fixed")
	case IssueStatusIgnored:
		i.MarkIgnored()
		return nil
	case IssueStatusMuted:
		i.MarkMuted()
		return nil
	default:
		return fmt.Errorf("invalid issue status: %s", status)
	}
}

// SetResolutionNotes adds notes about the resolution
func (i *Issue) SetResolutionNotes(notes string) {
	i.ResolutionNotes = notes
}

func (e *ErrorEvent) GetStoragePath() string {
	return fmt.Sprintf("events/%s/%s/%s.json",
		e.Timestamp.Format("2006/01/02"),
		e.ProjectID,
		e.EventID,
	)
}

func (e *ErrorEvent) ShouldStore() bool {
	// Store if event is large or has attachments
	return e.Size > 10*1024 || len(e.Breadcrumbs) > 50
}

// Constants for status values
const (
	ProjectStatusActive   = "active"
	ProjectStatusPaused   = "paused"
	ProjectStatusDisabled = "disabled"

	IssueStatusUnresolved = "unresolved"
	IssueStatusResolved   = "resolved"
	IssueStatusIgnored    = "ignored"
	IssueStatusMuted      = "muted"

	ErrorLevelFatal   = "fatal"
	ErrorLevelError   = "error"
	ErrorLevelWarning = "warning"
	ErrorLevelInfo    = "info"
	ErrorLevelDebug   = "debug"
)

// SentryEvent is an alias for ErrorEvent to maintain Sentry protocol compatibility
type SentryEvent = ErrorEvent
