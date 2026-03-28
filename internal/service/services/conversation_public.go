package serviceapp

import (
	"context"
	"errors"
	"strings"
	"time"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

const defaultPublicConversationSourceRef = "public:web_chat"

type StartPublicConversationParams struct {
	WorkspaceSlug      string
	QueueSlug          string
	ExternalSessionKey string
	Title              string
	DisplayName        string
	ContactEmail       string
	InitialMessage     string
	SourceRef          string
	Metadata           shareddomain.TypedSchema
}

type StartPublicConversationResult struct {
	Session     *servicedomain.ConversationSession
	Participant *servicedomain.ConversationParticipant
	Message     *servicedomain.ConversationMessage
	QueueID     string
	QueueSlug   string
	Created     bool
}

type AppendPublicConversationMessageParams struct {
	ExternalSessionKey string
	DisplayName        string
	ContactEmail       string
	ContentText        string
}

type AppendPublicConversationMessageResult struct {
	Session     *servicedomain.ConversationSession
	Participant *servicedomain.ConversationParticipant
	Message     *servicedomain.ConversationMessage
	QueueID     string
}

type EnsureConversationParticipantParams struct {
	ParticipantKind servicedomain.ConversationParticipantKind
	ParticipantRef  string
	RoleInSession   servicedomain.ConversationParticipantRole
	DisplayName     string
}

func (s *ConversationService) StartPublicConversation(ctx context.Context, params StartPublicConversationParams) (*StartPublicConversationResult, error) {
	if s.conversationStore == nil || s.queueStore == nil || s.workspaceStore == nil {
		return nil, apierrors.New(apierrors.ErrorTypeInternal, "public conversation intake is not configured")
	}

	workspaceSlug := strings.TrimSpace(params.WorkspaceSlug)
	if workspaceSlug == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_slug", "required"))
	}
	queueSlug := strings.TrimSpace(params.QueueSlug)
	if queueSlug == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("queue_slug", "required"))
	}

	workspace, err := s.workspaceStore.GetWorkspaceBySlug(ctx, workspaceSlug)
	if err != nil || workspace == nil {
		return nil, apierrors.NotFoundError("workspace", workspaceSlug)
	}
	queue, err := s.queueStore.GetQueueBySlug(ctx, workspace.ID, queueSlug)
	if err != nil || queue == nil {
		return nil, apierrors.NotFoundError("queue", queueSlug)
	}

	externalSessionKey := strings.TrimSpace(params.ExternalSessionKey)
	if externalSessionKey == "" {
		externalSessionKey = id.New()
	}

	session, err := s.conversationStore.GetConversationSessionByExternalKey(ctx, workspace.ID, servicedomain.ConversationChannelWebChat, externalSessionKey)
	switch {
	case err == nil:
	case errors.Is(err, shared.ErrNotFound):
		session = nil
	default:
		return nil, apierrors.DatabaseError("get conversation session", err)
	}

	if session == nil && strings.TrimSpace(params.InitialMessage) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("initial_message", "required for a new public conversation"))
	}
	if session != nil {
		if err := validatePublicConversationSessionWritable(session); err != nil {
			return nil, err
		}
	}

	result := &StartPublicConversationResult{
		QueueID:   queue.ID,
		QueueSlug: queue.Slug,
		Created:   session == nil,
	}

	if session == nil {
		session = servicedomain.NewConversationSession(workspace.ID, servicedomain.ConversationChannelWebChat)
		session.Status = servicedomain.ConversationStatusWaiting
		session.ExternalSessionKey = externalSessionKey
		session.Title = strings.TrimSpace(params.Title)
		session.SourceRef = normalizePublicConversationSourceRef(params.SourceRef)
		session.Metadata = normalizePublicConversationMetadata(params.Metadata, workspaceSlug, queue, session.SourceRef)
	} else {
		session.Metadata = mergePublicConversationMetadata(session.Metadata, params.Metadata, queue, workspaceSlug)
		if strings.TrimSpace(params.Title) != "" && strings.TrimSpace(session.Title) == "" {
			session.Title = strings.TrimSpace(params.Title)
		}
		if strings.TrimSpace(session.SourceRef) == "" {
			session.SourceRef = normalizePublicConversationSourceRef(params.SourceRef)
		}
		if session.Status == servicedomain.ConversationStatusOpen {
			session.Status = servicedomain.ConversationStatusWaiting
		}
	}

	run := func(txCtx context.Context) error {
		var sessionNeedsUpdate bool
		if result.Created {
			if err := s.conversationStore.CreateConversationSession(txCtx, session); err != nil {
				return apierrors.DatabaseError("create conversation session", err)
			}
		} else {
			sessionNeedsUpdate = true
		}

		participant, err := s.resolvePublicConversationParticipant(txCtx, session, params.DisplayName, params.ContactEmail)
		if err != nil {
			return err
		}
		result.Participant = participant

		if content := strings.TrimSpace(params.InitialMessage); content != "" {
			message := newPublicConversationMessage(session, participant, content)
			if err := s.conversationStore.CreateConversationMessage(txCtx, message); err != nil {
				return apierrors.DatabaseError("create conversation message", err)
			}
			session.LastActivityAt = message.CreatedAt
			session.UpdatedAt = message.CreatedAt
			sessionNeedsUpdate = true
			result.Message = message
		}

		if err := s.syncConversationQueueItem(txCtx, session, queue.ID); err != nil {
			return err
		}

		if sessionNeedsUpdate {
			if err := s.conversationStore.UpdateConversationSession(txCtx, session); err != nil {
				return apierrors.DatabaseError("update conversation session", err)
			}
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

	result.Session = session
	return result, nil
}

func (s *ConversationService) AppendPublicConversationMessage(ctx context.Context, sessionID string, params AppendPublicConversationMessageParams) (*AppendPublicConversationMessageResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "required"))
	}
	content := strings.TrimSpace(params.ContentText)
	if content == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("content", "required"))
	}

	session, err := s.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.Channel != servicedomain.ConversationChannelWebChat {
		return nil, apierrors.NotFoundError("conversation session", sessionID)
	}
	if strings.TrimSpace(params.ExternalSessionKey) == "" || session.ExternalSessionKey != strings.TrimSpace(params.ExternalSessionKey) {
		return nil, apierrors.NotFoundError("conversation session", sessionID)
	}
	if err := validatePublicConversationSessionWritable(session); err != nil {
		return nil, err
	}

	queueID := strings.TrimSpace(session.Metadata.GetString("queue_id"))
	if queueID == "" {
		return nil, apierrors.New(apierrors.ErrorTypeInternal, "conversation routing metadata is incomplete")
	}

	result := &AppendPublicConversationMessageResult{
		Session: session,
		QueueID: queueID,
	}

	run := func(txCtx context.Context) error {
		participant, err := s.resolvePublicConversationParticipant(txCtx, session, params.DisplayName, params.ContactEmail)
		if err != nil {
			return err
		}
		result.Participant = participant

		message := newPublicConversationMessage(session, participant, content)
		if err := s.conversationStore.CreateConversationMessage(txCtx, message); err != nil {
			return apierrors.DatabaseError("create conversation message", err)
		}
		if session.Status == servicedomain.ConversationStatusOpen {
			session.Status = servicedomain.ConversationStatusWaiting
		}
		session.LastActivityAt = message.CreatedAt
		session.UpdatedAt = message.CreatedAt
		if err := s.syncConversationQueueItem(txCtx, session, queueID); err != nil {
			return err
		}
		if err := s.conversationStore.UpdateConversationSession(txCtx, session); err != nil {
			return apierrors.DatabaseError("update conversation session", err)
		}
		result.Message = message
		return nil
	}

	if s.tx != nil {
		if err := s.tx.WithTransaction(ctx, run); err != nil {
			return nil, err
		}
	} else if err := run(ctx); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ConversationService) EnsureConversationParticipant(ctx context.Context, sessionID string, params EnsureConversationParticipantParams) (*servicedomain.ConversationParticipant, error) {
	session, err := s.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.ensureConversationParticipant(ctx, session, params)
}

