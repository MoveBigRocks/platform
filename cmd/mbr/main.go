package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"

	"gopkg.in/yaml.v3"
)

func registerInstanceURLFlag(fs *flag.FlagSet) *string {
	var value string
	fs.StringVar(&value, "url", "", "Move Big Rocks instance URL, for example https://app.yourdomain.com")
	fs.StringVar(&value, "api-url", "", "Deprecated: use --url")
	return &value
}

func resolveStoredInstanceURL(flagValue string, stored cliapi.StoredConfig) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(cliapi.EnvInstanceURL)); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(cliapi.EnvAPIURL)); value != "" {
		return value
	}
	if value := strings.TrimSpace(stored.InstanceURL); value != "" {
		return value
	}
	return strings.TrimSpace(stored.APIURL)
}

func resolveStoredWorkspaceID(flagValue string, stored cliapi.StoredConfig) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	return strings.TrimSpace(stored.CurrentWorkspaceID)
}

func resolveStoredTeamID(flagValue string, stored cliapi.StoredConfig) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	return strings.TrimSpace(stored.CurrentTeamID)
}

func requireWorkspaceID(flagValue string, stored cliapi.StoredConfig) (string, error) {
	workspaceID := resolveStoredWorkspaceID(flagValue, stored)
	if workspaceID == "" {
		return "", fmt.Errorf("--workspace is required")
	}
	return workspaceID, nil
}

