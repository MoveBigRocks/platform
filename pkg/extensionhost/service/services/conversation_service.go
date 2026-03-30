package serviceapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/errors"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

type ConversationService struct {
	conversationStore shared.ConversationStore
	queueStore        shared.QueueStore
	queueItemStore    shared.QueueItemStore
	workspaceStore    shared.WorkspaceStore
	caseService       *CaseService
	tx                contracts.TransactionRunner
}

type AddConversationMessageParams struct {
	ParticipantID string
	Role          servicedomain.ConversationMessageRole
	Kind          servicedomain.ConversationMessageKind
	Visibility    servicedomain.ConversationMessageVisibility
	ContentText   string
	Content       shareddomain.TypedSchema
}

type HandoffConversationParams struct {
	TeamID           string
	QueueID          string
	OperatorUserID   string
	Reason           string
	PerformedByID    string
	PerformedByName  string
	PerformedByType  string
	OnBehalfOfUserID string
}

type EscalateConversationParams struct {
	TeamID           string
	QueueID          string
	OperatorUserID   string
	Subject          string
	Description      string
	Priority         servicedomain.CasePriority
	Category         string
	Reason           string
	PerformedByID    string
	PerformedByName  string
	PerformedByType  string
	OnBehalfOfUserID string
}

func NewConversationService(
	conversationStore shared.ConversationStore,
	queueStore shared.QueueStore,
	queueItemStore shared.QueueItemStore,
	workspaceStore shared.WorkspaceStore,
	caseService *CaseService,
	tx contracts.TransactionRunner,
) *ConversationService {
	return &ConversationService{
		conversationStore: conversationStore,
		queueStore:        queueStore,
		queueItemStore:    queueItemStore,
		workspaceStore:    workspaceStore,
		caseService:       caseService,
		tx:                tx,
	}
}

func (s *ConversationService) GetConversationSession(ctx context.Context, sessionID string) (*servicedomain.ConversationSession, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "required"))
	}
	session, err := s.conversationStore.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, apierrors.NotFoundError("conversation session", sessionID)
	}
	return session, nil
}

func (s *ConversationService) ListWorkspaceConversationSessions(ctx context.Context, workspaceID string, filter servicedomain.ConversationSessionFilter) ([]*servicedomain.ConversationSession, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	sessions, err := s.conversationStore.ListWorkspaceConversationSessions(ctx, workspaceID, filter)
	if err != nil {
		return nil, apierrors.DatabaseError("list conversation sessions", err)
	}
	return sessions, nil
}

func (s *ConversationService) ListConversationParticipants(ctx context.Context, sessionID string) ([]*servicedomain.ConversationParticipant, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "required"))
	}
	participants, err := s.conversationStore.ListConversationParticipants(ctx, sessionID)
	if err != nil {
		return nil, apierrors.DatabaseError("list conversation participants", err)
	}
	return participants, nil
}

func (s *ConversationService) ListConversationMessages(ctx context.Context, sessionID string, visibility servicedomain.ConversationMessageVisibility) ([]*servicedomain.ConversationMessage, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "required"))
	}
	messages, err := s.conversationStore.ListConversationMessages(ctx, sessionID, visibility)
	if err != nil {
		return nil, apierrors.DatabaseError("list conversation messages", err)
	}
	return messages, nil
}

func (s *ConversationService) GetConversationWorkingState(ctx context.Context, sessionID string) (*servicedomain.ConversationWorkingState, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "required"))
	}
	state, err := s.conversationStore.GetConversationWorkingState(ctx, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, nil
		}
		return nil, apierrors.DatabaseError("get conversation working state", err)
	}
	return state, nil
}

func (s *ConversationService) ListConversationOutcomes(ctx context.Context, sessionID string) ([]*servicedomain.ConversationOutcome, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "required"))
	}
	outcomes, err := s.conversationStore.ListConversationOutcomes(ctx, sessionID)
	if err != nil {
		return nil, apierrors.DatabaseError("list conversation outcomes", err)
	}
	return outcomes, nil
}