func (s *ConversationService) resolvePublicConversationParticipant(ctx context.Context, session *servicedomain.ConversationSession, displayName, contactEmail string) (*servicedomain.ConversationParticipant, error) {
	participantKind, participantRef := publicConversationParticipantIdentity(contactEmail, session.ExternalSessionKey)
	return s.ensureConversationParticipant(ctx, session, EnsureConversationParticipantParams{
		ParticipantKind: participantKind,
		ParticipantRef:  participantRef,
		RoleInSession:   servicedomain.ConversationParticipantRoleRequester,
		DisplayName:     displayName,
	})
}

func validatePublicConversationSessionWritable(session *servicedomain.ConversationSession) error {
	switch session.Status {
	case servicedomain.ConversationStatusEscalated:
		return apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "conversation has already escalated into a case"))
	case servicedomain.ConversationStatusResolved, servicedomain.ConversationStatusClosed:
		return apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "conversation is no longer accepting public messages"))
	default:
		return nil
	}
}

func normalizePublicConversationSourceRef(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return defaultPublicConversationSourceRef
}

func normalizePublicConversationMetadata(metadata shareddomain.TypedSchema, workspaceSlug string, queue *servicedomain.Queue, sourceRef string) shareddomain.TypedSchema {
	normalized := metadata.Clone()
	normalized.Set("surface", "public_web_chat")
	normalized.Set("workspace_slug", workspaceSlug)
	normalized.Set("queue_id", queue.ID)
	normalized.Set("queue_slug", queue.Slug)
	normalized.Set("source_ref", sourceRef)
	return normalized
}