func requireTeamID(flagValue string, stored cliapi.StoredConfig) (string, error) {
	teamID := resolveStoredTeamID(flagValue, stored)
	if teamID == "" {
		return "", fmt.Errorf("--team is required")
	}
	return teamID, nil
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

const conversationParticipantSelection = `
id
workspaceID
conversationSessionID
participantKind
participantRef
roleInSession
displayName
joinedAt
leftAt
metadata
createdAt
`

const conversationMessageSelection = `
id
workspaceID
conversationSessionID
participantID
role
kind
visibility
contentText
content
createdAt
`

const conversationWorkingStateSelection = `
conversationSessionID
workspaceID
primaryCatalogNodeID
suggestedCatalogNodes {
  catalogNodeID
  reason
  confidence
}
classificationConfidence
activePolicyProfileRef
activeFormSpecID
activeFormSubmissionID
collectedFields
missingFields
requiresOperatorReview
updatedAt
`

const conversationOutcomeSelection = `
id
workspaceID
conversationSessionID
kind
resultRef
createdAt
`

const conversationListSelection = `
id
workspaceID
channel
status
title
primaryContactID
primaryCatalogNodeID
linkedCaseID
handlingTeamID
openedAt
lastActivityAt
updatedAt
`

const conversationDetailSelection = `
` + conversationListSelection + `
languageCode
sourceRef
externalSessionKey
activeFormSpecID
activeFormSubmissionID
assignedOperatorUserID
delegatedRuntimeConnectorID
closedAt
metadata
createdAt
participants {
  ` + conversationParticipantSelection + `
}
messages {
  ` + conversationMessageSelection + `
}
workingState {
  ` + conversationWorkingStateSelection + `
}
outcomes {
  ` + conversationOutcomeSelection + `
}
`

const knowledgeSelection = `
id
workspaceID
ownerTeamID
slug
title
kind
conceptSpecKey
conceptSpecVersion
sourceKind
sourceRef
pathRef
artifactPath
summary
bodyMarkdown
frontmatter
supportedChannels
sharedWithTeamIDs
surface
trustLevel
searchKeywords
status
reviewStatus
contentHash
revisionRef
publishedRevision
reviewedAt
publishedAt
publishedByID
createdByID
createdAt
updatedAt
`

const conceptSpecSelection = `
id
workspaceID
ownerTeamID
key
version
name
description
extendsKey
extendsVersion
instanceKind
metadataSchema
sectionsSchema
workflowSchema
agentGuidanceMarkdown
artifactPath
revisionRef
sourceKind
sourceRef
status
createdByID
createdAt
updatedAt
`

const serviceCatalogBindingSelection = `
id
workspaceID
catalogNodeID
targetKind
targetID
bindingKind
confidence
createdAt
`

const serviceCatalogNodeListSelection = `
id
workspaceID
parentNodeID
slug
pathSlug
title
nodeKind
status
visibility
defaultQueueID
displayOrder
createdAt
updatedAt
`

const serviceCatalogNodeDetailSelection = `
` + serviceCatalogNodeListSelection + `
descriptionMarkdown
defaultCaseCategory
defaultPriority
searchKeywords
supportedChannels
bindings {
  ` + serviceCatalogBindingSelection + `
}
`

const formSelection = `
id
workspaceID
workspaceName
name
slug
description
status
cryptoID
isPublic
requiresCaptcha
collectEmail
autoCreateCase
autoCasePriority
autoCaseType
autoAssignTeamID
autoTags
notifyOnSubmission
notificationEmails
submissionMessage
redirectURL
schemaData
submissionCount
createdAt
updatedAt
createdByID
`

const ruleSelection = `
id
workspaceID
workspaceName
title
description
isActive
priority
maxExecutionsPerHour
maxExecutionsPerDay
conditions
actions
executionCount
lastExecutedAt
createdAt
updatedAt
createdByID
`

type formOutput struct {
	ID                 string         `json:"id"`
	WorkspaceID        string         `json:"workspaceID"`
	WorkspaceName      *string        `json:"workspaceName"`
	Name               string         `json:"name"`
	Slug               string         `json:"slug"`
	Description        *string        `json:"description"`
	Status             string         `json:"status"`
	CryptoID           string         `json:"cryptoID"`
	IsPublic           bool           `json:"isPublic"`
	RequiresCaptcha    bool           `json:"requiresCaptcha"`
	CollectEmail       bool           `json:"collectEmail"`
	AutoCreateCase     bool           `json:"autoCreateCase"`
	AutoCasePriority   *string        `json:"autoCasePriority"`
	AutoCaseType       *string        `json:"autoCaseType"`
	AutoAssignTeamID   *string        `json:"autoAssignTeamID"`
	AutoTags           []string       `json:"autoTags"`
	NotifyOnSubmission bool           `json:"notifyOnSubmission"`
	NotificationEmails []string       `json:"notificationEmails"`
	SubmissionMessage  *string        `json:"submissionMessage"`
	RedirectURL        *string        `json:"redirectURL"`
	SchemaData         map[string]any `json:"schemaData"`
	SubmissionCount    int            `json:"submissionCount"`
	CreatedAt          string         `json:"createdAt"`
	UpdatedAt          string         `json:"updatedAt"`
	CreatedByID        string         `json:"createdByID"`
}

type ruleOutput struct {
	ID                   string  `json:"id"`
	WorkspaceID          string  `json:"workspaceID"`
	WorkspaceName        *string `json:"workspaceName"`
	Title                string  `json:"title"`
	Description          *string `json:"description"`
	IsActive             bool    `json:"isActive"`
	Priority             int     `json:"priority"`
	MaxExecutionsPerHour int     `json:"maxExecutionsPerHour"`
	MaxExecutionsPerDay  int     `json:"maxExecutionsPerDay"`
	Conditions           []any   `json:"conditions"`
	Actions              []any   `json:"actions"`
	ExecutionCount       int     `json:"executionCount"`
	LastExecutedAt       *string `json:"lastExecutedAt"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
	CreatedByID          string  `json:"createdByID"`
}

type attachmentOutput struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceID"`
	CaseID      string `json:"caseID"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type workspaceOutput struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ShortCode   string  `json:"shortCode"`
	Description *string `json:"description,omitempty"`
}

type teamOutput struct {
	ID                  string   `json:"id"`
	WorkspaceID         string   `json:"workspaceID"`
	Name                string   `json:"name"`
	Description         *string  `json:"description,omitempty"`
	EmailAddress        *string  `json:"emailAddress,omitempty"`
	ResponseTimeHours   int      `json:"responseTimeHours"`
	ResolutionTimeHours int      `json:"resolutionTimeHours"`
	AutoAssign          bool     `json:"autoAssign"`
	AutoAssignKeywords  []string `json:"autoAssignKeywords"`
	IsActive            bool     `json:"isActive"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type teamMemberOutput struct {
	ID          string `json:"id"`
	TeamID      string `json:"teamID"`
	UserID      string `json:"userID"`
	WorkspaceID string `json:"workspaceID"`
	Role        string `json:"role"`
	IsActive    bool   `json:"isActive"`
	JoinedAt    string `json:"joinedAt"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type queueOutput struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspaceID"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type conversationCatalogSuggestionOutput struct {
	CatalogNodeID string  `json:"catalogNodeID"`
	Reason        *string `json:"reason,omitempty"`
	Confidence    float64 `json:"confidence"`
}

type conversationParticipantOutput struct {
	ID                    string         `json:"id"`
	WorkspaceID           string         `json:"workspaceID"`
	ConversationSessionID string         `json:"conversationSessionID"`
	ParticipantKind       string         `json:"participantKind"`
	ParticipantRef        string         `json:"participantRef"`
	RoleInSession         string         `json:"roleInSession"`
	DisplayName           *string        `json:"displayName,omitempty"`
	JoinedAt              string         `json:"joinedAt"`
	LeftAt                *string        `json:"leftAt,omitempty"`
	Metadata              map[string]any `json:"metadata"`
	CreatedAt             string         `json:"createdAt"`
}

type conversationMessageOutput struct {
	ID                    string         `json:"id"`
	WorkspaceID           string         `json:"workspaceID"`
	ConversationSessionID string         `json:"conversationSessionID"`
	ParticipantID         *string        `json:"participantID,omitempty"`
	Role                  string         `json:"role"`
	Kind                  string         `json:"kind"`
	Visibility            string         `json:"visibility"`
	ContentText           *string        `json:"contentText,omitempty"`
	Content               map[string]any `json:"content"`
	CreatedAt             string         `json:"createdAt"`
}

type conversationWorkingStateOutput struct {
	ConversationSessionID    string                                `json:"conversationSessionID"`
	WorkspaceID              string                                `json:"workspaceID"`
	PrimaryCatalogNodeID     *string                               `json:"primaryCatalogNodeID,omitempty"`
	SuggestedCatalogNodes    []conversationCatalogSuggestionOutput `json:"suggestedCatalogNodes"`
	ClassificationConfidence *float64                              `json:"classificationConfidence,omitempty"`
	ActivePolicyProfileRef   *string                               `json:"activePolicyProfileRef,omitempty"`
	ActiveFormSpecID         *string                               `json:"activeFormSpecID,omitempty"`
	ActiveFormSubmissionID   *string                               `json:"activeFormSubmissionID,omitempty"`
	CollectedFields          map[string]any                        `json:"collectedFields"`
	MissingFields            map[string]any                        `json:"missingFields"`
	RequiresOperatorReview   bool                                  `json:"requiresOperatorReview"`
	UpdatedAt                string                                `json:"updatedAt"`
}

type conversationOutcomeOutput struct {
	ID                    string         `json:"id"`
	WorkspaceID           string         `json:"workspaceID"`
	ConversationSessionID string         `json:"conversationSessionID"`
	Kind                  string         `json:"kind"`
	ResultRef             map[string]any `json:"resultRef"`
	CreatedAt             string         `json:"createdAt"`
}

type conversationSessionOutput struct {
	ID                          string                          `json:"id"`
	WorkspaceID                 string                          `json:"workspaceID"`
	Channel                     string                          `json:"channel"`
	Status                      string                          `json:"status"`
	Title                       *string                         `json:"title,omitempty"`
	LanguageCode                *string                         `json:"languageCode,omitempty"`
	SourceRef                   *string                         `json:"sourceRef,omitempty"`
	ExternalSessionKey          *string                         `json:"externalSessionKey,omitempty"`
	PrimaryContactID            *string                         `json:"primaryContactID,omitempty"`
	PrimaryCatalogNodeID        *string                         `json:"primaryCatalogNodeID,omitempty"`
	ActiveFormSpecID            *string                         `json:"activeFormSpecID,omitempty"`
	ActiveFormSubmissionID      *string                         `json:"activeFormSubmissionID,omitempty"`
	LinkedCaseID                *string                         `json:"linkedCaseID,omitempty"`
	HandlingTeamID              *string                         `json:"handlingTeamID,omitempty"`
	AssignedOperatorUserID      *string                         `json:"assignedOperatorUserID,omitempty"`
	DelegatedRuntimeConnectorID *string                         `json:"delegatedRuntimeConnectorID,omitempty"`
	OpenedAt                    string                          `json:"openedAt"`
	LastActivityAt              string                          `json:"lastActivityAt"`
	ClosedAt                    *string                         `json:"closedAt,omitempty"`
	Metadata                    map[string]any                  `json:"metadata"`
	CreatedAt                   string                          `json:"createdAt"`
	UpdatedAt                   string                          `json:"updatedAt"`
	Participants                []conversationParticipantOutput `json:"participants,omitempty"`
	Messages                    []conversationMessageOutput     `json:"messages,omitempty"`
	WorkingState                *conversationWorkingStateOutput `json:"workingState,omitempty"`
	Outcomes                    []conversationOutcomeOutput     `json:"outcomes,omitempty"`
}

type serviceCatalogBindingOutput struct {
	ID            string   `json:"id"`
	WorkspaceID   string   `json:"workspaceID"`
	CatalogNodeID string   `json:"catalogNodeID"`
	TargetKind    string   `json:"targetKind"`
	TargetID      string   `json:"targetID"`
	BindingKind   string   `json:"bindingKind"`
	Confidence    *float64 `json:"confidence,omitempty"`
	CreatedAt     string   `json:"createdAt"`
}

type serviceCatalogNodeOutput struct {
	ID                  string                        `json:"id"`
	WorkspaceID         string                        `json:"workspaceID"`
	ParentNodeID        *string                       `json:"parentNodeID,omitempty"`
	Slug                string                        `json:"slug"`
	PathSlug            string                        `json:"pathSlug"`
	Title               string                        `json:"title"`
	DescriptionMarkdown *string                       `json:"descriptionMarkdown,omitempty"`
	NodeKind            string                        `json:"nodeKind"`
	Status              string                        `json:"status"`
	Visibility          string                        `json:"visibility"`
	SupportedChannels   []string                      `json:"supportedChannels,omitempty"`
	DefaultCaseCategory *string                       `json:"defaultCaseCategory,omitempty"`
	DefaultQueueID      *string                       `json:"defaultQueueID,omitempty"`
	DefaultPriority     *string                       `json:"defaultPriority,omitempty"`
	SearchKeywords      []string                      `json:"searchKeywords,omitempty"`
	DisplayOrder        int                           `json:"displayOrder"`
	CreatedAt           string                        `json:"createdAt"`
	UpdatedAt           string                        `json:"updatedAt"`
	Bindings            []serviceCatalogBindingOutput `json:"bindings,omitempty"`
}

type knowledgeResourceOutput struct {
	ID                 string         `json:"id"`
	WorkspaceID        string         `json:"workspaceID"`
	OwnerTeamID        string         `json:"ownerTeamID"`
	Slug               string         `json:"slug"`
	Title              string         `json:"title"`
	Kind               string         `json:"kind"`
	ConceptSpecKey     string         `json:"conceptSpecKey"`
	ConceptSpecVersion string         `json:"conceptSpecVersion"`
	SourceKind         string         `json:"sourceKind"`
	SourceRef          *string        `json:"sourceRef,omitempty"`
	PathRef            *string        `json:"pathRef,omitempty"`
	ArtifactPath       string         `json:"artifactPath"`
	Summary            *string        `json:"summary,omitempty"`
	BodyMarkdown       string         `json:"bodyMarkdown"`
	Frontmatter        map[string]any `json:"frontmatter"`
	SupportedChannels  []string       `json:"supportedChannels"`
	SharedWithTeamIDs  []string       `json:"sharedWithTeamIDs"`
	Surface            string         `json:"surface"`
	TrustLevel         string         `json:"trustLevel"`
	SearchKeywords     []string       `json:"searchKeywords"`
	Status             string         `json:"status"`
	ReviewStatus       string         `json:"reviewStatus"`
	ContentHash        string         `json:"contentHash"`
	RevisionRef        string         `json:"revisionRef"`
	PublishedRevision  *string        `json:"publishedRevision,omitempty"`
	ReviewedAt         *string        `json:"reviewedAt,omitempty"`
	PublishedAt        *string        `json:"publishedAt,omitempty"`
	PublishedByID      *string        `json:"publishedByID,omitempty"`
	CreatedByID        *string        `json:"createdByID,omitempty"`
	CreatedAt          string         `json:"createdAt"`
	UpdatedAt          string         `json:"updatedAt"`
}

type conceptSpecOutput struct {
	ID                    string         `json:"id"`
	WorkspaceID           *string        `json:"workspaceID,omitempty"`
	OwnerTeamID           *string        `json:"ownerTeamID,omitempty"`
	Key                   string         `json:"key"`
	Version               string         `json:"version"`
	Name                  string         `json:"name"`
	Description           string         `json:"description"`
	ExtendsKey            *string        `json:"extendsKey,omitempty"`
	ExtendsVersion        *string        `json:"extendsVersion,omitempty"`
	InstanceKind          string         `json:"instanceKind"`
	MetadataSchema        map[string]any `json:"metadataSchema"`
	SectionsSchema        map[string]any `json:"sectionsSchema"`
	WorkflowSchema        map[string]any `json:"workflowSchema"`
	AgentGuidanceMarkdown string         `json:"agentGuidanceMarkdown"`
	ArtifactPath          string         `json:"artifactPath"`
	RevisionRef           *string        `json:"revisionRef,omitempty"`
	SourceKind            string         `json:"sourceKind"`
	SourceRef             *string        `json:"sourceRef,omitempty"`
	Status                string         `json:"status"`
	CreatedByID           *string        `json:"createdByID,omitempty"`
	CreatedAt             string         `json:"createdAt"`
	UpdatedAt             string         `json:"updatedAt"`
}

type knowledgeRevisionOutput struct {
	Ref         string `json:"ref"`
	CommittedAt string `json:"committedAt"`
	Subject     string `json:"subject"`
}

type knowledgeDiffOutput struct {
	Path         string  `json:"path"`
	FromRevision *string `json:"fromRevision,omitempty"`
	ToRevision   string  `json:"toRevision"`
	Patch        string  `json:"patch"`
}

type artifactRevisionOutput struct {
	Ref         string `json:"ref"`
	CommittedAt string `json:"committedAt"`
	Subject     string `json:"subject"`
}

type artifactDiffOutput struct {
	FromRevision *string `json:"fromRevision,omitempty"`
	ToRevision   string  `json:"toRevision"`
	Patch        string  `json:"patch"`
}

type extensionArtifactSurfaceOutput struct {
	Name          string  `json:"name"`
	Description   *string `json:"description,omitempty"`
	SeedAssetPath *string `json:"seedAssetPath,omitempty"`
}

type extensionArtifactFileOutput struct {
	Surface string `json:"surface"`
	Path    string `json:"path"`
}

type extensionArtifactPublicationOutput struct {
	Surface     string `json:"surface"`
	Path        string `json:"path"`
	RevisionRef string `json:"revisionRef"`
}

type knowledgeMutationInput struct {
	WorkspaceID        string
	TeamID             string
	Slug               string
	Title              string
	Kind               string
	ConceptSpecKey     string
	ConceptSpecVersion string
	Status             string
	Summary            string
	BodyMarkdown       *string
	SourceKind         string
	SourceRef          string
	PathRef            string
	SupportedChannels  []string
	SharedWithTeamIDs  []string
	SearchKeywords     []string
	Surface            string
	Frontmatter        map[string]any
}

type knowledgeSyncDefaults struct {
	WorkspaceID        string
	TeamID             string
	Surface            string
	Kind               string
	ConceptSpecKey     string
	ConceptSpecVersion string
	Status             string
	ReviewStatus       string
	SharedWithTeamIDs  []string
	SourceKind         string
	SourceRef          string
}

type knowledgeSyncDocument struct {
	AbsolutePath       string
	RelativePath       string
	Slug               string
	Title              string
	TeamID             string
	Surface            string
	Kind               string
	ConceptSpecKey     string
	ConceptSpecVersion string
	Status             string
	ReviewStatus       string
	Summary            string
	SharedWithTeamIDs  []string
	SupportedChannels  []string
	SearchKeywords     []string
	SourceKind         string
	SourceRef          string
	PathRef            string
	BodyMarkdown       string
	Frontmatter        map[string]any
}

type knowledgeSyncResult struct {
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
	Action       string `json:"action"`
	ID           string `json:"id"`
	TeamID       string `json:"teamID"`
	Surface      string `json:"surface"`
	Slug         string `json:"slug"`
	RevisionRef  string `json:"revisionRef"`
}

type knowledgeImportPlan struct {
	Path               string   `json:"path"`
	RelativePath       string   `json:"relativePath"`
	WorkspaceID        string   `json:"workspaceID,omitempty"`
	TeamID             string   `json:"teamID"`
	Surface            string   `json:"surface"`
	Kind               string   `json:"kind"`
	ConceptSpecKey     string   `json:"conceptSpecKey"`
	ConceptSpecVersion string   `json:"conceptSpecVersion"`
	Status             string   `json:"status,omitempty"`
	ReviewStatus       string   `json:"reviewStatus,omitempty"`
	Slug               string   `json:"slug"`
	Title              string   `json:"title"`
	Summary            string   `json:"summary,omitempty"`
	SourceKind         string   `json:"sourceKind"`
	SourceRef          string   `json:"sourceRef"`
	PathRef            string   `json:"pathRef"`
	SharedWithTeamIDs  []string `json:"sharedWithTeamIDs,omitempty"`
	SupportedChannels  []string `json:"supportedChannels,omitempty"`
	SearchKeywords     []string `json:"searchKeywords,omitempty"`
}

type conceptSpecMutationInput struct {
	WorkspaceID           string
	OwnerTeamID           string
	Key                   string
	Version               string
	Name                  string
	Description           string
	ExtendsKey            string
	ExtendsVersion        string
	InstanceKind          string
	MetadataSchema        map[string]any
	SectionsSchema        map[string]any
	WorkflowSchema        map[string]any
	AgentGuidanceMarkdown string
	SourceKind            string
	SourceRef             string
	Status                string
}

var knowledgeKindPathHints = []struct {
	match string
	kind  knowledgedomain.KnowledgeResourceKind
}{
	{match: "/policies/", kind: knowledgedomain.KnowledgeResourceKindPolicy},
	{match: "/guides/", kind: knowledgedomain.KnowledgeResourceKindGuide},
	{match: "/playbooks/", kind: knowledgedomain.KnowledgeResourceKindGuide},
	{match: "/runbooks/", kind: knowledgedomain.KnowledgeResourceKindGuide},
	{match: "/how-to/", kind: knowledgedomain.KnowledgeResourceKindGuide},
	{match: "/howto/", kind: knowledgedomain.KnowledgeResourceKindGuide},
	{match: "/procedures/", kind: knowledgedomain.KnowledgeResourceKindGuide},
	{match: "/skills/", kind: knowledgedomain.KnowledgeResourceKindSkill},
	{match: "/prompts/", kind: knowledgedomain.KnowledgeResourceKindSkill},
	{match: "/context/", kind: knowledgedomain.KnowledgeResourceKindContext},
	{match: "/contexts/", kind: knowledgedomain.KnowledgeResourceKindContext},
	{match: "/research/", kind: knowledgedomain.KnowledgeResourceKindContext},
	{match: "/briefs/", kind: knowledgedomain.KnowledgeResourceKindContext},
	{match: "/constraints/", kind: knowledgedomain.KnowledgeResourceKindConstraint},
	{match: "/guardrails/", kind: knowledgedomain.KnowledgeResourceKindConstraint},
	{match: "/requirements/", kind: knowledgedomain.KnowledgeResourceKindConstraint},
	{match: "/best-practices/", kind: knowledgedomain.KnowledgeResourceKindBestPractice},
	{match: "/best_practices/", kind: knowledgedomain.KnowledgeResourceKindBestPractice},
	{match: "/standards/", kind: knowledgedomain.KnowledgeResourceKindBestPractice},
	{match: "/conventions/", kind: knowledgedomain.KnowledgeResourceKindBestPractice},
	{match: "/templates/", kind: knowledgedomain.KnowledgeResourceKindTemplate},
	{match: "/snippets/", kind: knowledgedomain.KnowledgeResourceKindTemplate},
	{match: "/checklists/", kind: knowledgedomain.KnowledgeResourceKindChecklist},
	{match: "/decisions/", kind: knowledgedomain.KnowledgeResourceKindDecision},
	{match: "/rfcs/", kind: knowledgedomain.KnowledgeResourceKindDecision},
	{match: "/adr/", kind: knowledgedomain.KnowledgeResourceKindDecision},
	{match: "/adrs/", kind: knowledgedomain.KnowledgeResourceKindDecision},
	{match: "/ideas/", kind: knowledgedomain.KnowledgeResourceKindIdea},
	{match: "/brainstorm/", kind: knowledgedomain.KnowledgeResourceKindIdea},
	{match: "/proposals/", kind: knowledgedomain.KnowledgeResourceKindIdea},
}

type conversationListInput struct {
	WorkspaceID          string
	Status               string
	Channel              string
	PrimaryCatalogNodeID string
	PrimaryContactID     string
	LinkedCaseID         string
	Limit                int
	Offset               int
}

type conversationReplyInput struct {
	SessionID     string
	ParticipantID string
	Role          string
	Kind          string
	Visibility    string
	ContentText   string
}

type userOutput struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type healthCheckResult struct {
	HealthURL     string  `json:"healthURL"`
	HTTPStatus    int     `json:"httpStatus"`
	Status        string  `json:"status"`
	Service       string  `json:"service"`
	Version       *string `json:"version,omitempty"`
	GitCommit     *string `json:"gitCommit,omitempty"`
	BuildDate     *string `json:"buildDate,omitempty"`
	InstanceID    *string `json:"instanceID,omitempty"`
	AuthOK        bool    `json:"authOK"`
	AuthMessage   *string `json:"authMessage,omitempty"`
	PrincipalType string  `json:"principalType,omitempty"`
	PrincipalID   string  `json:"principalID,omitempty"`
}

type healthAuthResult struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type browserLoginStartResponse struct {
	RequestID        string `json:"requestID"`
	PollToken        string `json:"pollToken"`
	AuthorizeURL     string `json:"authorizeURL"`
	AdminBaseURL     string `json:"adminBaseURL"`
	AdminGraphQLURL  string `json:"adminGraphQLURL"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
	IntervalSeconds  int    `json:"intervalSeconds"`
}

type browserLoginPollResponse struct {
	Status       string `json:"status"`
	UserID       string `json:"userID"`
	SessionToken string `json:"sessionToken"`
}

type extensionOutput struct {
	ID                string  `json:"id"`
	WorkspaceID       *string `json:"workspaceID"`
	Slug              string  `json:"slug"`
	Name              string  `json:"name"`
	Publisher         string  `json:"publisher"`
	Version           string  `json:"version"`
	Kind              string  `json:"kind"`
	Scope             string  `json:"scope"`
	Risk              string  `json:"risk"`
	Status            string  `json:"status"`
	ValidationStatus  string  `json:"validationStatus"`
	ValidationMessage *string `json:"validationMessage"`
	HealthStatus      string  `json:"healthStatus"`
	HealthMessage     *string `json:"healthMessage"`
}

type extensionWorkspacePlanOutput struct {
	Mode        *string `json:"mode"`
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
}

type extensionSchemaOutput struct {
	Name            string `json:"name"`
	PackageKey      string `json:"packageKey"`
	TargetVersion   string `json:"targetVersion"`
	MigrationEngine string `json:"migrationEngine"`
}

type extensionRouteOutput struct {
	PathPrefix      string  `json:"pathPrefix"`
	AssetPath       *string `json:"assetPath,omitempty"`
	ArtifactSurface *string `json:"artifactSurface,omitempty"`
	ArtifactPath    *string `json:"artifactPath,omitempty"`
}

type extensionEndpointOutput struct {
	Name               string   `json:"name"`
	Class              string   `json:"class"`
	MountPath          string   `json:"mountPath"`
	Methods            []string `json:"methods"`
	Auth               string   `json:"auth"`
	ContentTypes       []string `json:"contentTypes"`
	MaxBodyBytes       int      `json:"maxBodyBytes"`
	RateLimitPerMinute int      `json:"rateLimitPerMinute"`
	WorkspaceBinding   string   `json:"workspaceBinding"`
	AssetPath          *string  `json:"assetPath"`
	ArtifactSurface    *string  `json:"artifactSurface,omitempty"`
	ArtifactPath       *string  `json:"artifactPath,omitempty"`
	ServiceTarget      *string  `json:"serviceTarget"`
}

type extensionAdminNavigationItemOutput struct {
	Name       string  `json:"name"`
	Section    *string `json:"section"`
	Title      string  `json:"title"`
	Icon       *string `json:"icon"`
	Endpoint   string  `json:"endpoint"`
	ActivePage *string `json:"activePage"`
}

type extensionDashboardWidgetOutput struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Endpoint    string  `json:"endpoint"`
}

type resolvedExtensionAdminNavigationItemOutput struct {
	ExtensionID   string  `json:"extensionID"`
	ExtensionSlug string  `json:"extensionSlug"`
	WorkspaceID   *string `json:"workspaceID,omitempty"`
	Section       *string `json:"section"`
	Title         string  `json:"title"`
	Icon          *string `json:"icon"`
	Href          string  `json:"href"`
	ActivePage    *string `json:"activePage"`
}

type resolvedExtensionDashboardWidgetOutput struct {
	ExtensionID   string  `json:"extensionID"`
	ExtensionSlug string  `json:"extensionSlug"`
	WorkspaceID   *string `json:"workspaceID,omitempty"`
	Title         string  `json:"title"`
	Description   *string `json:"description"`
	Icon          *string `json:"icon"`
	Href          string  `json:"href"`
}

type extensionSeededResourcesOutput struct {
	Queues          []extensionSeededQueueStateOutput          `json:"queues"`
	Forms           []extensionSeededFormStateOutput           `json:"forms"`
	AutomationRules []extensionSeededAutomationRuleStateOutput `json:"automationRules"`
}

type extensionSeededQueueStateOutput struct {
	Slug        string         `json:"slug"`
	ResourceID  *string        `json:"resourceID"`
	Exists      bool           `json:"exists"`
	MatchesSeed bool           `json:"matchesSeed"`
	Problems    []string       `json:"problems"`
	Expected    map[string]any `json:"expected"`
	Actual      map[string]any `json:"actual,omitempty"`
}

type extensionSeededFormStateOutput struct {
	Slug        string         `json:"slug"`
	ResourceID  *string        `json:"resourceID"`
	Exists      bool           `json:"exists"`
	MatchesSeed bool           `json:"matchesSeed"`
	Problems    []string       `json:"problems"`
	Expected    map[string]any `json:"expected"`
	Actual      map[string]any `json:"actual,omitempty"`
}

type extensionSeededAutomationRuleStateOutput struct {
	Key         string         `json:"key"`
	ResourceID  *string        `json:"resourceID"`
	Exists      bool           `json:"exists"`
	MatchesSeed bool           `json:"matchesSeed"`
	Problems    []string       `json:"problems"`
	Expected    map[string]any `json:"expected"`
	Actual      map[string]any `json:"actual,omitempty"`
}

type extensionEventConsumerOutput struct {
	Name          string   `json:"name"`
	Description   *string  `json:"description"`
	Stream        string   `json:"stream"`
	EventTypes    []string `json:"eventTypes"`
	ConsumerGroup *string  `json:"consumerGroup"`
	ServiceTarget string   `json:"serviceTarget"`
}

type extensionScheduledJobOutput struct {
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	IntervalSeconds int     `json:"intervalSeconds"`
	ServiceTarget   string  `json:"serviceTarget"`
}

type extensionCommandOutput struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type extensionAgentSkillOutput struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	AssetPath   string  `json:"assetPath"`
}