func (s *ConversationService) AddConversationMessage(ctx context.Context, sessionID string, params AddConversationMessageParams) (*servicedomain.ConversationMessage, error) {
	session, err := s.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ContentText) == "" && params.Content.IsEmpty() {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("content", "required"))
	}

	message := servicedomain.NewConversationMessage(session.WorkspaceID, session.ID)
	message.ID = id.New()
	message.ParticipantID = strings.TrimSpace(params.ParticipantID)
	if params.Role != "" {
		message.Role = params.Role
	}
	if params.Kind != "" {
		message.Kind = params.Kind
	}
	if params.Visibility != "" {
		message.Visibility = params.Visibility
	}
	message.ContentText = strings.TrimSpace(params.ContentText)
	message.Content = params.Content.Clone()
	message.CreatedAt = time.Now().UTC()

	session.LastActivityAt = message.CreatedAt
	session.UpdatedAt = message.CreatedAt

	run := func(txCtx context.Context) error {
		if err := s.conversationStore.CreateConversationMessage(txCtx, message); err != nil {
			return apierrors.DatabaseError("create conversation message", err)
		}
		if err := s.conversationStore.UpdateConversationSession(txCtx, session); err != nil {
			return apierrors.DatabaseError("update conversation session", err)
		}
		return nil
	}
	if s.tx != nil {
		if err := s.tx.WithTransaction(ctx, run); err != nil {
			return nil, err
		}
	} else if err := run(ctx); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *ConversationService) HandoffConversation(ctx context.Context, sessionID string, params HandoffConversationParams) (*servicedomain.ConversationSession, error) {
	session, err := s.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	queueID, err := s.validateConversationRouting(ctx, session.WorkspaceID, strings.TrimSpace(params.TeamID), strings.TrimSpace(params.QueueID))
	if err != nil {
		return nil, err
	}
	if err := session.Handoff(strings.TrimSpace(params.TeamID), strings.TrimSpace(params.OperatorUserID)); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "conversation handoff failed")
	}
	session.LastActivityAt = session.UpdatedAt

	outcome := servicedomain.NewConversationOutcome(session.WorkspaceID, session.ID, servicedomain.ConversationOutcomeKindHandedToOperator)
	outcome.ResultRef.Set("team_id", strings.TrimSpace(params.TeamID))
	outcome.ResultRef.Set("queue_id", queueID)
	outcome.ResultRef.Set("operator_user_id", strings.TrimSpace(params.OperatorUserID))
	outcome.ResultRef.Set("performed_by_id", strings.TrimSpace(params.PerformedByID))
	outcome.ResultRef.Set("performed_by_name", strings.TrimSpace(params.PerformedByName))
	outcome.ResultRef.Set("performed_by_type", strings.TrimSpace(params.PerformedByType))
	outcome.ResultRef.Set("routing_mode", routingMode(params.PerformedByType, params.OnBehalfOfUserID))
	if onBehalfOfUserID := strings.TrimSpace(params.OnBehalfOfUserID); onBehalfOfUserID != "" {
		outcome.ResultRef.Set("on_behalf_of_user_id", onBehalfOfUserID)
	}
	if reason := strings.TrimSpace(params.Reason); reason != "" {
		outcome.ResultRef.Set("reason", reason)
	}

	run := func(txCtx context.Context) error {
		if err := s.conversationStore.UpdateConversationSession(txCtx, session); err != nil {
			return apierrors.DatabaseError("update conversation session", err)
		}
		if err := s.conversationStore.CreateConversationOutcome(txCtx, outcome); err != nil {
			return apierrors.DatabaseError("create conversation outcome", err)
		}
		if err := s.syncConversationQueueItem(txCtx, session, queueID); err != nil {
			return err
		}
		return nil
	}
	if s.tx != nil {
		if err := s.tx.WithTransaction(ctx, run); err != nil {
			return nil, err
		}
	} else if err := run(ctx); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ConversationService) EscalateConversation(ctx context.Context, sessionID string, params EscalateConversationParams) (*servicedomain.Case, error) {
	if s.caseService == nil {
		return nil, fmt.Errorf("case service is not configured")
	}
	session, err := s.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(session.LinkedCaseID) != "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "conversation already escalated"))
	}
	queueID, err := s.validateConversationRouting(ctx, session.WorkspaceID, strings.TrimSpace(params.TeamID), strings.TrimSpace(params.QueueID))
	if err != nil {
		return nil, err
	}

	priority := params.Priority
	if priority == "" {
		priority = servicedomain.CasePriorityMedium
	}
	subject := strings.TrimSpace(params.Subject)
	if subject == "" {
		subject = strings.TrimSpace(session.Title)
	}
	if subject == "" {
		subject = "Conversation escalation"
	}
	description := strings.TrimSpace(params.Description)
	if description == "" {
		description = strings.TrimSpace(params.Reason)
	}
	if description == "" {
		description = fmt.Sprintf("Escalated from conversation %s", session.ID)
	}

	outcome := servicedomain.NewConversationOutcome(session.WorkspaceID, session.ID, servicedomain.ConversationOutcomeKindCaseCreated)
	outcome.ResultRef.Set("queue_id", queueID)
	outcome.ResultRef.Set("team_id", strings.TrimSpace(params.TeamID))
	outcome.ResultRef.Set("operator_user_id", strings.TrimSpace(params.OperatorUserID))
	outcome.ResultRef.Set("performed_by_id", strings.TrimSpace(params.PerformedByID))
	outcome.ResultRef.Set("performed_by_name", strings.TrimSpace(params.PerformedByName))
	outcome.ResultRef.Set("performed_by_type", strings.TrimSpace(params.PerformedByType))
	outcome.ResultRef.Set("routing_mode", routingMode(params.PerformedByType, params.OnBehalfOfUserID))
	if onBehalfOfUserID := strings.TrimSpace(params.OnBehalfOfUserID); onBehalfOfUserID != "" {
		outcome.ResultRef.Set("on_behalf_of_user_id", onBehalfOfUserID)
	}
	if reason := strings.TrimSpace(params.Reason); reason != "" {
		outcome.ResultRef.Set("reason", reason)
	}

	var caseObj *servicedomain.Case
	run := func(txCtx context.Context) error {
		createdCase, err := s.caseService.CreateCase(txCtx, CreateCaseParams{
			WorkspaceID:               session.WorkspaceID,
			Subject:                   subject,
			Description:               description,
			Priority:                  priority,
			Channel:                   caseChannelFromConversation(session.Channel),
			Category:                  strings.TrimSpace(params.Category),
			QueueID:                   queueID,
			OriginatingConversationID: session.ID,
			TeamID:                    strings.TrimSpace(params.TeamID),
			AssignedToID:              strings.TrimSpace(params.OperatorUserID),
		})
		if err != nil {
			return err
		}
		caseObj = createdCase
		if err := session.Escalate(caseObj.ID, strings.TrimSpace(params.TeamID), strings.TrimSpace(params.OperatorUserID)); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "conversation escalation failed")
		}
		session.LastActivityAt = session.UpdatedAt
		outcome.ResultRef.Set("case_id", caseObj.ID)
		if err := s.conversationStore.UpdateConversationSession(txCtx, session); err != nil {
			return apierrors.DatabaseError("update conversation session", err)
		}
		if err := s.conversationStore.CreateConversationOutcome(txCtx, outcome); err != nil {
			return apierrors.DatabaseError("create conversation outcome", err)
		}
		if err := s.deleteConversationQueueItem(txCtx, session.ID); err != nil {
			return err
		}
		return nil
	}
	if s.tx != nil {
		if err := s.tx.WithTransaction(ctx, run); err != nil {
			return nil, err
		}
	} else if err := run(ctx); err != nil {
		return nil, err
	}
	return caseObj, nil
}

