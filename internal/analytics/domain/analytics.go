package analyticsdomain

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// Property is the aggregate root for web analytics tracking.
// Called "Property" (GA4 terminology) to avoid collision with existing "Application" and "Project".
type Property struct {
	ID          string
	WorkspaceID string
	Domain      string
	Timezone    string
	Status      string // "active" | "paused"
	VerifiedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewProperty creates a new analytics property with validation.
func NewProperty(workspaceID, domain, timezone string) (*Property, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if len(domain) > 253 {
		return nil, fmt.Errorf("domain exceeds maximum length of 253 characters")
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if timezone == "" {
		timezone = "UTC"
	}

	now := time.Now().UTC()
	return &Property{
		WorkspaceID: workspaceID,
		Domain:      domain,
		Timezone:    timezone,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// IsActive returns true if the property is actively collecting events.
func (p *Property) IsActive() bool {
	return p.Status == "active"
}

// IsPaused returns true if the property is paused.
func (p *Property) IsPaused() bool {
	return p.Status == "paused"
}

// IsVerified returns true if at least one event has been received.
func (p *Property) IsVerified() bool {
	return p.VerifiedAt != nil
}

// MarkVerified sets verified_at to now if not already verified.
func (p *Property) MarkVerified() {
	if p.VerifiedAt == nil {
		now := time.Now().UTC()
		p.VerifiedAt = &now
	}
}

// SnippetHTML returns the ready-to-paste tracking script tag.
func (p *Property) SnippetHTML(baseURL string) string {
	return fmt.Sprintf(`<script defer data-domain="%s" src="%s/js/analytics.js"></script>`, p.Domain, baseURL)
}

// AnalyticsEvent represents one pageview or custom event row.
type AnalyticsEvent struct {
	PropertyID     string
	VisitorID      int64
	Name           string // "pageview" or custom event name
	Pathname       string
	ReferrerSource string
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	CountryCode    string
	Region         string
	City           string
	Browser        string
	OS             string
	DeviceType     string
	Timestamp      time.Time
}

// Goal defines a conversion for a property. Two types: event-based and page-based.
type Goal struct {
	ID         string
	PropertyID string
	GoalType   string // "event" | "page"
	EventName  string // for event goals
	PagePath   string // for page goals
	CreatedAt  time.Time
}

// NewGoal creates a new goal with validation.
func NewGoal(propertyID, goalType, eventName, pagePath string) (*Goal, error) {
	if propertyID == "" {
		return nil, fmt.Errorf("property_id is required")
	}
	switch goalType {
	case "event":
		if eventName == "" {
			return nil, fmt.Errorf("event_name is required for event goals")
		}
	case "page":
		if pagePath == "" {
			return nil, fmt.Errorf("page_path is required for page goals")
		}
	default:
		return nil, fmt.Errorf("goal_type must be 'event' or 'page'")
	}

	return &Goal{
		PropertyID: propertyID,
		GoalType:   goalType,
		EventName:  eventName,
		PagePath:   pagePath,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// DisplayName returns the goal's display name for the UI.
func (g *Goal) DisplayName() string {
	if g.GoalType == "page" {
		return "Visit " + g.PagePath
	}
	return g.EventName
}

// ValidateGoalCount enforces max 20 goals per property.
func ValidateGoalCount(currentCount int) error {
	if currentCount >= 20 {
		return fmt.Errorf("maximum of 20 goals per property reached")
	}
	return nil
}

// Salt holds a random salt for visitor ID generation.
type Salt struct {
	ID        int
	Salt      []byte // 16 bytes
	CreatedAt time.Time
}

// NewSalt generates a new 16-byte random salt.
func NewSalt() (*Salt, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return &Salt{
		Salt:      salt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// Session represents an analytics session, computed at ingest time.
type Session struct {
	SessionID      int64
	PropertyID     string
	VisitorID      int64
	EntryPage      string
	ExitPage       string
	ReferrerSource string
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	CountryCode    string
	Region         string
	City           string
	Browser        string
	OS             string
	DeviceType     string
	StartedAt      time.Time
	LastActivity   time.Time
	Duration       int // seconds
	Pageviews      int
	IsBounce       int // 1 if pageviews == 1
}

// HostnameRule defines an allowed hostname pattern for a property.
type HostnameRule struct {
	ID         string
	PropertyID string
	Pattern    string // e.g. "example.com" or "*.example.com"
	CreatedAt  time.Time
}

// NewHostnameRule creates a new hostname rule with validation.
func NewHostnameRule(propertyID, pattern string) (*HostnameRule, error) {
	if propertyID == "" {
		return nil, fmt.Errorf("property_id is required")
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	if len(pattern) > 253 {
		return nil, fmt.Errorf("pattern exceeds maximum length of 253 characters")
	}

	return &HostnameRule{
		PropertyID: propertyID,
		Pattern:    pattern,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// ValidateHostnameRuleCount enforces max 10 rules per property.
func ValidateHostnameRuleCount(currentCount int) error {
	if currentCount >= 10 {
		return fmt.Errorf("maximum of 10 hostname rules per property reached")
	}
	return nil
}

// MatchesHostname checks if a hostname matches this rule's pattern.
// Supports exact match and wildcard prefix (e.g. "*.example.com" matches "sub.example.com").
func (r *HostnameRule) MatchesHostname(hostname string) bool {
	hostname = strings.ToLower(hostname)
	pattern := strings.ToLower(r.Pattern)

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(hostname, suffix)
	}
	return hostname == pattern
}

// MatchesAnyHostnameRule checks if a hostname matches any rule in the list.
// If rules is empty, all hostnames are accepted.
func MatchesAnyHostnameRule(rules []*HostnameRule, hostname string) bool {
	if len(rules) == 0 {
		return true
	}
	for _, rule := range rules {
		if rule.MatchesHostname(hostname) {
			return true
		}
	}
	return false
}