type extensionSkillDetailOutput struct {
	ExtensionID   string  `json:"extensionID"`
	ExtensionSlug string  `json:"extensionSlug"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	AssetPath     string  `json:"assetPath"`
	ContentType   string  `json:"contentType"`
	Content       string  `json:"content"`
}

type extensionEventDefinitionOutput struct {
	Type          string  `json:"type"`
	Description   *string `json:"description"`
	SchemaVersion int     `json:"schemaVersion"`
}

type extensionEventCatalogOutput struct {
	Publishes  []extensionEventDefinitionOutput `json:"publishes"`
	Subscribes []string                         `json:"subscribes"`
}

type extensionRuntimeEventOutput struct {
	Type          string   `json:"type"`
	Description   *string  `json:"description"`
	SchemaVersion int      `json:"schemaVersion"`
	Core          bool     `json:"core"`
	Publishers    []string `json:"publishers"`
	Subscribers   []string `json:"subscribers"`
}

type extensionAssetOutput struct {
	ID             string `json:"id"`
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	ContentType    string `json:"contentType"`
	IsCustomizable bool   `json:"isCustomizable"`
	Checksum       string `json:"checksum"`
	Size           int    `json:"size"`
	UpdatedAt      string `json:"updatedAt"`
}

type extensionRuntimeConsumerStateOutput struct {
	Name                string  `json:"name"`
	Stream              string  `json:"stream"`
	ConsumerGroup       *string `json:"consumerGroup"`
	ServiceTarget       string  `json:"serviceTarget"`
	Status              string  `json:"status"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	RegisteredAt        *string `json:"registeredAt"`
	LastDeliveredAt     *string `json:"lastDeliveredAt"`
	LastSuccessAt       *string `json:"lastSuccessAt"`
	LastFailureAt       *string `json:"lastFailureAt"`
	LastError           *string `json:"lastError"`
}

type extensionRuntimeJobStateOutput struct {
	Name                string  `json:"name"`
	IntervalSeconds     int     `json:"intervalSeconds"`
	ServiceTarget       string  `json:"serviceTarget"`
	Status              string  `json:"status"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	RegisteredAt        *string `json:"registeredAt"`
	LastStartedAt       *string `json:"lastStartedAt"`
	LastSuccessAt       *string `json:"lastSuccessAt"`
	LastFailureAt       *string `json:"lastFailureAt"`
	BackoffUntil        *string `json:"backoffUntil"`
	LastError           *string `json:"lastError"`
}

type extensionRuntimeDiagnosticsOutput struct {
	BootstrapStatus    string                                `json:"bootstrapStatus"`
	LastBootstrapAt    *string                               `json:"lastBootstrapAt"`
	LastBootstrapError *string                               `json:"lastBootstrapError"`
	Endpoints          []extensionRuntimeEndpointStateOutput `json:"endpoints"`
	EventConsumers     []extensionRuntimeConsumerStateOutput `json:"eventConsumers"`
	ScheduledJobs      []extensionRuntimeJobStateOutput      `json:"scheduledJobs"`
}

type extensionRuntimeEndpointStateOutput struct {
	Name                string  `json:"name"`
	Class               string  `json:"class"`
	MountPath           string  `json:"mountPath"`
	ServiceTarget       *string `json:"serviceTarget"`
	Status              string  `json:"status"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	RegisteredAt        *string `json:"registeredAt"`
	LastCheckedAt       *string `json:"lastCheckedAt"`
	LastSuccessAt       *string `json:"lastSuccessAt"`
	LastFailureAt       *string `json:"lastFailureAt"`
	LastError           *string `json:"lastError"`
}

type extensionDetailOutput struct {
	extensionOutput
	Description              *string                                      `json:"description"`
	RuntimeClass             string                                       `json:"runtimeClass"`
	StorageClass             string                                       `json:"storageClass"`
	Schema                   *extensionSchemaOutput                       `json:"schema"`
	WorkspacePlan            *extensionWorkspacePlanOutput                `json:"workspacePlan"`
	Permissions              []string                                     `json:"permissions"`
	ArtifactSurfaces         []extensionArtifactSurfaceOutput             `json:"artifactSurfaces"`
	PublicRoutes             []extensionRouteOutput                       `json:"publicRoutes"`
	AdminRoutes              []extensionRouteOutput                       `json:"adminRoutes"`
	Endpoints                []extensionEndpointOutput                    `json:"endpoints"`
	AdminNavigation          []extensionAdminNavigationItemOutput         `json:"adminNavigation"`
	DashboardWidgets         []extensionDashboardWidgetOutput             `json:"dashboardWidgets"`
	ResolvedAdminNavigation  []resolvedExtensionAdminNavigationItemOutput `json:"resolvedAdminNavigation"`
	ResolvedDashboardWidgets []resolvedExtensionDashboardWidgetOutput     `json:"resolvedDashboardWidgets"`
	SeededResources          extensionSeededResourcesOutput               `json:"seededResources"`
	Events                   extensionEventCatalogOutput                  `json:"events"`
	EventConsumers           []extensionEventConsumerOutput               `json:"eventConsumers"`
	ScheduledJobs            []extensionScheduledJobOutput                `json:"scheduledJobs"`
	Commands                 []extensionCommandOutput                     `json:"commands"`
	AgentSkills              []extensionAgentSkillOutput                  `json:"agentSkills"`
	Customizable             []string                                     `json:"customizableAssets"`
	BundleSHA256             string                                       `json:"bundleSHA256"`
	BundleSize               int                                          `json:"bundleSize"`
	InstalledByID            *string                                      `json:"installedByID"`
	InstalledAt              string                                       `json:"installedAt"`
	ActivatedAt              *string                                      `json:"activatedAt"`
	DeactivatedAt            *string                                      `json:"deactivatedAt"`
	ValidatedAt              *string                                      `json:"validatedAt"`
	LastHealthCheckAt        *string                                      `json:"lastHealthCheckAt"`
	RuntimeDiagnostics       extensionRuntimeDiagnosticsOutput            `json:"runtimeDiagnostics"`
	Assets                   []extensionAssetOutput                       `json:"assets"`
}

type bundleFile struct {
	Manifest   map[string]any         `json:"manifest"`
	Assets     []bundleAssetInput     `json:"assets"`
	Migrations []bundleMigrationInput `json:"migrations,omitempty"`
}

type bundleAssetInput struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	ContentType    string `json:"contentType,omitempty"`
	IsCustomizable bool   `json:"isCustomizable,omitempty"`
}

type bundleMigrationInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type workspaceCreateInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
}

func resolveInstallWorkspace(ctx context.Context, cfg cliapi.Config, requestedWorkspaceID string, source bundleSourcePayload) (string, *workspaceOutput, error) {
	if requestedWorkspaceID != "" {
		return requestedWorkspaceID, nil, nil
	}

	manifest, err := decodeBundleManifest(source.Bundle.Manifest)
	if err != nil {
		return "", nil, fmt.Errorf("decode extension manifest: %w", err)
	}
	if manifest.Scope == platformdomain.ExtensionScopeInstance {
		return "", nil, nil
	}
	if manifest.WorkspacePlan.Mode != platformdomain.ExtensionWorkspaceProvisionDedicated {
		return "", nil, fmt.Errorf("--workspace is required unless the extension manifest provisions a dedicated workspace")
	}
	if cfg.AuthMode != cliapi.AuthModeSession {
		return "", nil, fmt.Errorf("provisioning a dedicated workspace requires browser login or session-backed auth")
	}

	workspace, err := createWorkspace(ctx, cfg, workspaceCreateInput{
		Name:        manifest.WorkspacePlan.Name,
		Slug:        manifest.WorkspacePlan.Slug,
		Description: manifest.WorkspacePlan.Description,
	})
	if err != nil {
		return "", nil, err
	}
	return workspace.ID, &workspace, nil
}

func decodeBundleManifest(raw map[string]any) (platformdomain.ExtensionManifest, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return platformdomain.ExtensionManifest{}, err
	}
	var manifest platformdomain.ExtensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return platformdomain.ExtensionManifest{}, err
	}
	manifest.Normalize()
	return manifest, nil
}

func runExtensionInstall(ctx context.Context, client *cliapi.Client, workspaceID, licenseToken string, source bundleSourcePayload) (extensionOutput, error) {
	assets := make([]map[string]any, 0, len(source.Bundle.Assets))
	for _, asset := range source.Bundle.Assets {
		item := map[string]any{
			"path":    asset.Path,
			"content": asset.Content,
		}
		if strings.TrimSpace(asset.ContentType) != "" {
			item["contentType"] = asset.ContentType
		}
		if asset.IsCustomizable {
			item["isCustomizable"] = asset.IsCustomizable
		}
		assets = append(assets, item)
	}
	migrations := make([]map[string]any, 0, len(source.Bundle.Migrations))
	for _, migration := range source.Bundle.Migrations {
		migrations = append(migrations, map[string]any{
			"path":    migration.Path,
			"content": migration.Content,
		})
	}

	var payload struct {
		InstallExtension extensionOutput `json:"installExtension"`
	}
	err := client.Query(ctx, `
		mutation CLIInstallExtension($input: InstallExtensionInput!) {
		  installExtension(input: $input) {
		    id
		    workspaceID
		    slug
		    name
		    publisher
		    version
		    kind
		    scope
		    risk
		    status
		    validationStatus
		    validationMessage
		    healthStatus
		    healthMessage
		  }
		}
	`, map[string]any{
		"input": buildInstallExtensionInput(workspaceID, licenseToken, base64.StdEncoding.EncodeToString(source.Bytes), source.Bundle.Manifest, assets, migrations),
	}, &payload)
	if err != nil {
		return extensionOutput{}, err
	}
	return payload.InstallExtension, nil
}

func findInstalledExtensionBySlug(ctx context.Context, client *cliapi.Client, workspaceID string, instanceScope bool, slug string) (*extensionOutput, error) {
	var payload struct {
		Extensions         []extensionOutput `json:"extensions"`
		InstanceExtensions []extensionOutput `json:"instanceExtensions"`
	}

	query := `
		query CLIDeployExtensions($workspaceID: ID!) {
		  extensions(workspaceID: $workspaceID) {
		    id
		    workspaceID
		    slug
		    name
		    publisher
		    version
		    kind
		    scope
		    risk
		    status
		    validationStatus
		    validationMessage
		    healthStatus
		    healthMessage
		  }
		}
	`
	variables := map[string]any{"workspaceID": workspaceID}
	if instanceScope {
		query = `
			query CLIDeployInstanceExtensions {
			  instanceExtensions {
			    id
			    workspaceID
			    slug
			    name
			    publisher
			    version
			    kind
			    scope
			    risk
			    status
			    validationStatus
			    validationMessage
			    healthStatus
			    healthMessage
			  }
			}
		`
		variables = nil
	}
	if err := client.Query(ctx, query, variables, &payload); err != nil {
		return nil, err
	}
	extensions := payload.Extensions
	if instanceScope {
		extensions = payload.InstanceExtensions
	}
	for i := range extensions {
		if extensions[i].Slug == slug {
			return &extensions[i], nil
		}
	}
	return nil, nil
}

func runExtensionAction(ctx context.Context, client *cliapi.Client, action, id, reason string) (extensionOutput, error) {
	query, variables := mutationForAction(action, id, reason)
	var payload map[string]extensionOutput
	if err := client.Query(ctx, query, variables, &payload); err != nil {
		return extensionOutput{}, err
	}
	switch action {
	case "validate":
		return payload["validateExtension"], nil
	case "activate":
		return payload["activateExtension"], nil
	case "deactivate":
		return payload["deactivateExtension"], nil
	case "checkHealth":
		return payload["checkExtensionHealth"], nil
	default:
		return extensionOutput{}, fmt.Errorf("unsupported extension action %q", action)
	}
}

func runExtensionUpgrade(ctx context.Context, client *cliapi.Client, id, licenseToken string, source bundleSourcePayload) (extensionOutput, error) {
	assets := make([]map[string]any, 0, len(source.Bundle.Assets))
	for _, asset := range source.Bundle.Assets {
		item := map[string]any{
			"path":    asset.Path,
			"content": asset.Content,
		}
		if strings.TrimSpace(asset.ContentType) != "" {
			item["contentType"] = asset.ContentType
		}
		if asset.IsCustomizable {
			item["isCustomizable"] = asset.IsCustomizable
		}
		assets = append(assets, item)
	}
	migrations := make([]map[string]any, 0, len(source.Bundle.Migrations))
	for _, migration := range source.Bundle.Migrations {
		migrations = append(migrations, map[string]any{
			"path":    migration.Path,
			"content": migration.Content,
		})
	}

	input := map[string]any{
		"bundleBase64": base64.StdEncoding.EncodeToString(source.Bytes),
		"manifest":     source.Bundle.Manifest,
		"assets":       assets,
		"migrations":   migrations,
	}
	if strings.TrimSpace(licenseToken) != "" {
		input["licenseToken"] = licenseToken
	}

	var payload struct {
		UpgradeExtension extensionOutput `json:"upgradeExtension"`
	}
	err := client.Query(ctx, `
		mutation CLIUpgradeExtension($id: ID!, $input: UpgradeExtensionInput!) {
		  upgradeExtension(id: $id, input: $input) {
		    id
		    workspaceID
		    slug
		    name
		    publisher
		    version
		    kind
		    scope
		    risk
		    status
		    validationStatus
		    validationMessage
		    healthStatus
		    healthMessage
		  }
		}
	`, map[string]any{
		"id":    id,
		"input": input,
	}, &payload)
	if err != nil {
		return extensionOutput{}, err
	}
	return payload.UpgradeExtension, nil
}

func runExtensionConfigure(ctx context.Context, client *cliapi.Client, id string, config map[string]any) (extensionOutput, error) {
	var payload struct {
		UpdateExtensionConfig extensionOutput `json:"updateExtensionConfig"`
	}
	err := client.Query(ctx, `
		mutation CLIConfigureExtension($id: ID!, $input: UpdateExtensionConfigInput!) {
		  updateExtensionConfig(id: $id, input: $input) {
		    id
		    workspaceID
		    slug
		    name
		    publisher
		    version
		    kind
		    scope
		    risk
		    status
		    validationStatus
		    validationMessage
		    healthStatus
		    healthMessage
		  }
		}
	`, map[string]any{
		"id":    id,
		"input": map[string]any{"config": config},
	}, &payload)
	if err != nil {
		return extensionOutput{}, err
	}
	return payload.UpdateExtensionConfig, nil
}

func runAttachmentUpload(ctx context.Context, client *cliapi.Client, path string, params cliapi.AttachmentUploadParams) (*cliapi.AttachmentUploadResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat attachment: %w", err)
	}
	if info.Size() <= 0 {
		return nil, fmt.Errorf("attachment file is empty")
	}

	params.Filename = filepath.Base(path)
	params.Reader = file
	if strings.TrimSpace(params.ContentType) == "" {
		params.ContentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(params.Filename)))
		if params.ContentType == "" {
			params.ContentType = "application/octet-stream"
		}
	}

	return client.UploadAttachment(ctx, params)
}

func runServiceCatalogShow(ctx context.Context, client *cliapi.Client, identifier, workspaceID string) (serviceCatalogNodeOutput, error) {
	if strings.TrimSpace(workspaceID) != "" {
		node, err := runServiceCatalogShowByPath(ctx, client, workspaceID, identifier)
		if err != nil {
			return serviceCatalogNodeOutput{}, err
		}
		if node != nil {
			return *node, nil
		}
	}

	var payload struct {
		ServiceCatalogNode *serviceCatalogNodeOutput `json:"serviceCatalogNode"`
	}
	err := client.Query(ctx, `
		query CLIServiceCatalogNode($id: ID!) {
		  serviceCatalogNode(id: $id) {
		    `+serviceCatalogNodeDetailSelection+`
		  }
		}
	`, map[string]any{"id": identifier}, &payload)
	if err != nil {
		return serviceCatalogNodeOutput{}, err
	}
	if payload.ServiceCatalogNode == nil {
		return serviceCatalogNodeOutput{}, fmt.Errorf("catalog node not found")
	}
	return *payload.ServiceCatalogNode, nil
}

