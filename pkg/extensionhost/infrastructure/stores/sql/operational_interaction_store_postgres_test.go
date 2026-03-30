package sql_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestOperationalInteractionStoresRoundTrip(t *testing.T) {
	testDSN, cleanupDatabase := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDatabase()

	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: testDSN})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	store, err := sqlstore.NewStore(db)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	team := &platformdomain.Team{
		WorkspaceID: workspace.ID,
		Name:        "Support",
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Workspaces().CreateTeam(ctx, team))

	queue := servicedomain.NewQueue(workspace.ID, "Billing", "billing", "Billing work queue")
	require.NoError(t, store.Queues().CreateQueue(ctx, queue))

	rootNode := servicedomain.NewServiceCatalogNode(workspace.ID, "support", "Support")
	rootNode.PathSlug = "support"
	rootNode.NodeKind = servicedomain.ServiceCatalogNodeKindDomain
	rootNode.SupportedChannels = []string{"web_chat", "email"}
	rootNode.SearchKeywords = []string{"support", "help"}
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogNode(ctx, rootNode))

	childNode := servicedomain.NewServiceCatalogNode(workspace.ID, "refunds", "Refund Requests")
	childNode.ParentNodeID = rootNode.ID
	childNode.PathSlug = "support/refunds"
	childNode.NodeKind = servicedomain.ServiceCatalogNodeKindRequestType
	childNode.DefaultQueueID = queue.ID
	childNode.DefaultCaseCategory = "billing"
	childNode.DefaultPriority = string(servicedomain.CasePriorityHigh)
	childNode.SupportedChannels = []string{"web_chat"}
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogNode(ctx, childNode))

	loadedNode, err := store.ServiceCatalog().GetServiceCatalogNodeByPath(ctx, workspace.ID, "support/refunds")
	require.NoError(t, err)
	assert.Equal(t, childNode.ID, loadedNode.ID)
	assert.Equal(t, []string{"web_chat"}, loadedNode.SupportedChannels)

	children, err := store.ServiceCatalog().ListChildServiceCatalogNodes(ctx, workspace.ID, rootNode.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, childNode.ID, children[0].ID)

	resource := knowledgedomain.NewKnowledgeResource(workspace.ID, team.ID, "refund-policy", "Refund Policy")
	resource.Kind = knowledgedomain.KnowledgeResourceKindPolicy
	resource.Status = knowledgedomain.KnowledgeResourceStatusActive
	resource.BodyMarkdown = "# Refunds\n\nWe review refund requests within 3 business days."
	resource.Summary = "Explains refund handling."
	resource.SupportedChannels = []string{"web_chat"}
	resource.SearchKeywords = []string{"refund", "billing"}
	resource.Frontmatter.Set("applies_when", map[string]interface{}{"catalog_path": "support/refunds"})
	require.NoError(t, store.KnowledgeResources().CreateKnowledgeResource(ctx, resource))

	spec := servicedomain.NewFormSpec(workspace.ID, "refund-request", "Refund Request")
	spec.Status = servicedomain.FormSpecStatusActive
	spec.PublicKey = "refund-request-public"
	spec.DescriptionMarkdown = "Collect the details needed to review a refund request."
	spec.SupportedChannels = []string{"web_chat", "operator_console"}
	spec.FieldSpec.Set("type", "object")
	spec.FieldSpec.Set("required", []string{"order_id", "reason"})
	spec.FieldSpec.Set("properties", map[string]interface{}{
		"order_id": map[string]interface{}{"type": "string"},
		"reason":   map[string]interface{}{"type": "string"},
	})
	spec.EvidenceRequirements = []shareddomain.TypedSchema{
		shareddomain.TypedSchemaFromMap(map[string]interface{}{
			"name":        "order_receipt",
			"description": "Proof of purchase or receipt",
		}),
	}
	require.NoError(t, store.FormSpecs().CreateFormSpec(ctx, spec))

	bindingOne := servicedomain.NewServiceCatalogBinding(workspace.ID, childNode.ID, servicedomain.ServiceCatalogBindingTargetKindKnowledgeResource, resource.ID)
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogBinding(ctx, bindingOne))

	bindingTwo := servicedomain.NewServiceCatalogBinding(workspace.ID, childNode.ID, servicedomain.ServiceCatalogBindingTargetKindFormSpec, spec.ID)
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogBinding(ctx, bindingTwo))

	bindings, err := store.ServiceCatalog().ListServiceCatalogBindings(ctx, childNode.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 2)

	filteredResources, total, err := store.KnowledgeResources().ListWorkspaceKnowledgeResources(ctx, workspace.ID, &knowledgedomain.KnowledgeResourceFilter{
		Status: knowledgedomain.KnowledgeResourceStatusActive,
		Search: "refund",
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, filteredResources, 1)
	assert.Equal(t, resource.ID, filteredResources[0].ID)

	submission := servicedomain.NewFormSubmission(workspace.ID, spec.ID)
	submission.Channel = "web_chat"
	submission.SubmitterEmail = "customer@example.com"
	submission.SubmitterName = "Casey Customer"
	submission.CompletionToken = "resume-refund-1"
	submission.CollectedFields.Set("order_id", "ORD-123")
	submission.MissingFields.Set("reason", "required")
	require.NoError(t, store.FormSpecs().CreateFormSubmission(ctx, submission))

	session := servicedomain.NewConversationSession(workspace.ID, servicedomain.ConversationChannelWebChat)
	session.Title = "Refund help"
	session.LanguageCode = "en"
	session.SourceRef = "widget:website"
	session.ExternalSessionKey = "widget-session-123"
	session.PrimaryCatalogNodeID = childNode.ID
	session.ActiveFormSpecID = spec.ID
	session.ActiveFormSubmissionID = submission.ID
	session.Metadata.Set("surface", "widget")
	require.NoError(t, store.Conversations().CreateConversationSession(ctx, session))

	participant := servicedomain.NewConversationParticipant(workspace.ID, session.ID, servicedomain.ConversationParticipantKindAnonymousVisitor, "visitor-1")
	participant.RoleInSession = servicedomain.ConversationParticipantRoleRequester
	participant.DisplayName = "Website Visitor"
	require.NoError(t, store.Conversations().CreateConversationParticipant(ctx, participant))

	message := servicedomain.NewConversationMessage(workspace.ID, session.ID)
	message.ParticipantID = participant.ID
	message.ContentText = "I need a refund for my order."
	message.Content.Set("citations", []string{"refund-policy"})
	require.NoError(t, store.Conversations().CreateConversationMessage(ctx, message))

	workingState := servicedomain.NewConversationWorkingState(workspace.ID, session.ID)
	workingState.PrimaryCatalogNodeID = childNode.ID
	workingState.ActiveFormSpecID = spec.ID
	workingState.ActiveFormSubmissionID = submission.ID
	workingState.SuggestedCatalogNodes = []servicedomain.ConversationCatalogSuggestion{
		{CatalogNodeID: childNode.ID, Reason: "refund intent detected", Confidence: 0.92},
	}
	workingState.CollectedFields.Set("order_id", "ORD-123")
	workingState.MissingFields.Set("reason", "required")
	workingState.RequiresOperatorReview = true
	require.NoError(t, store.Conversations().UpsertConversationWorkingState(ctx, workingState))

	outcome := servicedomain.NewConversationOutcome(workspace.ID, session.ID, servicedomain.ConversationOutcomeKindFormDrafted)
	outcome.ResultRef.Set("form_submission_id", submission.ID)
	require.NoError(t, store.Conversations().CreateConversationOutcome(ctx, outcome))

	reloadedSession, err := store.Conversations().GetConversationSessionByExternalKey(ctx, workspace.ID, servicedomain.ConversationChannelWebChat, "widget-session-123")
	require.NoError(t, err)
	assert.Equal(t, session.ID, reloadedSession.ID)
	assert.Equal(t, childNode.ID, reloadedSession.PrimaryCatalogNodeID)

	participants, err := store.Conversations().ListConversationParticipants(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, participants, 1)

	messages, err := store.Conversations().ListConversationMessages(ctx, session.ID, servicedomain.ConversationMessageVisibilityCustomer)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "I need a refund for my order.", messages[0].ContentText)

	reloadedState, err := store.Conversations().GetConversationWorkingState(ctx, session.ID)
	require.NoError(t, err)
	assert.True(t, reloadedState.RequiresOperatorReview)
	assert.Equal(t, spec.ID, reloadedState.ActiveFormSpecID)
	require.Len(t, reloadedState.SuggestedCatalogNodes, 1)
	assert.Equal(t, childNode.ID, reloadedState.SuggestedCatalogNodes[0].CatalogNodeID)

	outcomes, err := store.Conversations().ListConversationOutcomes(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, servicedomain.ConversationOutcomeKindFormDrafted, outcomes[0].Kind)

	caseObj := testutil.NewIsolatedCase(t, workspace.ID)
	caseObj.PrimaryCatalogNodeID = childNode.ID
	caseObj.OriginatingConversationID = session.ID
	caseObj.QueueID = queue.ID
	caseObj.Priority = servicedomain.CasePriorityHigh
	caseObj.CreatedAt = time.Now().UTC()
	caseObj.UpdatedAt = caseObj.CreatedAt
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	loadedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, childNode.ID, loadedCase.PrimaryCatalogNodeID)
	assert.Equal(t, session.ID, loadedCase.OriginatingConversationID)

	session.LinkedCaseID = caseObj.ID
	session.LastActivityAt = time.Now().UTC()
	session.UpdatedAt = session.LastActivityAt
	require.NoError(t, store.Conversations().UpdateConversationSession(ctx, session))

	sessions, err := store.Conversations().ListWorkspaceConversationSessions(ctx, workspace.ID, servicedomain.ConversationSessionFilter{
		PrimaryCatalogNodeID: childNode.ID,
		Limit:                10,
	})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, caseObj.ID, sessions[0].LinkedCaseID)

	submission.CaseID = caseObj.ID
	submission.Status = servicedomain.FormSubmissionStatusSubmitted
	now := time.Now().UTC()
	submission.SubmittedAt = &now
	submission.UpdatedAt = now
	require.NoError(t, store.FormSpecs().UpdateFormSubmission(ctx, submission))

	updatedSubmission, err := store.FormSpecs().GetFormSubmission(ctx, submission.ID)
	require.NoError(t, err)
	assert.Equal(t, caseObj.ID, updatedSubmission.CaseID)
	assert.Equal(t, servicedomain.FormSubmissionStatusSubmitted, updatedSubmission.Status)
}
