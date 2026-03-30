package shareddomain

// =============================================================================
// Type-Safe Enums
// Replaces magic strings with compile-time checked types
// =============================================================================

// CommunicationType represents the type of communication
type CommunicationType string

const (
	CommTypeEmail  CommunicationType = "email"
	CommTypeNote   CommunicationType = "note"
	CommTypeSystem CommunicationType = "system"
	CommTypePhone  CommunicationType = "phone"
	CommTypeChat   CommunicationType = "chat"
	CommTypePortal CommunicationType = "portal"
)

// IsValid returns true if the communication type is valid
func (ct CommunicationType) IsValid() bool {
	switch ct {
	case CommTypeEmail, CommTypeNote, CommTypeSystem, CommTypePhone, CommTypeChat, CommTypePortal:
		return true
	}
	return false
}

// String returns the string representation
func (ct CommunicationType) String() string {
	return string(ct)
}

// Direction represents the direction of a communication or flow
type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
	DirectionInternal Direction = "internal"
)

// IsValid returns true if the direction is valid
func (d Direction) IsValid() bool {
	switch d {
	case DirectionInbound, DirectionOutbound, DirectionInternal:
		return true
	}
	return false
}

// String returns the string representation
func (d Direction) String() string {
	return string(d)
}

// TriggerType represents what triggered a rule or action
type TriggerType string

const (
	TriggerTypeCaseCreated   TriggerType = "case_created"
	TriggerTypeCaseUpdated   TriggerType = "case_updated"
	TriggerTypeCaseAssigned  TriggerType = "case_assigned"
	TriggerTypeCaseResolved  TriggerType = "case_resolved"
	TriggerTypeTimeBased     TriggerType = "time_based"
	TriggerTypeEmailReceived TriggerType = "email_received"
	TriggerTypeManual        TriggerType = "manual"
	TriggerTypeScheduled     TriggerType = "scheduled"
	TriggerTypeWebhook       TriggerType = "webhook"
	TriggerTypeAPI           TriggerType = "api"
)

// IsValid returns true if the trigger type is valid
func (tt TriggerType) IsValid() bool {
	switch tt {
	case TriggerTypeCaseCreated, TriggerTypeCaseUpdated, TriggerTypeCaseAssigned,
		TriggerTypeCaseResolved, TriggerTypeTimeBased, TriggerTypeEmailReceived,
		TriggerTypeManual, TriggerTypeScheduled, TriggerTypeWebhook, TriggerTypeAPI:
		return true
	}
	return false
}

// String returns the string representation
func (tt TriggerType) String() string {
	return string(tt)
}

// RuleConditionType represents the type of a rule condition
type RuleConditionType string

const (
	ConditionTypeField    RuleConditionType = "field"
	ConditionTypeTime     RuleConditionType = "time"
	ConditionTypeCustom   RuleConditionType = "custom"
	ConditionTypeContact  RuleConditionType = "contact"
	ConditionTypeWorkflow RuleConditionType = "workflow"
)

// IsValid returns true if the condition type is valid
func (rct RuleConditionType) IsValid() bool {
	switch rct {
	case ConditionTypeField, ConditionTypeTime, ConditionTypeCustom,
		ConditionTypeContact, ConditionTypeWorkflow:
		return true
	}
	return false
}

// String returns the string representation
func (rct RuleConditionType) String() string {
	return string(rct)
}

// Operator represents a comparison operator
type Operator string

const (
	OpEquals      Operator = "equals"
	OpNotEquals   Operator = "not_equals"
	OpContains    Operator = "contains"
	OpNotContains Operator = "not_contains"
	OpStartsWith  Operator = "starts_with"
	OpEndsWith    Operator = "ends_with"
	OpGreaterThan Operator = "greater_than"
	OpLessThan    Operator = "less_than"
	OpGreaterOrEq Operator = "greater_or_equal"
	OpLessOrEq    Operator = "less_or_equal"
	OpIn          Operator = "in"
	OpNotIn       Operator = "not_in"
	OpIsEmpty     Operator = "is_empty"
	OpIsNotEmpty  Operator = "is_not_empty"
	OpExists      Operator = "exists"
	OpNotExists   Operator = "not_exists"
	OpChanged     Operator = "changed"
	OpMatches     Operator = "matches" // regex
)

// IsValid returns true if the operator is valid
func (o Operator) IsValid() bool {
	switch o {
	case OpEquals, OpNotEquals, OpContains, OpNotContains, OpStartsWith, OpEndsWith,
		OpGreaterThan, OpLessThan, OpGreaterOrEq, OpLessOrEq, OpIn, OpNotIn,
		OpIsEmpty, OpIsNotEmpty, OpExists, OpNotExists, OpChanged, OpMatches:
		return true
	}
	return false
}

// String returns the string representation
func (o Operator) String() string {
	return string(o)
}

// RuleActionType represents the type of action a rule can take
type RuleActionType string

const (
	ActionTypeSetStatus      RuleActionType = "set_status"
	ActionTypeSetPriority    RuleActionType = "set_priority"
	ActionTypeAssign         RuleActionType = "assign"
	ActionTypeSetTeam        RuleActionType = "set_team"
	ActionTypeAddTag         RuleActionType = "add_tag"
	ActionTypeRemoveTag      RuleActionType = "remove_tag"
	ActionTypeSetCategory    RuleActionType = "set_category"
	ActionTypeSetCustomField RuleActionType = "set_custom_field"
	ActionTypeSendNotify     RuleActionType = "notify"
	ActionTypeSendEmail      RuleActionType = "send_email"
	ActionTypeEscalate       RuleActionType = "escalate"
	ActionTypeMute           RuleActionType = "mute"
	ActionTypeAddNote        RuleActionType = "add_note"
	ActionTypeWebhook        RuleActionType = "webhook"
	ActionTypeCreateCase     RuleActionType = "create_case"
	ActionTypeLinkIssue      RuleActionType = "link_issue"
	ActionTypeAIClassify     RuleActionType = "ai_classify"
	ActionTypeAISuggest      RuleActionType = "ai_suggest"
)