func runServiceCatalogShowByPath(ctx context.Context, client *cliapi.Client, workspaceID, path string) (*serviceCatalogNodeOutput, error) {
	var payload struct {
		ServiceCatalogNodeByPath *serviceCatalogNodeOutput `json:"serviceCatalogNodeByPath"`
	}
	err := client.Query(ctx, `
		query CLIServiceCatalogNodeByPath($workspaceID: ID!, $path: String!) {
		  serviceCatalogNodeByPath(workspaceID: $workspaceID, path: $path) {
		    `+serviceCatalogNodeDetailSelection+`
		  }
		}
	`, map[string]any{
		"workspaceID": workspaceID,
		"path":        path,
	}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.ServiceCatalogNodeByPath, nil
}

func runKnowledgeShow(ctx context.Context, client *cliapi.Client, identifier, workspaceID, teamID, surface string) (knowledgeResourceOutput, error) {
	if strings.TrimSpace(workspaceID) != "" && strings.TrimSpace(teamID) != "" {
		resource, err := runKnowledgeShowBySlug(ctx, client, workspaceID, teamID, surface, identifier)
		if err != nil {
			return knowledgeResourceOutput{}, err
		}
		if resource == nil {
			return knowledgeResourceOutput{}, fmt.Errorf("knowledge resource not found")
		}
		return *resource, nil
	}

	var payload struct {
		KnowledgeResource *knowledgeResourceOutput `json:"knowledgeResource"`
	}
	err := client.Query(ctx, `
		query CLIKnowledgeResource($id: ID!) {
		  knowledgeResource(id: $id) {
		    `+knowledgeSelection+`
		  }
		}
	`, map[string]any{"id": identifier}, &payload)
	if err != nil {
		return knowledgeResourceOutput{}, err
	}
	if payload.KnowledgeResource == nil {
		return knowledgeResourceOutput{}, fmt.Errorf("knowledge resource not found")
	}
	return *payload.KnowledgeResource, nil
}

func runKnowledgeShowBySlug(ctx context.Context, client *cliapi.Client, workspaceID, teamID, surface, slug string) (*knowledgeResourceOutput, error) {
	var payload struct {
		KnowledgeResourceBySlug *knowledgeResourceOutput `json:"knowledgeResourceBySlug"`
	}
	err := client.Query(ctx, `
		query CLIKnowledgeResourceBySlug($workspaceID: ID!, $teamID: ID!, $surface: KnowledgeSurface!, $slug: String!) {
		  knowledgeResourceBySlug(workspaceID: $workspaceID, teamID: $teamID, surface: $surface, slug: $slug) {
		    `+knowledgeSelection+`
		  }
		}
	`, map[string]any{
		"workspaceID": workspaceID,
		"teamID":      teamID,
		"surface":     strings.ToLower(strings.TrimSpace(surface)),
		"slug":        slug,
	}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.KnowledgeResourceBySlug, nil
}

func runKnowledgeCreate(ctx context.Context, client *cliapi.Client, input knowledgeMutationInput) (knowledgeResourceOutput, error) {
	var payload struct {
		CreateKnowledgeResource *knowledgeResourceOutput `json:"createKnowledgeResource"`
	}
	graphQLInput := map[string]any{
		"workspaceID": input.WorkspaceID,
		"teamID":      input.TeamID,
		"slug":        input.Slug,
		"title":       input.Title,
	}
	if input.Kind != "" {
		graphQLInput["kind"] = strings.ToLower(input.Kind)
	}
	if input.ConceptSpecKey != "" {
		graphQLInput["conceptSpecKey"] = input.ConceptSpecKey
	}
	if input.ConceptSpecVersion != "" {
		graphQLInput["conceptSpecVersion"] = input.ConceptSpecVersion
	}
	if input.Status != "" {
		graphQLInput["status"] = strings.ToLower(input.Status)
	}
	if input.Summary != "" {
		graphQLInput["summary"] = input.Summary
	}
	if input.BodyMarkdown != nil {
		graphQLInput["bodyMarkdown"] = *input.BodyMarkdown
	}
	if input.SourceKind != "" {
		graphQLInput["sourceKind"] = strings.ToLower(input.SourceKind)
	}
	if input.SourceRef != "" {
		graphQLInput["sourceRef"] = input.SourceRef
	}
	if input.PathRef != "" {
		graphQLInput["pathRef"] = input.PathRef
	}
	if len(input.SupportedChannels) > 0 {
		graphQLInput["supportedChannels"] = input.SupportedChannels
	}
	if len(input.SharedWithTeamIDs) > 0 {
		graphQLInput["sharedWithTeamIDs"] = input.SharedWithTeamIDs
	}
	if len(input.SearchKeywords) > 0 {
		graphQLInput["searchKeywords"] = input.SearchKeywords
	}
	if input.Surface != "" {
		graphQLInput["surface"] = strings.ToLower(input.Surface)
	}
	if input.Frontmatter != nil {
		graphQLInput["frontmatter"] = input.Frontmatter
	}

	err := client.Query(ctx, `
		mutation CLICreateKnowledgeResource($input: CreateKnowledgeResourceInput!) {
		  createKnowledgeResource(input: $input) {
		    `+knowledgeSelection+`
		  }
		}
	`, map[string]any{"input": graphQLInput}, &payload)
	if err != nil {
		return knowledgeResourceOutput{}, err
	}
	if payload.CreateKnowledgeResource == nil {
		return knowledgeResourceOutput{}, fmt.Errorf("create knowledge resource returned no resource")
	}
	return *payload.CreateKnowledgeResource, nil
}

func runKnowledgeUpdate(ctx context.Context, client *cliapi.Client, id string, input knowledgeMutationInput) (knowledgeResourceOutput, error) {
	var payload struct {
		UpdateKnowledgeResource *knowledgeResourceOutput `json:"updateKnowledgeResource"`
	}
	graphQLInput := map[string]any{}
	if input.Slug != "" {
		graphQLInput["slug"] = input.Slug
	}
	if input.Title != "" {
		graphQLInput["title"] = input.Title
	}
	if input.Kind != "" {
		graphQLInput["kind"] = strings.ToLower(input.Kind)
	}
	if input.ConceptSpecKey != "" {
		graphQLInput["conceptSpecKey"] = input.ConceptSpecKey
	}
	if input.ConceptSpecVersion != "" {
		graphQLInput["conceptSpecVersion"] = input.ConceptSpecVersion
	}
	if input.Status != "" {
		graphQLInput["status"] = strings.ToLower(input.Status)
	}
	if input.Summary != "" {
		graphQLInput["summary"] = input.Summary
	}
	if input.BodyMarkdown != nil {
		graphQLInput["bodyMarkdown"] = *input.BodyMarkdown
	}
	if input.SourceKind != "" {
		graphQLInput["sourceKind"] = strings.ToLower(input.SourceKind)
	}
	if input.SourceRef != "" {
		graphQLInput["sourceRef"] = input.SourceRef
	}
	if input.PathRef != "" {
		graphQLInput["pathRef"] = input.PathRef
	}
	if len(input.SupportedChannels) > 0 {
		graphQLInput["supportedChannels"] = input.SupportedChannels
	}
	if len(input.SearchKeywords) > 0 {
		graphQLInput["searchKeywords"] = input.SearchKeywords
	}
	if input.Frontmatter != nil {
		graphQLInput["frontmatter"] = input.Frontmatter
	}

	err := client.Query(ctx, `
		mutation CLIUpdateKnowledgeResource($id: ID!, $input: UpdateKnowledgeResourceInput!) {
		  updateKnowledgeResource(id: $id, input: $input) {
		    `+knowledgeSelection+`
		  }
		}
	`, map[string]any{
		"id":    id,
		"input": graphQLInput,
	}, &payload)
	if err != nil {
		return knowledgeResourceOutput{}, err
	}
	if payload.UpdateKnowledgeResource == nil {
		return knowledgeResourceOutput{}, fmt.Errorf("update knowledge resource returned no resource")
	}
	return *payload.UpdateKnowledgeResource, nil
}

func runKnowledgeReview(ctx context.Context, client *cliapi.Client, id, status string) (knowledgeResourceOutput, error) {
	var payload struct {
		ReviewKnowledgeResource *knowledgeResourceOutput `json:"reviewKnowledgeResource"`
	}
	variables := map[string]any{"id": id}
	if strings.TrimSpace(status) != "" {
		variables["status"] = strings.ToLower(strings.TrimSpace(status))
	}
	err := client.Query(ctx, `
		mutation CLIReviewKnowledgeResource($id: ID!, $status: KnowledgeReviewStatus) {
		  reviewKnowledgeResource(id: $id, status: $status) {
		    `+knowledgeSelection+`
		  }
		}
	`, variables, &payload)
	if err != nil {
		return knowledgeResourceOutput{}, err
	}
	if payload.ReviewKnowledgeResource == nil {
		return knowledgeResourceOutput{}, fmt.Errorf("review knowledge resource returned no resource")
	}
	return *payload.ReviewKnowledgeResource, nil
}

func runKnowledgePublish(ctx context.Context, client *cliapi.Client, id, surface string) (knowledgeResourceOutput, error) {
	var payload struct {
		PublishKnowledgeResource *knowledgeResourceOutput `json:"publishKnowledgeResource"`
	}
	variables := map[string]any{"id": id}
	if strings.TrimSpace(surface) != "" {
		variables["surface"] = strings.ToLower(strings.TrimSpace(surface))
	}
	err := client.Query(ctx, `
		mutation CLIPublishKnowledgeResource($id: ID!, $surface: KnowledgeSurface) {
		  publishKnowledgeResource(id: $id, surface: $surface) {
		    `+knowledgeSelection+`
		  }
		}
	`, variables, &payload)
	if err != nil {
		return knowledgeResourceOutput{}, err
	}
	if payload.PublishKnowledgeResource == nil {
		return knowledgeResourceOutput{}, fmt.Errorf("publish knowledge resource returned no resource")
	}
	return *payload.PublishKnowledgeResource, nil
}

func runKnowledgeShare(ctx context.Context, client *cliapi.Client, id string, teamIDs []string) (knowledgeResourceOutput, error) {
	var payload struct {
		ShareKnowledgeResource *knowledgeResourceOutput `json:"shareKnowledgeResource"`
	}
	err := client.Query(ctx, `
		mutation CLIShareKnowledgeResource($id: ID!, $input: ShareKnowledgeResourceInput!) {
		  shareKnowledgeResource(id: $id, input: $input) {
		    `+knowledgeSelection+`
		  }
		}
	`, map[string]any{
		"id":    id,
		"input": map[string]any{"teamIDs": teamIDs},
	}, &payload)
	if err != nil {
		return knowledgeResourceOutput{}, err
	}
	if payload.ShareKnowledgeResource == nil {
		return knowledgeResourceOutput{}, fmt.Errorf("share knowledge resource returned no resource")
	}
	return *payload.ShareKnowledgeResource, nil
}

func runKnowledgeDelete(ctx context.Context, client *cliapi.Client, id string) (knowledgeResourceOutput, error) {
	var payload struct {
		DeleteKnowledgeResource *knowledgeResourceOutput `json:"deleteKnowledgeResource"`
	}
	err := client.Query(ctx, `
		mutation CLIDeleteKnowledgeResource($id: ID!) {
		  deleteKnowledgeResource(id: $id) {
		    `+knowledgeSelection+`
		  }
		}
	`, map[string]any{"id": id}, &payload)
	if err != nil {
		return knowledgeResourceOutput{}, err
	}
	if payload.DeleteKnowledgeResource == nil {
		return knowledgeResourceOutput{}, fmt.Errorf("delete knowledge resource returned no resource")
	}
	return *payload.DeleteKnowledgeResource, nil
}

func runKnowledgeHistory(ctx context.Context, client *cliapi.Client, id string, limit int) ([]knowledgeRevisionOutput, error) {
	var payload struct {
		KnowledgeResourceHistory []knowledgeRevisionOutput `json:"knowledgeResourceHistory"`
	}
	err := client.Query(ctx, `
		query CLIKnowledgeHistory($id: ID!, $limit: Int) {
		  knowledgeResourceHistory(id: $id, limit: $limit) {
		    ref
		    committedAt
		    subject
		  }
		}
	`, map[string]any{
		"id":    id,
		"limit": limit,
	}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.KnowledgeResourceHistory, nil
}

func runKnowledgeDiff(ctx context.Context, client *cliapi.Client, id, fromRevision, toRevision string) (knowledgeDiffOutput, error) {
	var payload struct {
		KnowledgeResourceDiff *knowledgeDiffOutput `json:"knowledgeResourceDiff"`
	}
	variables := map[string]any{"id": id}
	if strings.TrimSpace(fromRevision) != "" {
		variables["fromRevision"] = strings.TrimSpace(fromRevision)
	}
	if strings.TrimSpace(toRevision) != "" {
		variables["toRevision"] = strings.TrimSpace(toRevision)
	}
	err := client.Query(ctx, `
		query CLIKnowledgeDiff($id: ID!, $fromRevision: String, $toRevision: String) {
		  knowledgeResourceDiff(id: $id, fromRevision: $fromRevision, toRevision: $toRevision) {
		    path
		    fromRevision
		    toRevision
		    patch
		  }
		}
	`, variables, &payload)
	if err != nil {
		return knowledgeDiffOutput{}, err
	}
	if payload.KnowledgeResourceDiff == nil {
		return knowledgeDiffOutput{}, fmt.Errorf("knowledge diff returned no payload")
	}
	return *payload.KnowledgeResourceDiff, nil
}

func runConceptSpecList(ctx context.Context, client *cliapi.Client, workspaceID string) ([]conceptSpecOutput, error) {
	var payload struct {
		ConceptSpecs []conceptSpecOutput `json:"conceptSpecs"`
	}
	variables := map[string]any{}
	if strings.TrimSpace(workspaceID) != "" {
		variables["workspaceID"] = workspaceID
	}
	if err := client.Query(ctx, `
		query CLIConceptSpecs($workspaceID: ID) {
		  conceptSpecs(workspaceID: $workspaceID) {
		    `+conceptSpecSelection+`
		  }
		}
	`, variables, &payload); err != nil {
		return nil, err
	}
	return payload.ConceptSpecs, nil
}

func runConceptSpecShow(ctx context.Context, client *cliapi.Client, key, workspaceID, version string) (conceptSpecOutput, error) {
	var payload struct {
		ConceptSpec *conceptSpecOutput `json:"conceptSpec"`
	}
	variables := map[string]any{
		"key": key,
	}
	if strings.TrimSpace(workspaceID) != "" {
		variables["workspaceID"] = workspaceID
	}
	if strings.TrimSpace(version) != "" {
		variables["version"] = version
	}
	if err := client.Query(ctx, `
		query CLIConceptSpec($workspaceID: ID, $key: String!, $version: String) {
		  conceptSpec(workspaceID: $workspaceID, key: $key, version: $version) {
		    `+conceptSpecSelection+`
		  }
		}
	`, variables, &payload); err != nil {
		return conceptSpecOutput{}, err
	}
	if payload.ConceptSpec == nil {
		return conceptSpecOutput{}, fmt.Errorf("concept spec not found")
	}
	return *payload.ConceptSpec, nil
}

func runConceptSpecRegister(ctx context.Context, client *cliapi.Client, input conceptSpecMutationInput) (conceptSpecOutput, error) {
	var payload struct {
		RegisterConceptSpec *conceptSpecOutput `json:"registerConceptSpec"`
	}
	graphQLInput := map[string]any{
		"workspaceID":  input.WorkspaceID,
		"key":          input.Key,
		"name":         input.Name,
		"instanceKind": strings.ToLower(strings.TrimSpace(input.InstanceKind)),
	}
	if input.OwnerTeamID != "" {
		graphQLInput["ownerTeamID"] = input.OwnerTeamID
	}
	if input.Version != "" {
		graphQLInput["version"] = input.Version
	}
	if input.Description != "" {
		graphQLInput["description"] = input.Description
	}
	if input.ExtendsKey != "" {
		graphQLInput["extendsKey"] = input.ExtendsKey
	}
	if input.ExtendsVersion != "" {
		graphQLInput["extendsVersion"] = input.ExtendsVersion
	}
	if len(input.MetadataSchema) > 0 {
		graphQLInput["metadataSchema"] = input.MetadataSchema
	}
	if len(input.SectionsSchema) > 0 {
		graphQLInput["sectionsSchema"] = input.SectionsSchema
	}
	if len(input.WorkflowSchema) > 0 {
		graphQLInput["workflowSchema"] = input.WorkflowSchema
	}
	if input.AgentGuidanceMarkdown != "" {
		graphQLInput["agentGuidanceMarkdown"] = input.AgentGuidanceMarkdown
	}
	if input.SourceKind != "" {
		graphQLInput["sourceKind"] = strings.ToLower(input.SourceKind)
	}
	if input.SourceRef != "" {
		graphQLInput["sourceRef"] = input.SourceRef
	}
	if input.Status != "" {
		graphQLInput["status"] = strings.ToLower(input.Status)
	}
	if err := client.Query(ctx, `
		mutation CLIRegisterConceptSpec($input: RegisterConceptSpecInput!) {
		  registerConceptSpec(input: $input) {
		    `+conceptSpecSelection+`
		  }
		}
	`, map[string]any{"input": graphQLInput}, &payload); err != nil {
		return conceptSpecOutput{}, err
	}
	if payload.RegisterConceptSpec == nil {
		return conceptSpecOutput{}, fmt.Errorf("register concept spec returned no payload")
	}
	return *payload.RegisterConceptSpec, nil
}

func runConceptSpecHistory(ctx context.Context, client *cliapi.Client, workspaceID, key, version string, limit int) ([]artifactRevisionOutput, error) {
	var payload struct {
		ConceptSpecHistory []artifactRevisionOutput `json:"conceptSpecHistory"`
	}
	variables := map[string]any{
		"key":   key,
		"limit": limit,
	}
	if strings.TrimSpace(workspaceID) != "" {
		variables["workspaceID"] = workspaceID
	}
	if strings.TrimSpace(version) != "" {
		variables["version"] = version
	}
	err := client.Query(ctx, `
		query CLIConceptSpecHistory($workspaceID: ID, $key: String!, $version: String, $limit: Int) {
		  conceptSpecHistory(workspaceID: $workspaceID, key: $key, version: $version, limit: $limit) {
		    ref
		    committedAt
		    subject
		  }
		}
	`, variables, &payload)
	if err != nil {
		return nil, err
	}
	return payload.ConceptSpecHistory, nil
}

func runConceptSpecDiff(ctx context.Context, client *cliapi.Client, workspaceID, key, version, fromRevision, toRevision string) (knowledgeDiffOutput, error) {
	var payload struct {
		ConceptSpecDiff *knowledgeDiffOutput `json:"conceptSpecDiff"`
	}
	variables := map[string]any{
		"key": key,
	}
	if strings.TrimSpace(workspaceID) != "" {
		variables["workspaceID"] = workspaceID
	}
	if strings.TrimSpace(version) != "" {
		variables["version"] = version
	}
	if strings.TrimSpace(fromRevision) != "" {
		variables["fromRevision"] = fromRevision
	}
	if strings.TrimSpace(toRevision) != "" {
		variables["toRevision"] = toRevision
	}
	err := client.Query(ctx, `
		query CLIConceptSpecDiff($workspaceID: ID, $key: String!, $version: String, $fromRevision: String, $toRevision: String) {
		  conceptSpecDiff(workspaceID: $workspaceID, key: $key, version: $version, fromRevision: $fromRevision, toRevision: $toRevision) {
		    path
		    fromRevision
		    toRevision
		    patch
		  }
		}
	`, variables, &payload)
	if err != nil {
		return knowledgeDiffOutput{}, err
	}
	if payload.ConceptSpecDiff == nil {
		return knowledgeDiffOutput{}, fmt.Errorf("concept spec diff returned no payload")
	}
	return *payload.ConceptSpecDiff, nil
}

func runConversationList(ctx context.Context, client *cliapi.Client, input conversationListInput) ([]conversationSessionOutput, error) {
	filter := map[string]any{
		"limit": input.Limit,
	}
	if input.Offset > 0 {
		filter["offset"] = input.Offset
	}
	if value := strings.TrimSpace(input.Status); value != "" {
		filter["status"] = value
	}
	if value := strings.TrimSpace(input.Channel); value != "" {
		filter["channel"] = value
	}
	if value := strings.TrimSpace(input.PrimaryCatalogNodeID); value != "" {
		filter["primaryCatalogNodeID"] = value
	}
	if value := strings.TrimSpace(input.PrimaryContactID); value != "" {
		filter["primaryContactID"] = value
	}
	if value := strings.TrimSpace(input.LinkedCaseID); value != "" {
		filter["linkedCaseID"] = value
	}

	var payload struct {
		ConversationSessions []conversationSessionOutput `json:"conversationSessions"`
	}
	err := client.Query(ctx, `
		query CLIConversationSessions($workspaceID: ID!, $filter: ConversationSessionFilter) {
		  conversationSessions(workspaceID: $workspaceID, filter: $filter) {
		    `+conversationListSelection+`
		  }
		}
	`, map[string]any{
		"workspaceID": input.WorkspaceID,
		"filter":      filter,
	}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.ConversationSessions, nil
}

func runConversationShow(ctx context.Context, client *cliapi.Client, id string) (conversationSessionOutput, error) {
	var payload struct {
		ConversationSession *conversationSessionOutput `json:"conversationSession"`
	}
	err := client.Query(ctx, `
		query CLIConversationSession($id: ID!) {
		  conversationSession(id: $id) {
		    `+conversationDetailSelection+`
		  }
		}
	`, map[string]any{"id": id}, &payload)
	if err != nil {
		return conversationSessionOutput{}, err
	}
	if payload.ConversationSession == nil {
		return conversationSessionOutput{}, fmt.Errorf("conversation not found")
	}
	return *payload.ConversationSession, nil
}

func runConversationReply(ctx context.Context, client *cliapi.Client, input conversationReplyInput) (conversationMessageOutput, error) {
	mutationInput := map[string]any{
		"contentText": input.ContentText,
	}
	if value := strings.TrimSpace(input.ParticipantID); value != "" {
		mutationInput["participantID"] = value
	}
	if value := strings.TrimSpace(input.Role); value != "" {
		mutationInput["role"] = value
	}
	if value := strings.TrimSpace(input.Kind); value != "" {
		mutationInput["kind"] = value
	}
	if value := strings.TrimSpace(input.Visibility); value != "" {
		mutationInput["visibility"] = value
	}

	var payload struct {
		AddConversationMessage conversationMessageOutput `json:"addConversationMessage"`
	}
	err := client.Query(ctx, `
		mutation CLIAddConversationMessage($sessionID: ID!, $input: AddConversationMessageInput!) {
		  addConversationMessage(sessionID: $sessionID, input: $input) {
		    `+conversationMessageSelection+`
		  }
		}
	`, map[string]any{
		"sessionID": input.SessionID,
		"input":     mutationInput,
	}, &payload)
	if err != nil {
		return conversationMessageOutput{}, err
	}
	return payload.AddConversationMessage, nil
}

func runExtensionArtifactList(ctx context.Context, client *cliapi.Client, extensionID, surface string) ([]extensionArtifactFileOutput, error) {
	var payload struct {
		ExtensionArtifactFiles []extensionArtifactFileOutput `json:"extensionArtifactFiles"`
	}
	err := client.Query(ctx, `
		query CLIExtensionArtifactFiles($id: ID!, $surface: String!) {
		  extensionArtifactFiles(id: $id, surface: $surface) {
		    surface
		    path
		  }
		}
	`, map[string]any{
		"id":      extensionID,
		"surface": strings.TrimSpace(surface),
	}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.ExtensionArtifactFiles, nil
}

func runExtensionArtifactShow(ctx context.Context, client *cliapi.Client, extensionID, surface, artifactPath, ref string) (string, error) {
	var payload struct {
		ExtensionArtifactContent *string `json:"extensionArtifactContent"`
	}
	variables := map[string]any{
		"id":      extensionID,
		"surface": strings.TrimSpace(surface),
		"path":    strings.TrimSpace(artifactPath),
	}
	if strings.TrimSpace(ref) != "" {
		variables["ref"] = strings.TrimSpace(ref)
	}
	err := client.Query(ctx, `
		query CLIExtensionArtifactContent($id: ID!, $surface: String!, $path: String!, $ref: String) {
		  extensionArtifactContent(id: $id, surface: $surface, path: $path, ref: $ref)
		}
	`, variables, &payload)
	if err != nil {
		return "", err
	}
	if payload.ExtensionArtifactContent == nil {
		return "", fmt.Errorf("extension artifact not found")
	}
	return *payload.ExtensionArtifactContent, nil
}

func runExtensionArtifactHistory(ctx context.Context, client *cliapi.Client, extensionID, surface, artifactPath string, limit int) ([]artifactRevisionOutput, error) {
	var payload struct {
		ExtensionArtifactHistory []artifactRevisionOutput `json:"extensionArtifactHistory"`
	}
	err := client.Query(ctx, `
		query CLIExtensionArtifactHistory($id: ID!, $surface: String!, $path: String!, $limit: Int) {
		  extensionArtifactHistory(id: $id, surface: $surface, path: $path, limit: $limit) {
		    ref
		    committedAt
		    subject
		  }
		}
	`, map[string]any{
		"id":      extensionID,
		"surface": strings.TrimSpace(surface),
		"path":    strings.TrimSpace(artifactPath),
		"limit":   limit,
	}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.ExtensionArtifactHistory, nil
}

func runExtensionArtifactDiff(ctx context.Context, client *cliapi.Client, extensionID, surface, artifactPath, fromRevision, toRevision string) (artifactDiffOutput, error) {
	var payload struct {
		ExtensionArtifactDiff *artifactDiffOutput `json:"extensionArtifactDiff"`
	}
	variables := map[string]any{
		"id":      extensionID,
		"surface": strings.TrimSpace(surface),
		"path":    strings.TrimSpace(artifactPath),
	}
	if strings.TrimSpace(fromRevision) != "" {
		variables["fromRevision"] = strings.TrimSpace(fromRevision)
	}
	if strings.TrimSpace(toRevision) != "" {
		variables["toRevision"] = strings.TrimSpace(toRevision)
	}
	err := client.Query(ctx, `
		query CLIExtensionArtifactDiff($id: ID!, $surface: String!, $path: String!, $fromRevision: String, $toRevision: String) {
		  extensionArtifactDiff(id: $id, surface: $surface, path: $path, fromRevision: $fromRevision, toRevision: $toRevision) {
		    fromRevision
		    toRevision
		    patch
		  }
		}
	`, variables, &payload)
	if err != nil {
		return artifactDiffOutput{}, err
	}
	if payload.ExtensionArtifactDiff == nil {
		return artifactDiffOutput{}, fmt.Errorf("extension artifact diff returned no payload")
	}
	return *payload.ExtensionArtifactDiff, nil
}

func runExtensionArtifactPublish(ctx context.Context, client *cliapi.Client, extensionID, surface, artifactPath, content string) (extensionArtifactPublicationOutput, error) {
	var payload struct {
		PublishExtensionArtifact *extensionArtifactPublicationOutput `json:"publishExtensionArtifact"`
	}
	err := client.Query(ctx, `
		mutation CLIPublishExtensionArtifact($id: ID!, $input: PublishExtensionArtifactInput!) {
		  publishExtensionArtifact(id: $id, input: $input) {
		    surface
		    path
		    revisionRef
		  }
		}
	`, map[string]any{
		"id": extensionID,
		"input": map[string]any{
			"surface": strings.TrimSpace(surface),
			"path":    strings.TrimSpace(artifactPath),
			"content": content,
		},
	}, &payload)
	if err != nil {
		return extensionArtifactPublicationOutput{}, err
	}
	if payload.PublishExtensionArtifact == nil {
		return extensionArtifactPublicationOutput{}, fmt.Errorf("extension artifact publish returned no payload")
	}
	return *payload.PublishExtensionArtifact, nil
}

func readArtifactPublishContent(filePath, inlineContent string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	inlineContent = strings.TrimSpace(inlineContent)
	switch {
	case filePath == "" && inlineContent == "":
		return "", fmt.Errorf("either --file or --content is required")
	case filePath != "" && inlineContent != "":
		return "", fmt.Errorf("--file and --content cannot be used together")
	case filePath != "":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read artifact file: %w", err)
		}
		return string(data), nil
	default:
		return inlineContent, nil
	}
}

func runQueueCreate(ctx context.Context, client *cliapi.Client, workspaceID, name, slug, description string) (queueOutput, error) {
	var payload struct {
		CreateQueue *queueOutput `json:"createQueue"`
	}
	input := map[string]any{
		"workspaceID": workspaceID,
		"name":        strings.TrimSpace(name),
	}
	if value := strings.TrimSpace(slug); value != "" {
		input["slug"] = value
	}
	if value := strings.TrimSpace(description); value != "" {
		input["description"] = value
	}

	err := client.Query(ctx, `
		mutation CLICreateQueue($input: CreateQueueInput!) {
		  createQueue(input: $input) {
		    id
		    workspaceID
		    slug
		    name
		    description
		  }
		}
	`, map[string]any{"input": input}, &payload)
	if err != nil {
		return queueOutput{}, err
	}
	if payload.CreateQueue == nil {
		return queueOutput{}, fmt.Errorf("create queue returned no queue")
	}
	return *payload.CreateQueue, nil
}

func mutationForAction(action, id, reason string) (string, map[string]any) {
	const selection = `
		id
		workspaceID
		slug
		name
		publisher
		version
		kind
		scope
		risk
		status
		validationStatus
		validationMessage
		healthStatus
		healthMessage
	`

	switch action {
	case "validate":
		return "mutation CLIValidateExtension($id: ID!) { validateExtension(id: $id) { " + selection + " } }", map[string]any{"id": id}
	case "activate":
		return "mutation CLIActivateExtension($id: ID!) { activateExtension(id: $id) { " + selection + " } }", map[string]any{"id": id}
	case "deactivate":
		return "mutation CLIDeactivateExtension($id: ID!, $reason: String) { deactivateExtension(id: $id, reason: $reason) { " + selection + " } }", map[string]any{
			"id":     id,
			"reason": reason,
		}
	case "checkHealth":
		return "mutation CLICheckExtensionHealth($id: ID!) { checkExtensionHealth(id: $id) { " + selection + " } }", map[string]any{"id": id}
	default:
		panic("unsupported extension action")
	}
}

func fetchExtensionDetail(ctx context.Context, client *cliapi.Client, id string) (extensionDetailOutput, error) {
	var payload struct {
		Extension *extensionDetailOutput `json:"extension"`
	}
	err := client.Query(ctx, `
		query CLIExtension($id: ID!) {
		  extension(id: $id) {
		    id
		    workspaceID
		    slug
		    name
		    publisher
		    version
		    description
		    kind
		    scope
		    risk
		    runtimeClass
		    storageClass
		    schema {
		      name
		      packageKey
		      targetVersion
		      migrationEngine
		    }
		    workspacePlan {
		      mode
		      name
		      slug
		      description
		    }
		    permissions
		    artifactSurfaces {
		      name
		      description
		      seedAssetPath
		    }
		    publicRoutes {
		      pathPrefix
		      assetPath
		      artifactSurface
		      artifactPath
		    }
		    adminRoutes {
		      pathPrefix
		      assetPath
		      artifactSurface
		      artifactPath
		    }
		    endpoints {
		      name
		      class
		      mountPath
		      methods
		      auth
		      contentTypes
		      maxBodyBytes
		      rateLimitPerMinute
		      workspaceBinding
		      assetPath
		      artifactSurface
		      artifactPath
		      serviceTarget
		    }
		    adminNavigation {
		      name
		      section
		      title
		      icon
		      endpoint
		      activePage
		    }
		    dashboardWidgets {
		      name
		      title
		      description
		      icon
		      endpoint
		    }
		    resolvedAdminNavigation {
		      extensionID
		      extensionSlug
		      workspaceID
		      section
		      title
		      icon
		      href
		      activePage
		    }
		    resolvedDashboardWidgets {
		      extensionID
		      extensionSlug
		      workspaceID
		      title
		      description
		      icon
		      href
		    }
		    seededResources {
		      queues {
		        slug
		        resourceID
		        exists
		        matchesSeed
		        problems
		        expected
		        actual
		      }
		      forms {
		        slug
		        resourceID
		        exists
		        matchesSeed
		        problems
		        expected
		        actual
		      }
		      automationRules {
		        key
		        resourceID
		        exists
		        matchesSeed
		        problems
		        expected
		        actual
		      }
		    }
		    events {
		      publishes {
		        type
		        description
		        schemaVersion
		      }
		      subscribes
		    }
		    eventConsumers {
		      name
		      description
		      stream
		      eventTypes
		      consumerGroup
		      serviceTarget
		    }
		    scheduledJobs {
		      name
		      description
		      intervalSeconds
		      serviceTarget
		    }
		    commands {
		      name
		      description
		    }
		    agentSkills {
		      name
		      description
		      assetPath
		    }
		    customizableAssets
		    status
		    validationStatus
		    validationMessage
		    healthStatus
		    healthMessage
		    bundleSHA256
		    bundleSize
		    installedByID
		    installedAt
		    activatedAt
		    deactivatedAt
		    validatedAt
		    lastHealthCheckAt
		    runtimeDiagnostics {
		      bootstrapStatus
		      lastBootstrapAt
		      lastBootstrapError
		      endpoints {
		        name
		        class
		        mountPath
		        serviceTarget
		        status
		        consecutiveFailures
		        registeredAt
		        lastCheckedAt
		        lastSuccessAt
		        lastFailureAt
		        lastError
		      }
		      eventConsumers {
		        name
		        stream
		        consumerGroup
		        serviceTarget
		        status
		        consecutiveFailures
		        registeredAt
		        lastDeliveredAt
		        lastSuccessAt
		        lastFailureAt
		        lastError
		      }
		      scheduledJobs {
		        name
		        intervalSeconds
		        serviceTarget
		        status
		        consecutiveFailures
		        registeredAt
		        lastStartedAt
		        lastSuccessAt
		        lastFailureAt
		        backoffUntil
		        lastError
		      }
		    }
		    assets {
		      id
		      path
		      kind
		      contentType
		      isCustomizable
		      checksum
		      size
		      updatedAt
		    }
		  }
		}
	`, map[string]any{"id": id}, &payload)
	if err != nil {
		return extensionDetailOutput{}, err
	}
	if payload.Extension == nil {
		return extensionDetailOutput{}, fmt.Errorf("extension not found")
	}
	return *payload.Extension, nil
}

func fetchExtensionSkill(ctx context.Context, client *cliapi.Client, id, name string) (extensionSkillDetailOutput, error) {
	skills, err := fetchExtensionSkills(ctx, client, id)
	if err != nil {
		return extensionSkillDetailOutput{}, err
	}

	var selected *extensionAgentSkillOutput
	for i := range skills {
		if skills[i].Name == name {
			selected = &skills[i]
			break
		}
	}
	if selected == nil {
		return extensionSkillDetailOutput{}, fmt.Errorf("extension skill %q not found", name)
	}

	var payload struct {
		Extension *struct {
			ID     string `json:"id"`
			Slug   string `json:"slug"`
			Assets []struct {
				Path        string  `json:"path"`
				ContentType string  `json:"contentType"`
				TextContent *string `json:"textContent"`
			} `json:"assets"`
		} `json:"extension"`
	}

	err = client.Query(ctx, `
		query CLIExtensionSkillAsset($id: ID!) {
		  extension(id: $id) {
		    id
		    slug
		    assets {
		      path
		      contentType
		      textContent
		    }
		  }
		}
	`, map[string]any{"id": id}, &payload)
	if err != nil {
		return extensionSkillDetailOutput{}, err
	}
	if payload.Extension == nil {
		return extensionSkillDetailOutput{}, fmt.Errorf("extension not found")
	}

	for _, asset := range payload.Extension.Assets {
		if asset.Path != selected.AssetPath {
			continue
		}
		if asset.TextContent == nil {
			return extensionSkillDetailOutput{}, fmt.Errorf("extension skill %q asset %s is not readable as text", name, selected.AssetPath)
		}
		return extensionSkillDetailOutput{
			ExtensionID:   payload.Extension.ID,
			ExtensionSlug: payload.Extension.Slug,
			Name:          selected.Name,
			Description:   selected.Description,
			AssetPath:     selected.AssetPath,
			ContentType:   asset.ContentType,
			Content:       *asset.TextContent,
		}, nil
	}

	return extensionSkillDetailOutput{}, fmt.Errorf("extension skill %q asset %s not found", name, selected.AssetPath)
}

func fetchExtensionSkills(ctx context.Context, client *cliapi.Client, id string) ([]extensionAgentSkillOutput, error) {
	var payload struct {
		Extension *struct {
			ID          string                      `json:"id"`
			Slug        string                      `json:"slug"`
			AgentSkills []extensionAgentSkillOutput `json:"agentSkills"`
		} `json:"extension"`
	}

	err := client.Query(ctx, `
		query CLIExtensionSkills($id: ID!) {
		  extension(id: $id) {
		    id
		    slug
		    agentSkills {
		      name
		      description
		      assetPath
		    }
		  }
		}
	`, map[string]any{"id": id}, &payload)
	if err != nil {
		return nil, err
	}
	if payload.Extension == nil {
		return nil, fmt.Errorf("extension not found")
	}
	return payload.Extension.AgentSkills, nil
}

func fetchExtensionEventCatalog(ctx context.Context, client *cliapi.Client, workspaceID string) ([]extensionRuntimeEventOutput, error) {
	var payload struct {
		ExtensionEventCatalog []extensionRuntimeEventOutput `json:"extensionEventCatalog"`
	}

	err := client.Query(ctx, `
		query CLIExtensionEventCatalog($workspaceID: ID!) {
		  extensionEventCatalog(workspaceID: $workspaceID) {
		    type
		    description
		    schemaVersion
		    core
		    publishers
		    subscribers
		  }
		}
	`, map[string]any{"workspaceID": workspaceID}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.ExtensionEventCatalog, nil
}

func buildInstallExtensionInput(
	workspaceID, licenseToken, bundleBase64 string,
	manifest map[string]any,
	assets []map[string]any,
	migrations []map[string]any,
) map[string]any {
	input := map[string]any{
		"bundleBase64": bundleBase64,
		"manifest":     manifest,
		"assets":       assets,
		"migrations":   migrations,
	}
	if strings.TrimSpace(licenseToken) != "" {
		input["licenseToken"] = licenseToken
	}
	if strings.TrimSpace(workspaceID) != "" {
		input["workspaceID"] = workspaceID
	}
	return input
}

func printExtensionDetail(stdout io.Writer, extension extensionDetailOutput) {
	fmt.Fprintf(stdout, "id:\t%s\n", extension.ID)
	fmt.Fprintf(stdout, "workspaceID:\t%s\n", coalesce(extension.WorkspaceID, "instance"))
	fmt.Fprintf(stdout, "slug:\t%s\n", extension.Slug)
	fmt.Fprintf(stdout, "name:\t%s\n", extension.Name)
	fmt.Fprintf(stdout, "publisher:\t%s\n", extension.Publisher)
	fmt.Fprintf(stdout, "version:\t%s\n", extension.Version)
	fmt.Fprintf(stdout, "kind:\t%s\n", extension.Kind)
	fmt.Fprintf(stdout, "scope:\t%s\n", extension.Scope)
	fmt.Fprintf(stdout, "risk:\t%s\n", extension.Risk)
	fmt.Fprintf(stdout, "runtimeClass:\t%s\n", extension.RuntimeClass)
	fmt.Fprintf(stdout, "storageClass:\t%s\n", extension.StorageClass)
	fmt.Fprintf(stdout, "status:\t%s\n", extension.Status)
	fmt.Fprintf(stdout, "validation:\t%s\t%s\n", extension.ValidationStatus, coalesce(extension.ValidationMessage, ""))
	fmt.Fprintf(stdout, "health:\t%s\t%s\n", extension.HealthStatus, coalesce(extension.HealthMessage, ""))
	fmt.Fprintf(stdout, "bundleSHA256:\t%s\n", extension.BundleSHA256)
	fmt.Fprintf(stdout, "bundleSize:\t%d\n", extension.BundleSize)
	fmt.Fprintf(stdout, "installedAt:\t%s\n", extension.InstalledAt)
	if extension.ActivatedAt != nil {
		fmt.Fprintf(stdout, "activatedAt:\t%s\n", *extension.ActivatedAt)
	}
	if extension.ValidatedAt != nil {
		fmt.Fprintf(stdout, "validatedAt:\t%s\n", *extension.ValidatedAt)
	}
	if extension.LastHealthCheckAt != nil {
		fmt.Fprintf(stdout, "lastHealthCheckAt:\t%s\n", *extension.LastHealthCheckAt)
	}
	if extension.WorkspacePlan != nil && extension.WorkspacePlan.Mode != nil {
		fmt.Fprintf(stdout, "workspacePlan:\t%s\t%s\t%s\n",
			*extension.WorkspacePlan.Mode,
			coalesce(extension.WorkspacePlan.Name, ""),
			coalesce(extension.WorkspacePlan.Slug, ""),
		)
	}
	if extension.Schema != nil {
		fmt.Fprintf(stdout, "schema:\t%s\t%s\t%s\t%s\n",
			extension.Schema.Name,
			extension.Schema.PackageKey,
			extension.Schema.TargetVersion,
			extension.Schema.MigrationEngine,
		)
	}
	if len(extension.ArtifactSurfaces) > 0 {
		fmt.Fprintln(stdout, "artifactSurfaces:")
		for _, surface := range extension.ArtifactSurfaces {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\n", surface.Name, coalesce(surface.Description, ""), coalesce(surface.SeedAssetPath, ""))
		}
	}
	if len(extension.PublicRoutes) > 0 {
		fmt.Fprintln(stdout, "publicRoutes:")
		for _, route := range extension.PublicRoutes {
			target := coalesce(route.AssetPath, "")
			if route.ArtifactSurface != nil {
				target = fmt.Sprintf("artifact:%s/%s", *route.ArtifactSurface, coalesce(route.ArtifactPath, ""))
			}
			fmt.Fprintf(stdout, "  %s -> %s\n", route.PathPrefix, target)
		}
	}
	if len(extension.AdminRoutes) > 0 {
		fmt.Fprintln(stdout, "adminRoutes:")
		for _, route := range extension.AdminRoutes {
			target := coalesce(route.AssetPath, "")
			if route.ArtifactSurface != nil {
				target = fmt.Sprintf("artifact:%s/%s", *route.ArtifactSurface, coalesce(route.ArtifactPath, ""))
			}
			fmt.Fprintf(stdout, "  %s -> %s\n", route.PathPrefix, target)
		}
	}
	if len(extension.Endpoints) > 0 {
		fmt.Fprintln(stdout, "endpoints:")
		for _, endpoint := range extension.Endpoints {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\t%s\n",
				endpoint.Name,
				endpoint.Class,
				endpoint.Auth,
				strings.Join(endpoint.Methods, ","),
				endpoint.MountPath,
			)
		}
	}
	if len(extension.AdminNavigation) > 0 {
		fmt.Fprintln(stdout, "adminNavigation:")
		for _, item := range extension.AdminNavigation {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\n",
				item.Name,
				coalesce(item.Section, ""),
				item.Title,
				item.Endpoint,
			)
		}
	}
	if len(extension.DashboardWidgets) > 0 {
		fmt.Fprintln(stdout, "dashboardWidgets:")
		for _, widget := range extension.DashboardWidgets {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\n",
				widget.Name,
				widget.Title,
				widget.Endpoint,
			)
		}
	}
	if len(extension.ResolvedAdminNavigation) > 0 {
		fmt.Fprintln(stdout, "resolvedAdminNavigation:")
		for _, item := range extension.ResolvedAdminNavigation {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\n",
				item.ExtensionSlug,
				coalesce(item.Section, ""),
				item.Title,
				item.Href,
			)
		}
	}
	if len(extension.ResolvedDashboardWidgets) > 0 {
		fmt.Fprintln(stdout, "resolvedDashboardWidgets:")
		for _, widget := range extension.ResolvedDashboardWidgets {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\n",
				widget.ExtensionSlug,
				widget.Title,
				widget.Href,
			)
		}
	}
	if len(extension.SeededResources.Queues) > 0 || len(extension.SeededResources.Forms) > 0 || len(extension.SeededResources.AutomationRules) > 0 {
		fmt.Fprintln(stdout, "seededResources:")
		for _, queue := range extension.SeededResources.Queues {
			fmt.Fprintf(stdout, "  queue\t%s\t%s\t%s\n",
				queue.Slug,
				extensionSeedStateLabel(queue.Exists, queue.MatchesSeed),
				coalesce(queue.ResourceID, ""),
			)
			for _, problem := range queue.Problems {
				fmt.Fprintf(stdout, "    problem\t%s\n", problem)
			}
		}
		for _, form := range extension.SeededResources.Forms {
			fmt.Fprintf(stdout, "  form\t%s\t%s\t%s\n",
				form.Slug,
				extensionSeedStateLabel(form.Exists, form.MatchesSeed),
				coalesce(form.ResourceID, ""),
			)
			for _, problem := range form.Problems {
				fmt.Fprintf(stdout, "    problem\t%s\n", problem)
			}
		}
		for _, rule := range extension.SeededResources.AutomationRules {
			fmt.Fprintf(stdout, "  automationRule\t%s\t%s\t%s\n",
				rule.Key,
				extensionSeedStateLabel(rule.Exists, rule.MatchesSeed),
				coalesce(rule.ResourceID, ""),
			)
			for _, problem := range rule.Problems {
				fmt.Fprintf(stdout, "    problem\t%s\n", problem)
			}
		}
	}
	if len(extension.Events.Publishes) > 0 || len(extension.Events.Subscribes) > 0 {
		fmt.Fprintln(stdout, "events:")
		for _, event := range extension.Events.Publishes {
			fmt.Fprintf(stdout, "  publishes\t%s\tv%d\n", event.Type, event.SchemaVersion)
		}
		for _, eventType := range extension.Events.Subscribes {
			fmt.Fprintf(stdout, "  subscribes\t%s\n", eventType)
		}
	}
	if len(extension.EventConsumers) > 0 {
		fmt.Fprintln(stdout, "eventConsumers:")
		for _, consumer := range extension.EventConsumers {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\n",
				consumer.Name,
				consumer.Stream,
				coalesce(consumer.ConsumerGroup, ""),
				consumer.ServiceTarget,
			)
			for _, eventType := range consumer.EventTypes {
				fmt.Fprintf(stdout, "    eventType\t%s\n", eventType)
			}
		}
	}
	if len(extension.ScheduledJobs) > 0 {
		fmt.Fprintln(stdout, "scheduledJobs:")
		for _, job := range extension.ScheduledJobs {
			fmt.Fprintf(stdout, "  %s\t%ds\t%s\n", job.Name, job.IntervalSeconds, job.ServiceTarget)
		}
	}
	if extension.RuntimeDiagnostics.BootstrapStatus != "" || len(extension.RuntimeDiagnostics.Endpoints) > 0 || len(extension.RuntimeDiagnostics.EventConsumers) > 0 || len(extension.RuntimeDiagnostics.ScheduledJobs) > 0 {
		fmt.Fprintln(stdout, "runtimeDiagnostics:")
		if extension.RuntimeDiagnostics.BootstrapStatus != "" {
			fmt.Fprintf(stdout, "  bootstrapStatus\t%s\n", extension.RuntimeDiagnostics.BootstrapStatus)
			if extension.RuntimeDiagnostics.LastBootstrapAt != nil {
				fmt.Fprintf(stdout, "  lastBootstrapAt\t%s\n", *extension.RuntimeDiagnostics.LastBootstrapAt)
			}
			if extension.RuntimeDiagnostics.LastBootstrapError != nil {
				fmt.Fprintf(stdout, "  lastBootstrapError\t%s\n", *extension.RuntimeDiagnostics.LastBootstrapError)
			}
		}
		if len(extension.RuntimeDiagnostics.Endpoints) > 0 {
			fmt.Fprintln(stdout, "  endpoints:")
			for _, endpoint := range extension.RuntimeDiagnostics.Endpoints {
				fmt.Fprintf(stdout, "    %s\t%s\t%s\t%s\n",
					endpoint.Name,
					endpoint.Status,
					endpoint.Class,
					endpoint.MountPath,
				)
				if endpoint.LastCheckedAt != nil {
					fmt.Fprintf(stdout, "      lastCheckedAt\t%s\n", *endpoint.LastCheckedAt)
				}
				if endpoint.LastSuccessAt != nil {
					fmt.Fprintf(stdout, "      lastSuccessAt\t%s\n", *endpoint.LastSuccessAt)
				}
				if endpoint.LastFailureAt != nil {
					fmt.Fprintf(stdout, "      lastFailureAt\t%s\n", *endpoint.LastFailureAt)
				}
				if endpoint.ConsecutiveFailures > 0 {
					fmt.Fprintf(stdout, "      consecutiveFailures\t%d\n", endpoint.ConsecutiveFailures)
				}
				if endpoint.LastError != nil {
					fmt.Fprintf(stdout, "      lastError\t%s\n", *endpoint.LastError)
				}
			}
		}
		if len(extension.RuntimeDiagnostics.EventConsumers) > 0 {
			fmt.Fprintln(stdout, "  eventConsumers:")
			for _, consumer := range extension.RuntimeDiagnostics.EventConsumers {
				fmt.Fprintf(stdout, "    %s\t%s\t%s\t%s\n",
					consumer.Name,
					consumer.Status,
					consumer.Stream,
					consumer.ServiceTarget,
				)
				if consumer.LastSuccessAt != nil {
					fmt.Fprintf(stdout, "      lastSuccessAt\t%s\n", *consumer.LastSuccessAt)
				}
				if consumer.LastFailureAt != nil {
					fmt.Fprintf(stdout, "      lastFailureAt\t%s\n", *consumer.LastFailureAt)
				}
				if consumer.ConsecutiveFailures > 0 {
					fmt.Fprintf(stdout, "      consecutiveFailures\t%d\n", consumer.ConsecutiveFailures)
				}
				if consumer.LastError != nil {
					fmt.Fprintf(stdout, "      lastError\t%s\n", *consumer.LastError)
				}
			}
		}
		if len(extension.RuntimeDiagnostics.ScheduledJobs) > 0 {
			fmt.Fprintln(stdout, "  scheduledJobs:")
			for _, job := range extension.RuntimeDiagnostics.ScheduledJobs {
				fmt.Fprintf(stdout, "    %s\t%s\t%ds\t%s\n",
					job.Name,
					job.Status,
					job.IntervalSeconds,
					job.ServiceTarget,
				)
				if job.LastStartedAt != nil {
					fmt.Fprintf(stdout, "      lastStartedAt\t%s\n", *job.LastStartedAt)
				}
				if job.LastSuccessAt != nil {
					fmt.Fprintf(stdout, "      lastSuccessAt\t%s\n", *job.LastSuccessAt)
				}
				if job.LastFailureAt != nil {
					fmt.Fprintf(stdout, "      lastFailureAt\t%s\n", *job.LastFailureAt)
				}
				if job.ConsecutiveFailures > 0 {
					fmt.Fprintf(stdout, "      consecutiveFailures\t%d\n", job.ConsecutiveFailures)
				}
				if job.BackoffUntil != nil {
					fmt.Fprintf(stdout, "      backoffUntil\t%s\n", *job.BackoffUntil)
				}
				if job.LastError != nil {
					fmt.Fprintf(stdout, "      lastError\t%s\n", *job.LastError)
				}
			}
		}
	}
	if len(extension.Commands) > 0 {
		fmt.Fprintln(stdout, "commands:")
		for _, command := range extension.Commands {
			fmt.Fprintf(stdout, "  %s\t%s\n", command.Name, coalesce(command.Description, ""))
		}
	}
	if len(extension.AgentSkills) > 0 {
		fmt.Fprintln(stdout, "agentSkills:")
		for _, skill := range extension.AgentSkills {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\n", skill.Name, coalesce(skill.Description, ""), skill.AssetPath)
		}
	}
	if len(extension.Customizable) > 0 {
		fmt.Fprintln(stdout, "customizableAssets:")
		for _, assetPath := range extension.Customizable {
			fmt.Fprintf(stdout, "  %s\n", assetPath)
		}
	}
	if len(extension.Assets) > 0 {
		fmt.Fprintln(stdout, "assets:")
		for _, asset := range extension.Assets {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\t%d\n", asset.Path, asset.Kind, asset.ContentType, asset.Size)
		}
	}
}

