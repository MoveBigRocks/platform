//go:build integration

package servicehandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	graphmodel "github.com/movebigrocks/platform/internal/graph/model"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/service/resolvers"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
)

func setupPublicConversationRouter(handler *PublicConversationHandler) *gin.Engine {
	router := gin.New()
	conversations := router.Group("/v1/conversations")
	conversations.POST("", handler.StartConversation)
	conversations.POST("/:session_id/messages", handler.AddMessage)
	return router
}

func performConversationRequest(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestPublicConversationStartAndOperatorContinuation(t *testing.T) {
	store, cleanup := setupFormTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.Name = "Public Chat Workspace"
	workspace.Slug = "public-chat-" + workspace.Slug
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	operator := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, operator))

	queue := servicedomain.NewQueue(workspace.ID, "Support Inbox", "support-inbox", "Public support intake")
	require.NoError(t, store.Queues().CreateQueue(ctx, queue))

	conversationService := serviceapp.NewConversationService(
		store.Conversations(),
		store.Queues(),
		store.QueueItems(),
		store.Workspaces(),
		nil,
		store,
	)
	handler := NewPublicConversationHandler(conversationService, store.Users())
	router := setupPublicConversationRouter(handler)

	start := performConversationRequest(router, http.MethodPost, "/v1/conversations", map[string]any{
		"workspace_slug":  workspace.Slug,
		"queue_slug":      queue.Slug,
		"title":           "Billing help",
		"display_name":    "Casey Customer",
		"contact_email":   "casey@example.com",
		"initial_message": "I need help with an invoice question.",
		"source_ref":      "widget:website",
		"metadata": map[string]any{
			"surface_variant": "homepage",
		},
	})
	require.Equal(t, http.StatusCreated, start.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &created))
	sessionID, _ := created["session_id"].(string)
	externalSessionKey, _ := created["external_session_key"].(string)
	initialMessageID, _ := created["message_id"].(string)
	require.NotEmpty(t, sessionID)
	require.NotEmpty(t, externalSessionKey)
	require.NotEmpty(t, initialMessageID)

	session, err := store.Conversations().GetConversationSession(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.ConversationStatusWaiting, session.Status)
	assert.Equal(t, servicedomain.ConversationChannelWebChat, session.Channel)
	assert.Equal(t, "widget:website", session.SourceRef)
	assert.Equal(t, externalSessionKey, session.ExternalSessionKey)
	assert.Equal(t, queue.ID, session.Metadata.GetString("queue_id"))
	assert.Equal(t, queue.Slug, session.Metadata.GetString("queue_slug"))

	queueItem, err := store.QueueItems().GetQueueItemByConversationSessionID(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, queue.ID, queueItem.QueueID)

	publicParticipants, err := store.Conversations().ListConversationParticipants(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, publicParticipants, 1)
	assert.Equal(t, servicedomain.ConversationParticipantKindContact, publicParticipants[0].ParticipantKind)
	assert.Equal(t, "casey@example.com", publicParticipants[0].ParticipantRef)
	assert.Equal(t, "Casey Customer", publicParticipants[0].DisplayName)

	publicMessages, err := store.Conversations().ListConversationMessages(ctx, sessionID, servicedomain.ConversationMessageVisibilityCustomer)
	require.NoError(t, err)
	require.Len(t, publicMessages, 1)
	assert.Equal(t, "I need help with an invoice question.", publicMessages[0].ContentText)

	resolver := resolvers.NewResolver(resolvers.Config{
		ConversationService: conversationService,
	})
	operatorCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     operator,
		PrincipalType: platformdomain.PrincipalTypeUser,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Permissions: []string{
			platformdomain.PermissionConversationRead,
			platformdomain.PermissionConversationWrite,
		},
	})

	role := "assistant"
	visibility := "customer"
	replyText := "Hi Casey, I can help review the invoice details."
	operatorReply, err := resolver.AddConversationMessage(operatorCtx, sessionID, graphmodel.AddConversationMessageInput{
		Role:        &role,
		Visibility:  &visibility,
		ContentText: &replyText,
	})
	require.NoError(t, err)
	require.NotNil(t, operatorReply.ParticipantID())

	followUp := performConversationRequest(router, http.MethodPost, "/v1/conversations/"+sessionID+"/messages", map[string]any{
		"external_session_key": externalSessionKey,
		"contact_email":        "casey@example.com",
		"display_name":         "Casey Customer",
		"content":              "Thanks, the issue is on invoice INV-2048.",
	})
	require.Equal(t, http.StatusCreated, followUp.Code)

	var followUpResp map[string]any
	require.NoError(t, json.Unmarshal(followUp.Body.Bytes(), &followUpResp))
	followUpMessageID, _ := followUpResp["message_id"].(string)
	require.NotEmpty(t, followUpMessageID)

	participants, err := store.Conversations().ListConversationParticipants(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, participants, 2)

	var operatorParticipant *servicedomain.ConversationParticipant
	for _, participant := range participants {
		if participant.ParticipantKind == servicedomain.ConversationParticipantKindUser {
			operatorParticipant = participant
			break
		}
	}
	require.NotNil(t, operatorParticipant)
	assert.Equal(t, operator.ID, operatorParticipant.ParticipantRef)
	assert.Equal(t, operator.Name, operatorParticipant.DisplayName)

	messages, err := store.Conversations().ListConversationMessages(ctx, sessionID, servicedomain.ConversationMessageVisibilityCustomer)
	require.NoError(t, err)
	require.Len(t, messages, 3)

	workflowproof.WriteJSON(t, "public-conversation-intake", map[string]any{
		"workspace_id":            workspace.ID,
		"workspace_slug":          workspace.Slug,
		"queue_id":                queue.ID,
		"queue_slug":              queue.Slug,
		"queue_item_id":           queueItem.ID,
		"session_id":              session.ID,
		"external_session_key":    session.ExternalSessionKey,
		"status":                  session.Status,
		"initial_message_id":      initialMessageID,
		"operator_reply_id":       string(operatorReply.ID()),
		"operator_participant_id": operatorParticipant.ID,
		"follow_up_message_id":    followUpMessageID,
		"message_count":           len(messages),
	})
}