// IsValid returns true if the action type is valid
func (at RuleActionType) IsValid() bool {
	switch at {
	case ActionTypeSetStatus, ActionTypeSetPriority, ActionTypeAssign, ActionTypeSetTeam,
		ActionTypeAddTag, ActionTypeRemoveTag, ActionTypeSetCategory, ActionTypeSetCustomField,
		ActionTypeSendNotify, ActionTypeSendEmail, ActionTypeEscalate, ActionTypeMute,
		ActionTypeAddNote, ActionTypeWebhook, ActionTypeCreateCase, ActionTypeLinkIssue,
		ActionTypeAIClassify, ActionTypeAISuggest:
		return true
	}
	return false
}

// String returns the string representation
func (at RuleActionType) String() string {
	return string(at)
}

// LogicalOperator represents logical operators for combining conditions
type LogicalOperator string

const (
	LogicalAnd LogicalOperator = "and"
	LogicalOr  LogicalOperator = "or"
)

// IsValid returns true if the logical operator is valid
func (lo LogicalOperator) IsValid() bool {
	switch lo {
	case LogicalAnd, LogicalOr:
		return true
	}
	return false
}

// String returns the string representation
func (lo LogicalOperator) String() string {
	return string(lo)
}

// EntityType represents the type of entity
type EntityType string

const (
	EntityTypeCase       EntityType = "case"
	EntityTypeContact    EntityType = "contact"
	EntityTypeUser       EntityType = "user"
	EntityTypeWorkspace  EntityType = "workspace"
	EntityTypeTeam       EntityType = "team"
	EntityTypeIssue      EntityType = "issue"
	EntityTypeProject    EntityType = "project"
	EntityTypeRule       EntityType = "rule"
	EntityTypeJob        EntityType = "job"
	EntityTypeEmail      EntityType = "email"
	EntityTypeAttachment EntityType = "attachment"
	EntityTypeKnowledge  EntityType = "knowledge_resource"
	EntityTypeForm       EntityType = "form"
)

// SourceType represents the source of an action or event
type SourceType string

const (
	SourceTypeCustomerEmail SourceType = "customer_email"
	SourceTypeAutoMonitor   SourceType = "auto_monitoring"
	SourceTypeManual        SourceType = "manual"
	SourceTypeAPI           SourceType = "api"
	SourceTypeWebhook       SourceType = "webhook"
	SourceTypeForm          SourceType = "form"
	SourceTypeImport        SourceType = "import"
	SourceTypeSystem        SourceType = "system"
	SourceTypeRule          SourceType = "rule"
	SourceTypeAI            SourceType = "ai"
)

// IsValid returns true if the source type is valid
func (st SourceType) IsValid() bool {
	switch st {
	case SourceTypeCustomerEmail, SourceTypeAutoMonitor, SourceTypeManual,
		SourceTypeAPI, SourceTypeWebhook, SourceTypeForm, SourceTypeImport,
		SourceTypeSystem, SourceTypeRule, SourceTypeAI:
		return true
	}
	return false
}

// String returns the string representation
func (st SourceType) String() string {
	return string(st)
}

// NotificationChannel represents a notification delivery channel
type NotificationChannel string

const (
	NotifyChannelEmail   NotificationChannel = "email"
	NotifyChannelSlack   NotificationChannel = "slack"
	NotifyChannelWebhook NotificationChannel = "webhook"
	NotifyChannelInApp   NotificationChannel = "in_app"
	NotifyChannelSMS     NotificationChannel = "sms"
	NotifyChannelPush    NotificationChannel = "push"
)

// SpamStatus represents the spam classification status
type SpamStatus string

const (
	SpamStatusClean   SpamStatus = "clean"
	SpamStatusSuspect SpamStatus = "suspect"
	SpamStatusSpam    SpamStatus = "spam"
	SpamStatusUnknown SpamStatus = "unknown"
)

// VirusScanStatus represents the virus scan status of a file
type VirusScanStatus string

const (
	ScanStatusPending  VirusScanStatus = "pending"
	ScanStatusClean    VirusScanStatus = "clean"
	ScanStatusInfected VirusScanStatus = "infected"
	ScanStatusError    VirusScanStatus = "error"
	ScanStatusSkipped  VirusScanStatus = "skipped"
)

// IssueLevel represents the severity level of an error/issue
type IssueLevel string

const (
	IssueLevelDebug   IssueLevel = "debug"
	IssueLevelInfo    IssueLevel = "info"
	IssueLevelWarning IssueLevel = "warning"
	IssueLevelError   IssueLevel = "error"
	IssueLevelFatal   IssueLevel = "fatal"
)

// IssueStatus represents the status of an error issue
type IssueStatus string

const (
	IssueStatusUnresolved IssueStatus = "unresolved"
	IssueStatusResolved   IssueStatus = "resolved"
	IssueStatusIgnored    IssueStatus = "ignored"
	IssueStatusMuted      IssueStatus = "muted"
)