func extensionSeedStateLabel(exists, matchesSeed bool) string {
	switch {
	case !exists:
		return "missing"
	case matchesSeed:
		return "ok"
	default:
		return "drift"
	}
}

func adminActionURL(baseURL, path string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid admin base URL: %w", err)
	}
	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func writeJSON(stdout io.Writer, value any, stderr io.Writer) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode json: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintln(stdout, string(data)); err != nil {
		fmt.Fprintf(stderr, "write output: %v\n", err)
		return 1
	}
	return 0
}

type optionalBoolFlag struct {
	set   bool
	value bool
}

func newOptionalBoolFlag() *optionalBoolFlag {
	return &optionalBoolFlag{}
}

func (f *optionalBoolFlag) String() string {
	if !f.set {
		return ""
	}
	return strconv.FormatBool(f.value)
}

func (f *optionalBoolFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		f.set = true
		f.value = true
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	f.set = true
	f.value = parsed
	return nil
}

func (f *optionalBoolFlag) IsBoolFlag() bool {
	return true
}

func readBundleFile(path string) (bundleFile, error) {
	payload, err := readBundleFilePayload(path)
	if err != nil {
		return bundleFile{}, err
	}
	return payload.Bundle, nil
}

func readBundleFilePayload(path string) (bundleSourcePayload, error) {
	if remoteURL, ok := bundleSourceURL(path); ok {
		return readBundleURLPayloadWithHeaders(context.Background(), remoteURL, nil, bundleSourceKindHTTP)
	}

	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("read bundle file: %w", err)
	}
	if info.IsDir() {
		return readBundleDirectoryPayload(cleanPath)
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("read bundle file: %w", err)
	}
	return decodeBundlePayload(data, bundleSourceKindLocal)
}