func (s *ConversationService) validateConversationRouting(ctx context.Context, workspaceID, teamID, queueID string) (string, error) {
	if queueID == "" {
		return "", apierrors.NewValidationErrors(apierrors.NewValidationError("queue_id", "required"))
	}
	if s.queueStore == nil {
		return "", fmt.Errorf("queue store is not configured")
	}
	queue, err := s.queueStore.GetQueue(ctx, queueID)
	if err != nil || queue == nil || queue.WorkspaceID != workspaceID {
		return "", apierrors.NotFoundError("queue", queueID)
	}
	if teamID != "" && s.workspaceStore != nil {
		team, err := s.workspaceStore.GetTeam(ctx, teamID)
		if err != nil || team == nil || team.WorkspaceID != workspaceID {
			return "", apierrors.NotFoundError("team", teamID)
		}
	}
	return queueID, nil
}

func (s *ConversationService) syncConversationQueueItem(ctx context.Context, session *servicedomain.ConversationSession, queueID string) error {
	if s.queueItemStore == nil || session == nil {
		return nil
	}
	item, err := s.queueItemStore.GetQueueItemByConversationSessionID(ctx, session.ID)
	switch {
	case err == nil && item != nil:
		if item.QueueID == queueID {
			return nil
		}
		if err := item.MoveToQueue(queueID); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue item mutation failed")
		}
		if err := s.queueItemStore.UpdateQueueItem(ctx, item); err != nil {
			return apierrors.DatabaseError("update queue item", err)
		}
		return nil
	case errors.Is(err, shared.ErrNotFound):
		item = servicedomain.NewConversationQueueItem(session.WorkspaceID, queueID, session.ID)
		if err := item.Validate(); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue item validation failed")
		}
		if err := s.queueItemStore.CreateQueueItem(ctx, item); err != nil {
			return apierrors.DatabaseError("create queue item", err)
		}
		return nil
	case err != nil:
		return apierrors.DatabaseError("get queue item", err)
	default:
		return nil
	}
}

func (s *ConversationService) deleteConversationQueueItem(ctx context.Context, sessionID string) error {
	if s.queueItemStore == nil || sessionID == "" {
		return nil
	}
	if err := s.queueItemStore.DeleteQueueItemByConversationSessionID(ctx, sessionID); err != nil && !errors.Is(err, shared.ErrNotFound) {
		return apierrors.DatabaseError("delete queue item", err)
	}
	return nil
}

