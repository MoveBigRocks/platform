package servicedomain

import "time"

// EmailStats represents email statistics
type EmailStats struct {
	ID          string
	WorkspaceID string
	Date        time.Time // Daily stats

	// Send statistics
	EmailsSent       int
	EmailsDelivered  int
	EmailsBounced    int
	EmailsComplained int
	EmailsFailed     int

	// Receive statistics
	EmailsReceived  int
	EmailsProcessed int
	EmailsSpam      int
	EmailsBounces   int
	EmailsAutoReply int

	// Engagement statistics
	UniqueOpens  int
	TotalOpens   int
	UniqueClicks int
	TotalClicks  int

	// Performance metrics
	AverageDeliveryTime int64   // milliseconds
	DeliveryRate        float64 // percentage
	OpenRate            float64 // percentage
	ClickRate           float64 // percentage
	BounceRate          float64 // percentage
	ComplaintRate       float64 // percentage

	// Volume by hour (24-hour array)
	HourlyVolume []int

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// QuarantinedMessage represents a quarantined email message
type QuarantinedMessage struct {
	ID          string
	WorkspaceID string

	// Message details
	FromEmail  string
	Subject    string
	Content    string
	ReceivedAt time.Time

	// Quarantine reason
	QuarantineReason string // "spam", "virus", "policy", "manual"
	SpamScore        float64
	SpamReasons      []string
	VirusScanResult  string

	// Review status
	ReviewStatus string // "pending", "approved", "rejected"
	ReviewedAt   *time.Time
	ReviewedByID string
	ReviewNotes  string

	// Auto-release
	AutoReleaseAt *time.Time

	// Actions taken
	WasReleased bool
	ReleasedAt  *time.Time
	WasDeleted  bool

	// Original email data
	OriginalHeaders map[string]string
	AttachmentCount int

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}