//nolint:unused // pending extension install CLI
func readBundleURL(rawURL string) (bundleFile, error) {
	return readBundleURLWithHeaders(context.Background(), rawURL, nil)
}

func bundleSourceURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	if u.Host == "" {
		return "", false
	}
	return u.String(), true
}

//nolint:unused // pending extension install CLI
func readBundleDirectory(root string) (bundleFile, error) {
	payload, err := readBundleDirectoryPayload(root)
	if err != nil {
		return bundleFile{}, err
	}
	return payload.Bundle, nil
}

//nolint:unused // pending extension install CLI
func readBundleDirectoryPayload(root string) (bundleSourcePayload, error) {
	manifestPath := filepath.Join(root, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("read bundle manifest: %w", err)
	}
	manifest, err := parseJSONObject(manifestBytes)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("decode bundle manifest: %w", err)
	}

	assetsRoot := filepath.Join(root, "assets")
	assets := []bundleAssetInput{}
	if info, err := os.Stat(assetsRoot); err == nil && info.IsDir() {
		if err := filepath.WalkDir(assetsRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(assetsRoot, path)
			if err != nil {
				return err
			}
			assets = append(assets, bundleAssetInput{
				Path:        filepath.ToSlash(relative),
				Content:     string(content),
				ContentType: detectAssetContentType(path, content),
			})
			return nil
		}); err != nil {
			return bundleSourcePayload{}, fmt.Errorf("walk bundle assets: %w", err)
		}
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	migrationsRoot := filepath.Join(root, "migrations")
	migrations := []bundleMigrationInput{}
	if info, err := os.Stat(migrationsRoot); err == nil && info.IsDir() {
		if err := filepath.WalkDir(migrationsRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(migrationsRoot, path)
			if err != nil {
				return err
			}
			migrations = append(migrations, bundleMigrationInput{
				Path:    filepath.ToSlash(relative),
				Content: string(content),
			})
			return nil
		}); err != nil {
			return bundleSourcePayload{}, fmt.Errorf("walk bundle migrations: %w", err)
		}
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Path < migrations[j].Path
	})

	bundle := bundleFile{
		Manifest:   manifest,
		Assets:     assets,
		Migrations: migrations,
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("encode bundle directory payload: %w", err)
	}
	return bundleSourcePayload{
		Kind:   bundleSourceKindLocal,
		Bundle: bundle,
		Bytes:  encoded,
	}, nil
}

