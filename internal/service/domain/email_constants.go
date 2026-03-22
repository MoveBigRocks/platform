package servicedomain

// EmailProvider represents different email service providers
type EmailProvider string

const (
	EmailProviderSendGrid EmailProvider = "sendgrid"
	EmailProviderSES      EmailProvider = "ses"
	EmailProviderSMTP     EmailProvider = "smtp"
	EmailProviderPostmark EmailProvider = "postmark"
)

// SenderStatus represents the status of an email sender
type SenderStatus string

const (
	SenderStatusActive    SenderStatus = "active"
	SenderStatusInactive  SenderStatus = "inactive"
	SenderStatusSuspended SenderStatus = "suspended"
	SenderStatusPending   SenderStatus = "pending"
)

// SenderType represents the type of sender
type SenderType string

const (
	SenderTypeUser   SenderType = "user"
	SenderTypeSystem SenderType = "system"
	SenderTypeAlias  SenderType = "alias"
	SenderTypeShared SenderType = "shared"
)

// EmailStatus represents the status of an email
type EmailStatus string

const (
	EmailStatusPending    EmailStatus = "pending"
	EmailStatusSending    EmailStatus = "sending"
	EmailStatusSent       EmailStatus = "sent"
	EmailStatusDelivered  EmailStatus = "delivered"
	EmailStatusBounced    EmailStatus = "bounced"
	EmailStatusComplained EmailStatus = "complained"
	EmailStatusFailed     EmailStatus = "failed"
	EmailStatusQueued     EmailStatus = "queued"
)

// EmailProcessingStatus represents the processing status of an inbound email
type EmailProcessingStatus string

const (
	EmailProcessingStatusPending    EmailProcessingStatus = "pending"
	EmailProcessingStatusProcessing EmailProcessingStatus = "processing"
	EmailProcessingStatusProcessed  EmailProcessingStatus = "processed"
	EmailProcessingStatusFailed     EmailProcessingStatus = "failed"
	EmailProcessingStatusSpam       EmailProcessingStatus = "spam"
)

// Sender default limits and thresholds
const (
	DefaultSenderDailyLimit         = 1000
	DefaultSenderHourlyLimit        = 100
	DefaultSenderWarmupDailyLimit   = 50
	DefaultSenderBounceThreshold    = 5.0  // percentage - auto-pause at 5%
	DefaultSenderComplaintThreshold = 0.5  // percentage - auto-pause at 0.5%
	DefaultSenderReputationScore    = 50.0 // neutral starting score
	DefaultSenderWarmupMultiplier   = 1.2  // 20% daily increase during warmup
)

// Reputation scoring weights
const (
	ReputationBaseScore       = 50.0
	ReputationDeliveryWeight  = 2.0  // bonus per % above 95% delivery
	ReputationBounceWeight    = 10.0 // penalty per % bounce rate
	ReputationComplaintWeight = 20.0 // penalty per % complaint rate
	ReputationDeliveryTarget  = 95.0 // target delivery rate for bonus
	ReputationMinScore        = 0.0
	ReputationMaxScore        = 100.0
)
