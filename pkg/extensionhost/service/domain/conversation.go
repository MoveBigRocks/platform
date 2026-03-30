package servicedomain

import (
	"fmt"
	"time"

	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

type ConversationChannel string

const (
	ConversationChannelWebChat         ConversationChannel = "web_chat"
	ConversationChannelMobileChat      ConversationChannel = "mobile_chat"
	ConversationChannelEmail           ConversationChannel = "email"
	ConversationChannelOperatorConsole ConversationChannel = "operator_console"
)

type ConversationStatus string

const (
	ConversationStatusOpen      ConversationStatus = "open"
	ConversationStatusWaiting   ConversationStatus = "waiting"
	ConversationStatusEscalated ConversationStatus = "escalated"
	ConversationStatusResolved  ConversationStatus = "resolved"
	ConversationStatusClosed    ConversationStatus = "closed"
)

type ConversationParticipantKind string

const (
	ConversationParticipantKindAnonymousVisitor ConversationParticipantKind = "anonymous_visitor"
	ConversationParticipantKindContact          ConversationParticipantKind = "contact"
	ConversationParticipantKindUser             ConversationParticipantKind = "user"
	ConversationParticipantKindAgent            ConversationParticipantKind = "agent"
	ConversationParticipantKindRuntimeConnector ConversationParticipantKind = "runtime_connector"
)

type ConversationParticipantRole string

const (
	ConversationParticipantRoleRequester ConversationParticipantRole = "requester"
	ConversationParticipantRoleOperator  ConversationParticipantRole = "operator"
	ConversationParticipantRoleAssistant ConversationParticipantRole = "assistant"
	ConversationParticipantRoleObserver  ConversationParticipantRole = "observer"
)

type ConversationMessageRole string

const (
	ConversationMessageRoleUser      ConversationMessageRole = "user"
	ConversationMessageRoleAssistant ConversationMessageRole = "assistant"
	ConversationMessageRoleSystem    ConversationMessageRole = "system"
	ConversationMessageRoleTool      ConversationMessageRole = "tool"
)

type ConversationMessageKind string

const (
	ConversationMessageKindText       ConversationMessageKind = "text"
	ConversationMessageKindToolCall   ConversationMessageKind = "tool_call"
	ConversationMessageKindToolResult ConversationMessageKind = "tool_result"
	ConversationMessageKindCitation   ConversationMessageKind = "citation"
	ConversationMessageKindFormUpdate ConversationMessageKind = "form_update"
	ConversationMessageKindEvent      ConversationMessageKind = "event"
)

type ConversationMessageVisibility string

const (
	ConversationMessageVisibilityInternal ConversationMessageVisibility = "internal"
	ConversationMessageVisibilityCustomer ConversationMessageVisibility = "customer"
)

type ConversationOutcomeKind string

const (
	ConversationOutcomeKindResolvedInSession ConversationOutcomeKind = "resolved_in_session"
	ConversationOutcomeKindFormDrafted       ConversationOutcomeKind = "form_drafted"
	ConversationOutcomeKindFormSubmitted     ConversationOutcomeKind = "form_submitted"
	ConversationOutcomeKindCaseCreated       ConversationOutcomeKind = "case_created"
	ConversationOutcomeKindHandedToOperator  ConversationOutcomeKind = "handed_to_operator"
)

type ConversationSession struct {
	ID          string
	WorkspaceID string

	Channel            ConversationChannel
	Status             ConversationStatus
	Title              string
	LanguageCode       string
	SourceRef          string
	ExternalSessionKey string

	PrimaryContactID            string
	PrimaryCatalogNodeID        string
	ActiveFormSpecID            string
	ActiveFormSubmissionID      string
	LinkedCaseID                string
	HandlingTeamID              string
	AssignedOperatorUserID      string
	DelegatedRuntimeConnectorID string

	OpenedAt       time.Time
	LastActivityAt time.Time
	ClosedAt       *time.Time
	Metadata       shareddomain.TypedSchema
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewConversationSession(workspaceID string, channel ConversationChannel) *ConversationSession {
	now := time.Now().UTC()
	if channel == "" {
		channel = ConversationChannelWebChat
	}
	return &ConversationSession{
		WorkspaceID:    workspaceID,
		Channel:        channel,
		Status:         ConversationStatusOpen,
		Metadata:       shareddomain.NewTypedSchema(),
		OpenedAt:       now,
		LastActivityAt: now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (c *ConversationSession) Handoff(teamID, operatorUserID string) error {
	if teamID == "" && operatorUserID == "" {
		return fmt.Errorf("team_id or operator_user_id is required")
	}
	switch c.Status {
	case ConversationStatusResolved, ConversationStatusClosed:
		return fmt.Errorf("cannot hand off a closed conversation")
	}
	c.HandlingTeamID = teamID
	c.AssignedOperatorUserID = operatorUserID
	c.Status = ConversationStatusWaiting
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *ConversationSession) Escalate(caseID, teamID, operatorUserID string) error {
	if caseID == "" {
		return fmt.Errorf("case_id is required")
	}
	switch c.Status {
	case ConversationStatusResolved, ConversationStatusClosed:
		return fmt.Errorf("cannot escalate a closed conversation")
	}
	c.LinkedCaseID = caseID
	if teamID != "" {
		c.HandlingTeamID = teamID
	}
	if operatorUserID != "" {
		c.AssignedOperatorUserID = operatorUserID
	}
	c.Status = ConversationStatusEscalated
	c.UpdatedAt = time.Now().UTC()
	return nil
}

type ConversationParticipant struct {
	ID                    string
	WorkspaceID           string
	ConversationSessionID string
	ParticipantKind       ConversationParticipantKind
	ParticipantRef        string
	RoleInSession         ConversationParticipantRole
	DisplayName           string
	JoinedAt              time.Time
	LeftAt                *time.Time
	Metadata              shareddomain.TypedSchema
	CreatedAt             time.Time
}

func NewConversationParticipant(workspaceID, sessionID string, kind ConversationParticipantKind, participantRef string) *ConversationParticipant {
	now := time.Now().UTC()
	return &ConversationParticipant{
		WorkspaceID:           workspaceID,
		ConversationSessionID: sessionID,
		ParticipantKind:       kind,
		ParticipantRef:        participantRef,
		RoleInSession:         ConversationParticipantRoleRequester,
		Metadata:              shareddomain.NewTypedSchema(),
		JoinedAt:              now,
		CreatedAt:             now,
	}
}

type ConversationMessage struct {
	ID                    string
	WorkspaceID           string
	ConversationSessionID string
	ParticipantID         string
	Role                  ConversationMessageRole
	Kind                  ConversationMessageKind
	Visibility            ConversationMessageVisibility
	ContentText           string
	Content               shareddomain.TypedSchema
	CreatedAt             time.Time
}

func NewConversationMessage(workspaceID, sessionID string) *ConversationMessage {
	return &ConversationMessage{
		WorkspaceID:           workspaceID,
		ConversationSessionID: sessionID,
		Role:                  ConversationMessageRoleUser,
		Kind:                  ConversationMessageKindText,
		Visibility:            ConversationMessageVisibilityCustomer,
		Content:               shareddomain.NewTypedSchema(),
		CreatedAt:             time.Now().UTC(),
	}
}

type ConversationCatalogSuggestion struct {
	CatalogNodeID string
	Reason        string
	Confidence    float64
}

type ConversationWorkingState struct {
	ConversationSessionID    string
	WorkspaceID              string
	PrimaryCatalogNodeID     string
	SuggestedCatalogNodes    []ConversationCatalogSuggestion
	ClassificationConfidence *float64
	ActivePolicyProfileRef   string
	ActiveFormSpecID         string
	ActiveFormSubmissionID   string
	CollectedFields          shareddomain.TypedSchema
	MissingFields            shareddomain.TypedSchema
	RequiresOperatorReview   bool
	UpdatedAt                time.Time
}

func NewConversationWorkingState(workspaceID, sessionID string) *ConversationWorkingState {
	return &ConversationWorkingState{
		ConversationSessionID: sessionID,
		WorkspaceID:           workspaceID,
		SuggestedCatalogNodes: []ConversationCatalogSuggestion{},
		CollectedFields:       shareddomain.NewTypedSchema(),
		MissingFields:         shareddomain.NewTypedSchema(),
		UpdatedAt:             time.Now().UTC(),
	}
}

type ConversationOutcome struct {
	ID                    string
	WorkspaceID           string
	ConversationSessionID string
	Kind                  ConversationOutcomeKind
	ResultRef             shareddomain.TypedSchema
	CreatedAt             time.Time
}

func NewConversationOutcome(workspaceID, sessionID string, kind ConversationOutcomeKind) *ConversationOutcome {
	return &ConversationOutcome{
		WorkspaceID:           workspaceID,
		ConversationSessionID: sessionID,
		Kind:                  kind,
		ResultRef:             shareddomain.NewTypedSchema(),
		CreatedAt:             time.Now().UTC(),
	}
}

type ConversationSessionFilter struct {
	Status               ConversationStatus
	Channel              ConversationChannel
	PrimaryCatalogNodeID string
	PrimaryContactID     string
	LinkedCaseID         string
	Limit                int
	Offset               int
}