func detectAssetContentType(path string, content []byte) string {
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return byExt
		}
	}
	return http.DetectContentType(content)
}

func readConfigInput(configPath, configJSON string) (map[string]any, error) {
	switch {
	case strings.TrimSpace(configPath) != "" && strings.TrimSpace(configJSON) != "":
		return nil, fmt.Errorf("pass either --config-file or --config-json, not both")
	case strings.TrimSpace(configPath) != "":
		data, err := os.ReadFile(filepath.Clean(configPath))
		if err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
		return parseJSONObject(data)
	case strings.TrimSpace(configJSON) != "":
		return parseJSONObject([]byte(configJSON))
	default:
		return nil, fmt.Errorf("one of --config-file or --config-json is required")
	}
}

func readOptionalTextInput(path, inline, fieldName string) (*string, error) {
	switch {
	case strings.TrimSpace(path) != "" && inline != "":
		return nil, fmt.Errorf("pass either --file or --%s, not both", fieldName)
	case strings.TrimSpace(path) != "":
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("read %s file: %w", fieldName, err)
		}
		value := string(data)
		return &value, nil
	case inline != "":
		value := inline
		return &value, nil
	default:
		return nil, nil
	}
}

func readOptionalJSONObjectInput(path, inline, fieldName string) (map[string]any, error) {
	switch {
	case strings.TrimSpace(path) != "" && strings.TrimSpace(inline) != "":
		return nil, fmt.Errorf("pass either --%s-file or --%s-json, not both", fieldName, fieldName)
	case strings.TrimSpace(path) != "":
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("read %s file: %w", fieldName, err)
		}
		return parseJSONObject(data)
	case strings.TrimSpace(inline) != "":
		return parseJSONObject([]byte(inline))
	default:
		return nil, nil
	}
}

func readConceptSpecInputFile(path string) (conceptSpecMutationInput, error) {
	type conceptSpecFile struct {
		Key                   string         `yaml:"key"`
		Version               string         `yaml:"version"`
		Name                  string         `yaml:"name"`
		Description           string         `yaml:"description"`
		ExtendsKey            string         `yaml:"extends_key"`
		ExtendsVersion        string         `yaml:"extends_version"`
		InstanceKind          string         `yaml:"instance_kind"`
		MetadataSchema        map[string]any `yaml:"metadata_schema"`
		SectionsSchema        map[string]any `yaml:"sections_schema"`
		WorkflowSchema        map[string]any `yaml:"workflow_schema"`
		AgentGuidanceMarkdown string         `yaml:"agent_guidance_markdown"`
		SourceKind            string         `yaml:"source_kind"`
		SourceRef             string         `yaml:"source_ref"`
		Status                string         `yaml:"status"`
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return conceptSpecMutationInput{}, fmt.Errorf("read concept spec file: %w", err)
	}
	var payload conceptSpecFile
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return conceptSpecMutationInput{}, fmt.Errorf("decode concept spec yaml: %w", err)
	}
	if strings.TrimSpace(payload.Key) == "" {
		return conceptSpecMutationInput{}, fmt.Errorf("concept spec key is required")
	}
	if strings.TrimSpace(payload.Name) == "" {
		return conceptSpecMutationInput{}, fmt.Errorf("concept spec name is required")
	}
	if strings.TrimSpace(payload.InstanceKind) == "" {
		return conceptSpecMutationInput{}, fmt.Errorf("concept spec instance_kind is required")
	}
	return conceptSpecMutationInput{
		Key:                   strings.TrimSpace(payload.Key),
		Version:               strings.TrimSpace(payload.Version),
		Name:                  strings.TrimSpace(payload.Name),
		Description:           strings.TrimSpace(payload.Description),
		ExtendsKey:            strings.TrimSpace(payload.ExtendsKey),
		ExtendsVersion:        strings.TrimSpace(payload.ExtendsVersion),
		InstanceKind:          strings.TrimSpace(payload.InstanceKind),
		MetadataSchema:        normalizeJSONObjectMap(payload.MetadataSchema),
		SectionsSchema:        normalizeJSONObjectMap(payload.SectionsSchema),
		WorkflowSchema:        normalizeJSONObjectMap(payload.WorkflowSchema),
		AgentGuidanceMarkdown: strings.TrimSpace(payload.AgentGuidanceMarkdown),
		SourceKind:            strings.TrimSpace(payload.SourceKind),
		SourceRef:             strings.TrimSpace(payload.SourceRef),
		Status:                strings.TrimSpace(payload.Status),
	}, nil
}

func normalizeJSONObjectMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return value
	}
	normalized := map[string]any{}
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return value
	}
	return normalized
}

func readFormDefinitionInput(definitionPath, definitionJSON, formName string) (map[string]any, error) {
	switch {
	case strings.TrimSpace(definitionPath) != "" || strings.TrimSpace(definitionJSON) != "":
		return readConfigInput(definitionPath, definitionJSON)
	default:
		return defaultFormDefinition(formName), nil
	}
}

func defaultFormDefinition(formName string) map[string]any {
	title := strings.TrimSpace(formName)
	if title == "" {
		title = "Contact Form"
	}
	return map[string]any{
		"title": title,
		"fields": []map[string]any{
			{"name": "name", "type": "text", "required": true, "label": "Your Name"},
			{"name": "email", "type": "email", "required": true, "label": "Email Address"},
			{"name": "subject", "type": "text", "required": true, "label": "Subject"},
			{"name": "message", "type": "textarea", "required": true, "label": "Message"},
		},
	}
}

func readJSONValueInput(path, inline, fieldName string) (any, error) {
	switch {
	case strings.TrimSpace(path) != "" && strings.TrimSpace(inline) != "":
		return nil, fmt.Errorf("pass either --%s-file or --%s-json, not both", fieldName, fieldName)
	case strings.TrimSpace(path) != "":
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("read %s file: %w", fieldName, err)
		}
		return parseJSONValue(data, fieldName)
	case strings.TrimSpace(inline) != "":
		return parseJSONValue([]byte(inline), fieldName)
	default:
		return nil, fmt.Errorf("one of --%s-file or --%s-json is required", fieldName, fieldName)
	}
}

func readJSONArrayInput(path, inline, fieldName string) ([]any, error) {
	value, err := readJSONValueInput(path, inline, fieldName)
	if err != nil {
		return nil, err
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON array", fieldName)
	}
	return items, nil
}

func parseJSONValue(data []byte, fieldName string) (any, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode %s json: %w", fieldName, err)
	}
	if value == nil {
		return nil, fmt.Errorf("%s must not be null", fieldName)
	}
	return value, nil
}

func parseJSONObject(data []byte) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode json object: %w", err)
	}
	if value == nil {
		return nil, fmt.Errorf("config must be a JSON object")
	}
	return value, nil
}

func readKnowledgeSyncDocuments(root string) ([]knowledgeSyncDocument, error) {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	info, err := os.Stat(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("stat markdown path: %w", err)
	}

	documents := make([]knowledgeSyncDocument, 0)
	if !info.IsDir() {
		document, err := parseKnowledgeSyncDocument(cleanRoot, filepath.Base(cleanRoot))
		if err != nil {
			return nil, err
		}
		return []knowledgeSyncDocument{document}, nil
	}

	if err := filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(d.Name())) {
		case ".md", ".markdown":
		default:
			return nil
		}

		relativePath, err := filepath.Rel(cleanRoot, path)
		if err != nil {
			return err
		}
		document, err := parseKnowledgeSyncDocument(path, relativePath)
		if err != nil {
			return err
		}
		documents = append(documents, document)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk markdown path: %w", err)
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("no markdown files found in %s", cleanRoot)
	}
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].RelativePath < documents[j].RelativePath
	})
	return documents, nil
}