func caseChannelFromConversation(channel servicedomain.ConversationChannel) servicedomain.CaseChannel {
	switch channel {
	case servicedomain.ConversationChannelEmail:
		return servicedomain.CaseChannelEmail
	case servicedomain.ConversationChannelOperatorConsole:
		return servicedomain.CaseChannelInternal
	default:
		return servicedomain.CaseChannelChat
	}
}

func routingMode(performedByType, onBehalfOfUserID string) string {
	if strings.TrimSpace(performedByType) == "agent" {
		if strings.TrimSpace(onBehalfOfUserID) != "" {
			return "delegated_agent"
		}
		return "agent"
	}
	return "direct_user"
}

func NormalizeConversationStatus(value string) (servicedomain.ConversationStatus, error) {
	switch servicedomain.ConversationStatus(strings.ToLower(strings.TrimSpace(value))) {
	case "", servicedomain.ConversationStatusOpen:
		return servicedomain.ConversationStatusOpen, nil
	case servicedomain.ConversationStatusWaiting:
		return servicedomain.ConversationStatusWaiting, nil
	case servicedomain.ConversationStatusEscalated:
		return servicedomain.ConversationStatusEscalated, nil
	case servicedomain.ConversationStatusResolved:
		return servicedomain.ConversationStatusResolved, nil
	case servicedomain.ConversationStatusClosed:
		return servicedomain.ConversationStatusClosed, nil
	default:
		return "", fmt.Errorf("conversation status must be one of: open, waiting, escalated, resolved, closed")
	}
}

func NormalizeConversationChannel(value string) (servicedomain.ConversationChannel, error) {
	switch servicedomain.ConversationChannel(strings.ToLower(strings.TrimSpace(value))) {
	case "", servicedomain.ConversationChannelWebChat:
		return servicedomain.ConversationChannelWebChat, nil
	case servicedomain.ConversationChannelMobileChat:
		return servicedomain.ConversationChannelMobileChat, nil
	case servicedomain.ConversationChannelEmail:
		return servicedomain.ConversationChannelEmail, nil
	case servicedomain.ConversationChannelOperatorConsole:
		return servicedomain.ConversationChannelOperatorConsole, nil
	default:
		return "", fmt.Errorf("conversation channel must be one of: web_chat, mobile_chat, email, operator_console")
	}
}

func NormalizeConversationMessageRole(value string) (servicedomain.ConversationMessageRole, error) {
	switch servicedomain.ConversationMessageRole(strings.ToLower(strings.TrimSpace(value))) {
	case "", servicedomain.ConversationMessageRoleUser:
		return servicedomain.ConversationMessageRoleUser, nil
	case servicedomain.ConversationMessageRoleAssistant:
		return servicedomain.ConversationMessageRoleAssistant, nil
	case servicedomain.ConversationMessageRoleSystem:
		return servicedomain.ConversationMessageRoleSystem, nil
	case servicedomain.ConversationMessageRoleTool:
		return servicedomain.ConversationMessageRoleTool, nil
	default:
		return "", fmt.Errorf("conversation message role must be one of: user, assistant, system, tool")
	}
}

func NormalizeConversationMessageKind(value string) (servicedomain.ConversationMessageKind, error) {
	switch servicedomain.ConversationMessageKind(strings.ToLower(strings.TrimSpace(value))) {
	case "", servicedomain.ConversationMessageKindText:
		return servicedomain.ConversationMessageKindText, nil
	case servicedomain.ConversationMessageKindToolCall:
		return servicedomain.ConversationMessageKindToolCall, nil
	case servicedomain.ConversationMessageKindToolResult:
		return servicedomain.ConversationMessageKindToolResult, nil
	case servicedomain.ConversationMessageKindCitation:
		return servicedomain.ConversationMessageKindCitation, nil
	case servicedomain.ConversationMessageKindFormUpdate:
		return servicedomain.ConversationMessageKindFormUpdate, nil
	case servicedomain.ConversationMessageKindEvent:
		return servicedomain.ConversationMessageKindEvent, nil
	default:
		return "", fmt.Errorf("conversation message kind must be one of: text, tool_call, tool_result, citation, form_update, event")
	}
}

func NormalizeConversationMessageVisibility(value string) (servicedomain.ConversationMessageVisibility, error) {
	switch servicedomain.ConversationMessageVisibility(strings.ToLower(strings.TrimSpace(value))) {
	case "", servicedomain.ConversationMessageVisibilityCustomer:
		return servicedomain.ConversationMessageVisibilityCustomer, nil
	case servicedomain.ConversationMessageVisibilityInternal:
		return servicedomain.ConversationMessageVisibilityInternal, nil
	default:
		return "", fmt.Errorf("conversation message visibility must be one of: customer, internal")
	}
}
