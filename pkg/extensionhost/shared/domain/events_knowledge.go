package shareddomain

import (
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// KnowledgeCreated is published when a new knowledge resource is created.
type KnowledgeCreated struct {
	eventbus.BaseEvent

	KnowledgeResourceID string
	WorkspaceID         string
	OwnerTeamID         string
	Slug                string
	Title               string
	Kind                string
	Surface             string
	ReviewStatus        string
	CreatedBy           string
	CreatedAt           time.Time
}

func NewKnowledgeCreatedEvent(resourceID, workspaceID, ownerTeamID, slug, title, kind, surface, reviewStatus, createdBy string) KnowledgeCreated {
	return KnowledgeCreated{
		BaseEvent:           eventbus.NewBaseEvent(eventbus.TypeKnowledgeCreated),
		KnowledgeResourceID: resourceID,
		WorkspaceID:         workspaceID,
		OwnerTeamID:         ownerTeamID,
		Slug:                slug,
		Title:               title,
		Kind:                kind,
		Surface:             surface,
		ReviewStatus:        reviewStatus,
		CreatedBy:           createdBy,
		CreatedAt:           time.Now().UTC(),
	}
}

// KnowledgeReviewRequested is published when a team-owned knowledge item,
// such as an RFC, should be reviewed by teammates or their agents.
type KnowledgeReviewRequested struct {
	eventbus.BaseEvent

	KnowledgeResourceID string
	WorkspaceID         string
	OwnerTeamID         string
	Slug                string
	Title               string
	Kind                string
	Surface             string
	ReviewStatus        string
	RequestedBy         string
	RequestedAt         time.Time
}

func NewKnowledgeReviewRequestedEvent(resourceID, workspaceID, ownerTeamID, slug, title, kind, surface, reviewStatus, requestedBy string) KnowledgeReviewRequested {
	return KnowledgeReviewRequested{
		BaseEvent:           eventbus.NewBaseEvent(eventbus.TypeKnowledgeReviewRequested),
		KnowledgeResourceID: resourceID,
		WorkspaceID:         workspaceID,
		OwnerTeamID:         ownerTeamID,
		Slug:                slug,
		Title:               title,
		Kind:                kind,
		Surface:             surface,
		ReviewStatus:        reviewStatus,
		RequestedBy:         requestedBy,
		RequestedAt:         time.Now().UTC(),
	}
}

func (e KnowledgeCreated) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("knowledge_resource_id", e.KnowledgeResourceID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("owner_team_id", e.OwnerTeamID); err != nil {
		return err
	}
	if err := validateNonEmpty("slug", e.Slug); err != nil {
		return err
	}
	if err := validateNonEmpty("title", e.Title); err != nil {
		return err
	}
	if err := validateNonEmpty("kind", e.Kind); err != nil {
		return err
	}
	if e.CreatedAt.IsZero() {
		return ErrRequiredField("created_at")
	}
	return nil
}

func (e KnowledgeReviewRequested) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("knowledge_resource_id", e.KnowledgeResourceID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("owner_team_id", e.OwnerTeamID); err != nil {
		return err
	}
	if err := validateNonEmpty("slug", e.Slug); err != nil {
		return err
	}
	if err := validateNonEmpty("title", e.Title); err != nil {
		return err
	}
	if err := validateNonEmpty("kind", e.Kind); err != nil {
		return err
	}
	if e.RequestedAt.IsZero() {
		return ErrRequiredField("requested_at")
	}
	return nil
}