func parseKnowledgeSyncDocument(path, relativePath string) (knowledgeSyncDocument, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return knowledgeSyncDocument{}, fmt.Errorf("read markdown file %s: %w", path, err)
	}

	frontmatterRaw, body := splitMarkdownFrontmatter(string(data))
	rawFrontmatter := map[string]any{}
	if strings.TrimSpace(frontmatterRaw) != "" {
		if err := yaml.Unmarshal([]byte(frontmatterRaw), &rawFrontmatter); err != nil {
			return knowledgeSyncDocument{}, fmt.Errorf("decode markdown frontmatter for %s: %w", path, err)
		}
	}

	customFrontmatter, err := extractKnowledgeCustomFrontmatter(rawFrontmatter)
	if err != nil {
		return knowledgeSyncDocument{}, fmt.Errorf("parse markdown frontmatter for %s: %w", path, err)
	}
	relativeSlash := filepath.ToSlash(relativePath)
	document := knowledgeSyncDocument{
		AbsolutePath:       path,
		RelativePath:       relativeSlash,
		Slug:               firstNonEmpty(frontmatterString(rawFrontmatter, "slug")),
		Title:              firstNonEmpty(frontmatterString(rawFrontmatter, "title")),
		TeamID:             firstNonEmpty(frontmatterString(rawFrontmatter, "team_id"), frontmatterString(rawFrontmatter, "team")),
		Surface:            firstNonEmpty(frontmatterString(rawFrontmatter, "surface")),
		Kind:               firstNonEmpty(frontmatterString(rawFrontmatter, "kind"), frontmatterString(rawFrontmatter, "type"), frontmatterString(rawFrontmatter, "doc_type"), frontmatterString(rawFrontmatter, "knowledge_kind")),
		ConceptSpecKey:     firstNonEmpty(frontmatterString(rawFrontmatter, "concept_spec"), frontmatterString(rawFrontmatter, "concept_spec_key")),
		ConceptSpecVersion: firstNonEmpty(frontmatterString(rawFrontmatter, "concept_spec_version")),
		Status:             firstNonEmpty(frontmatterString(rawFrontmatter, "status")),
		ReviewStatus:       firstNonEmpty(frontmatterString(rawFrontmatter, "review_status")),
		Summary:            firstNonEmpty(frontmatterString(rawFrontmatter, "summary")),
		SharedWithTeamIDs:  frontmatterStrings(rawFrontmatter, "shared_with_team_ids", "share_with"),
		SupportedChannels:  frontmatterStrings(rawFrontmatter, "supported_channels", "channels"),
		SearchKeywords:     frontmatterStrings(rawFrontmatter, "search_keywords", "keywords"),
		SourceKind:         firstNonEmpty(frontmatterString(rawFrontmatter, "source_kind")),
		SourceRef:          firstNonEmpty(frontmatterString(rawFrontmatter, "source_ref")),
		PathRef:            firstNonEmpty(frontmatterString(rawFrontmatter, "path_ref"), relativeSlash),
		BodyMarkdown:       strings.TrimSpace(body),
		Frontmatter:        customFrontmatter,
	}
	return document, nil
}

func splitMarkdownFrontmatter(content string) (string, string) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", normalized
	}
	rest := strings.TrimPrefix(normalized, "---\n")
	index := strings.Index(rest, "\n---\n")
	if index < 0 {
		return "", normalized
	}
	return rest[:index], rest[index+len("\n---\n"):]
}

func extractKnowledgeCustomFrontmatter(raw map[string]any) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	custom := map[string]any{}
	if value, ok := raw["custom"]; ok {
		switch typed := value.(type) {
		case map[string]any:
			for key, item := range typed {
				custom[key] = item
			}
		case nil:
		default:
			return nil, fmt.Errorf("custom frontmatter must be an object")
		}
	}

	knownKeys := map[string]struct{}{
		"title": {}, "slug": {}, "team_id": {}, "team": {}, "surface": {}, "kind": {}, "review_status": {},
		"concept_spec": {}, "concept_spec_key": {}, "concept_spec_version": {},
		"status": {}, "summary": {}, "shared_with_team_ids": {}, "share_with": {}, "supported_channels": {},
		"channels": {}, "search_keywords": {}, "keywords": {}, "source_kind": {}, "source_ref": {},
		"path_ref": {}, "trust_level": {}, "published_at": {}, "published_by": {}, "custom": {},
	}
	for key, value := range raw {
		if _, ok := knownKeys[key]; ok {
			continue
		}
		custom[key] = value
	}
	if len(custom) == 0 {
		return nil, nil
	}
	return custom, nil
}

func frontmatterString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func frontmatterStrings(raw map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []any:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				items = append(items, fmt.Sprint(item))
			}
			return uniqueStrings(items)
		case []string:
			return uniqueStrings(typed)
		case string:
			return commaSeparatedValues(typed)
		default:
			return uniqueStrings([]string{fmt.Sprint(typed)})
		}
	}
	return nil
}

func normalizeKnowledgeKindValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	kind, ok := knowledgedomain.ParseKnowledgeResourceKind(trimmed)
	if !ok {
		validKinds := knowledgedomain.KnowledgeResourceKinds()
		valid := make([]string, 0, len(validKinds))
		for _, item := range validKinds {
			valid = append(valid, string(item))
		}
		return "", fmt.Errorf("invalid knowledge kind %q; expected one of %s", trimmed, strings.Join(valid, ", "))
	}
	return string(kind), nil
}

func inferKnowledgeKindFromPath(relativePath, title string) knowledgedomain.KnowledgeResourceKind {
	normalizedPath := "/" + strings.Trim(strings.ToLower(filepath.ToSlash(relativePath)), "/") + "/"
	for _, hint := range knowledgeKindPathHints {
		if strings.Contains(normalizedPath, hint.match) {
			return hint.kind
		}
	}

	name := strings.ToLower(strings.TrimSpace(title))
	switch {
	case strings.HasPrefix(name, "adr "), strings.HasPrefix(name, "adr:"), strings.HasPrefix(name, "decision "):
		return knowledgedomain.KnowledgeResourceKindDecision
	case strings.Contains(name, "checklist"):
		return knowledgedomain.KnowledgeResourceKindChecklist
	case strings.Contains(name, "template"):
		return knowledgedomain.KnowledgeResourceKindTemplate
	case strings.Contains(name, "best practice"), strings.Contains(name, "best-practice"):
		return knowledgedomain.KnowledgeResourceKindBestPractice
	case strings.Contains(name, "constraint"), strings.Contains(name, "guardrail"), strings.Contains(name, "requirement"):
		return knowledgedomain.KnowledgeResourceKindConstraint
	case strings.Contains(name, "idea"), strings.Contains(name, "proposal"), strings.Contains(name, "brainstorm"):
		return knowledgedomain.KnowledgeResourceKindIdea
	}

	return knowledgedomain.KnowledgeResourceKindGuide
}

func resolveKnowledgeSyncKind(document knowledgeSyncDocument, defaultKind string) (string, error) {
	if normalized, err := normalizeKnowledgeKindValue(firstNonEmpty(document.Kind, defaultKind)); err != nil {
		return "", err
	} else if normalized != "" {
		return normalized, nil
	}

	title := firstNonEmpty(document.Title, inferKnowledgeTitle(document.BodyMarkdown, frontmatterOrFallbackSlug(document), document.RelativePath))
	return string(inferKnowledgeKindFromPath(document.RelativePath, title)), nil
}

func inferKnowledgeConceptSpec(relativePath string, kind string) (string, string) {
	normalizedPath := "/" + strings.Trim(strings.ToLower(filepath.ToSlash(relativePath)), "/") + "/"
	switch {
	case strings.Contains(normalizedPath, "/adr/"), strings.Contains(normalizedPath, "/adrs/"):
		return "core/adr", "1"
	case strings.Contains(normalizedPath, "/rfc/"), strings.Contains(normalizedPath, "/rfcs/"):
		return "core/rfc", "1"
	}
	return knowledgedomain.DefaultConceptSpecForKind(knowledgedomain.KnowledgeResourceKind(kind))
}

func planKnowledgeImport(defaults knowledgeSyncDefaults, document knowledgeSyncDocument) (knowledgeImportPlan, error) {
	teamID := firstNonEmpty(document.TeamID, defaults.TeamID)
	if strings.TrimSpace(teamID) == "" {
		return knowledgeImportPlan{}, fmt.Errorf("owner team is required for %s", document.RelativePath)
	}

	slug := frontmatterOrFallbackSlug(document)
	title := firstNonEmpty(document.Title, inferKnowledgeTitle(document.BodyMarkdown, slug, document.RelativePath))
	desiredSurface := strings.ToLower(firstNonEmpty(document.Surface, defaults.Surface, "private"))
	desiredKind, err := resolveKnowledgeSyncKind(document, defaults.Kind)
	if err != nil {
		return knowledgeImportPlan{}, err
	}
	desiredConceptSpec := firstNonEmpty(document.ConceptSpecKey, defaults.ConceptSpecKey)
	desiredConceptVersion := firstNonEmpty(document.ConceptSpecVersion, defaults.ConceptSpecVersion)
	if strings.TrimSpace(desiredConceptSpec) == "" {
		desiredConceptSpec, desiredConceptVersion = inferKnowledgeConceptSpec(document.RelativePath, desiredKind)
	}
	desiredStatus := strings.ToLower(firstNonEmpty(document.Status, defaults.Status))
	desiredReviewStatus := strings.ToLower(firstNonEmpty(document.ReviewStatus, defaults.ReviewStatus))
	desiredSourceKind := strings.ToLower(firstNonEmpty(document.SourceKind, defaults.SourceKind, "imported"))
	desiredSourceRef := firstNonEmpty(document.SourceRef, defaults.SourceRef, document.RelativePath)
	sharedWithTeamIDs := document.SharedWithTeamIDs
	if len(sharedWithTeamIDs) == 0 {
		sharedWithTeamIDs = defaults.SharedWithTeamIDs
	}
	pathRef := firstNonEmpty(document.PathRef, document.RelativePath)

	return knowledgeImportPlan{
		Path:               document.AbsolutePath,
		RelativePath:       document.RelativePath,
		WorkspaceID:        defaults.WorkspaceID,
		TeamID:             teamID,
		Surface:            desiredSurface,
		Kind:               desiredKind,
		ConceptSpecKey:     desiredConceptSpec,
		ConceptSpecVersion: desiredConceptVersion,
		Status:             desiredStatus,
		ReviewStatus:       desiredReviewStatus,
		Slug:               slug,
		Title:              title,
		Summary:            document.Summary,
		SourceKind:         desiredSourceKind,
		SourceRef:          desiredSourceRef,
		PathRef:            pathRef,
		SharedWithTeamIDs:  sharedWithTeamIDs,
		SupportedChannels:  document.SupportedChannels,
		SearchKeywords:     document.SearchKeywords,
	}, nil
}

func syncKnowledgeDocument(ctx context.Context, client *cliapi.Client, defaults knowledgeSyncDefaults, document knowledgeSyncDocument) (knowledgeSyncResult, error) {
	plan, err := planKnowledgeImport(defaults, document)
	if err != nil {
		return knowledgeSyncResult{}, err
	}

	existing, err := runKnowledgeShowBySlugAnySurface(ctx, client, defaults.WorkspaceID, plan.TeamID, plan.Slug, plan.Surface)
	if err != nil {
		return knowledgeSyncResult{}, err
	}

	actions := make([]string, 0, 4)
	bodyValue := document.BodyMarkdown

	var resource knowledgeResourceOutput
	if existing == nil {
		resource, err = runKnowledgeCreate(ctx, client, knowledgeMutationInput{
			WorkspaceID:        defaults.WorkspaceID,
			TeamID:             plan.TeamID,
			Slug:               plan.Slug,
			Title:              plan.Title,
			Kind:               plan.Kind,
			ConceptSpecKey:     plan.ConceptSpecKey,
			ConceptSpecVersion: plan.ConceptSpecVersion,
			Status:             plan.Status,
			Summary:            plan.Summary,
			BodyMarkdown:       &bodyValue,
			SourceKind:         plan.SourceKind,
			SourceRef:          plan.SourceRef,
			PathRef:            plan.PathRef,
			SupportedChannels:  plan.SupportedChannels,
			SharedWithTeamIDs:  plan.SharedWithTeamIDs,
			SearchKeywords:     plan.SearchKeywords,
			Surface:            plan.Surface,
			Frontmatter:        document.Frontmatter,
		})
		if err != nil {
			return knowledgeSyncResult{}, err
		}
		actions = append(actions, "created")
	} else {
		resource, err = runKnowledgeUpdate(ctx, client, existing.ID, knowledgeMutationInput{
			Slug:               plan.Slug,
			Title:              plan.Title,
			Kind:               plan.Kind,
			ConceptSpecKey:     plan.ConceptSpecKey,
			ConceptSpecVersion: plan.ConceptSpecVersion,
			Status:             plan.Status,
			Summary:            plan.Summary,
			BodyMarkdown:       &bodyValue,
			SourceKind:         plan.SourceKind,
			SourceRef:          plan.SourceRef,
			PathRef:            plan.PathRef,
			SupportedChannels:  plan.SupportedChannels,
			SearchKeywords:     plan.SearchKeywords,
			Frontmatter:        document.Frontmatter,
		})
		if err != nil {
			return knowledgeSyncResult{}, err
		}
		actions = append(actions, "updated")
	}

	if plan.ReviewStatus != "" && !strings.EqualFold(resource.ReviewStatus, plan.ReviewStatus) {
		resource, err = runKnowledgeReview(ctx, client, resource.ID, plan.ReviewStatus)
		if err != nil {
			return knowledgeSyncResult{}, err
		}
		actions = append(actions, "reviewed")
	}

	if plan.Surface != "" && !strings.EqualFold(resource.Surface, plan.Surface) {
		if strings.EqualFold(plan.Surface, "private") {
			return knowledgeSyncResult{}, fmt.Errorf("cannot sync %s to private because the existing resource already lives on %s", document.RelativePath, resource.Surface)
		}
		resource, err = runKnowledgePublish(ctx, client, resource.ID, plan.Surface)
		if err != nil {
			return knowledgeSyncResult{}, err
		}
		actions = append(actions, "published")
	}

	if len(plan.SharedWithTeamIDs) > 0 && !sameStringSet(resource.SharedWithTeamIDs, plan.SharedWithTeamIDs) {
		resource, err = runKnowledgeShare(ctx, client, resource.ID, plan.SharedWithTeamIDs)
		if err != nil {
			return knowledgeSyncResult{}, err
		}
		actions = append(actions, "shared")
	}

	return knowledgeSyncResult{
		Path:         document.AbsolutePath,
		RelativePath: document.RelativePath,
		Action:       strings.Join(actions, "+"),
		ID:           resource.ID,
		TeamID:       resource.OwnerTeamID,
		Surface:      resource.Surface,
		Slug:         resource.Slug,
		RevisionRef:  resource.RevisionRef,
	}, nil
}

func runKnowledgeShowBySlugAnySurface(ctx context.Context, client *cliapi.Client, workspaceID, teamID, slug, preferredSurface string) (*knowledgeResourceOutput, error) {
	surfaces := make([]string, 0, 3)
	appendSurface := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		for _, existing := range surfaces {
			if existing == value {
				return
			}
		}
		surfaces = append(surfaces, value)
	}

	appendSurface(preferredSurface)
	appendSurface("private")
	appendSurface("published")
	appendSurface("workspace_shared")

	for _, surface := range surfaces {
		resource, err := runKnowledgeShowBySlug(ctx, client, workspaceID, teamID, surface, slug)
		if err == nil {
			if resource == nil {
				continue
			}
			return resource, nil
		}
		if isKnowledgeNotFoundError(err) {
			continue
		}
		return nil, err
	}
	return nil, nil
}

func isKnowledgeNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "knowledge resource not found")
}

func frontmatterOrFallbackSlug(document knowledgeSyncDocument) string {
	return normalizeKnowledgeSyncSlug(document.Slug, document.RelativePath)
}

func normalizeKnowledgeSyncSlug(slug, relativePath string) string {
	name := strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath))
	return knowledgedomain.NormalizeKnowledgeSlug(firstNonEmpty(slug, name), "")
}

func inferKnowledgeTitle(bodyMarkdown, slug, relativePath string) string {
	for _, line := range strings.Split(bodyMarkdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}

	source := slug
	if strings.TrimSpace(source) == "" {
		source = strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath))
	}
	source = strings.ReplaceAll(source, "-", " ")
	source = strings.ReplaceAll(source, "_", " ")
	parts := strings.Fields(source)
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	if len(parts) == 0 {
		return "Knowledge"
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func sameStringSet(left, right []string) bool {
	left = uniqueStrings(left)
	right = uniqueStrings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func splitSinglePositionalArgs(args []string, flagsWithValues map[string]bool) ([]string, []string) {
	flagArgs := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if flagsWithValues[arg] && i+1 < len(args) {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, arg)
	}

	return flagArgs, positionals
}

func requireSessionAuth(cfg cliapi.Config, feature string) error {
	if cfg.AuthMode != cliapi.AuthModeSession {
		return fmt.Errorf("%s commands require browser login or MBR_SESSION_TOKEN", feature)
	}
	return nil
}

func coalesce(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func commaSeparatedValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}