func mergePublicConversationMetadata(existing, incoming shareddomain.TypedSchema, queue *servicedomain.Queue, workspaceSlug string) shareddomain.TypedSchema {
	merged := existing.Clone()
	for key, value := range incoming.ToMap() {
		merged.Set(key, value)
	}
	merged.Set("surface", "public_web_chat")
	merged.Set("workspace_slug", workspaceSlug)
	merged.Set("queue_id", queue.ID)
	merged.Set("queue_slug", queue.Slug)
	if sourceRef := strings.TrimSpace(merged.GetString("source_ref")); sourceRef == "" {
		merged.Set("source_ref", defaultPublicConversationSourceRef)
	}
	return merged
}

func publicConversationParticipantIdentity(contactEmail, externalSessionKey string) (servicedomain.ConversationParticipantKind, string) {
	if email := strings.TrimSpace(contactEmail); email != "" {
		return servicedomain.ConversationParticipantKindContact, email
	}
	return servicedomain.ConversationParticipantKindAnonymousVisitor, strings.TrimSpace(externalSessionKey)
}

func (s *ConversationService) ensureConversationParticipant(ctx context.Context, session *servicedomain.ConversationSession, params EnsureConversationParticipantParams) (*servicedomain.ConversationParticipant, error) {
	if session == nil {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("session_id", "required"))
	}
	if strings.TrimSpace(params.ParticipantRef) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("participant_ref", "required"))
	}
	participantKind := params.ParticipantKind
	if participantKind == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("participant_kind", "required"))
	}
	roleInSession := params.RoleInSession
	if roleInSession == "" {
		roleInSession = servicedomain.ConversationParticipantRoleObserver
	}

	participants, err := s.conversationStore.ListConversationParticipants(ctx, session.ID)
	if err != nil {
		return nil, apierrors.DatabaseError("list conversation participants", err)
	}
	for _, participant := range participants {
		if participant == nil {
			continue
		}
		if participant.ParticipantKind == participantKind && participant.ParticipantRef == strings.TrimSpace(params.ParticipantRef) && participant.RoleInSession == roleInSession {
			return participant, nil
		}
	}

	participant := servicedomain.NewConversationParticipant(session.WorkspaceID, session.ID, participantKind, strings.TrimSpace(params.ParticipantRef))
	participant.RoleInSession = roleInSession
	participant.DisplayName = strings.TrimSpace(params.DisplayName)
	if err := s.conversationStore.CreateConversationParticipant(ctx, participant); err != nil {
		return nil, apierrors.DatabaseError("create conversation participant", err)
	}
	return participant, nil
}

func newPublicConversationMessage(session *servicedomain.ConversationSession, participant *servicedomain.ConversationParticipant, content string) *servicedomain.ConversationMessage {
	message := servicedomain.NewConversationMessage(session.WorkspaceID, session.ID)
	message.ParticipantID = participant.ID
	message.Role = servicedomain.ConversationMessageRoleUser
	message.Kind = servicedomain.ConversationMessageKindText
	message.Visibility = servicedomain.ConversationMessageVisibilityCustomer
	message.ContentText = strings.TrimSpace(content)
	message.CreatedAt = time.Now().UTC()
	return message
}
