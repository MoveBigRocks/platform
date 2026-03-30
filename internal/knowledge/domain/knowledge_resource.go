package knowledgedomain

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

type KnowledgeResourceKind string

const (
	KnowledgeResourceKindPolicy       KnowledgeResourceKind = "policy"
	KnowledgeResourceKindGuide        KnowledgeResourceKind = "guide"
	KnowledgeResourceKindSkill        KnowledgeResourceKind = "skill"
	KnowledgeResourceKindContext      KnowledgeResourceKind = "context"
	KnowledgeResourceKindConstraint   KnowledgeResourceKind = "constraint"
	KnowledgeResourceKindBestPractice KnowledgeResourceKind = "best_practice"
	KnowledgeResourceKindTemplate     KnowledgeResourceKind = "template"
	KnowledgeResourceKindChecklist    KnowledgeResourceKind = "checklist"
	KnowledgeResourceKindDecision     KnowledgeResourceKind = "decision"
	KnowledgeResourceKindIdea         KnowledgeResourceKind = "idea"
)

var knowledgeResourceKindAliases = map[string]KnowledgeResourceKind{
	"policy":         KnowledgeResourceKindPolicy,
	"policies":       KnowledgeResourceKindPolicy,
	"guide":          KnowledgeResourceKindGuide,
	"guides":         KnowledgeResourceKindGuide,
	"playbook":       KnowledgeResourceKindGuide,
	"playbooks":      KnowledgeResourceKindGuide,
	"runbook":        KnowledgeResourceKindGuide,
	"runbooks":       KnowledgeResourceKindGuide,
	"howto":          KnowledgeResourceKindGuide,
	"how_to":         KnowledgeResourceKindGuide,
	"how-to":         KnowledgeResourceKindGuide,
	"procedure":      KnowledgeResourceKindGuide,
	"procedures":     KnowledgeResourceKindGuide,
	"skill":          KnowledgeResourceKindSkill,
	"skills":         KnowledgeResourceKindSkill,
	"prompt":         KnowledgeResourceKindSkill,
	"prompts":        KnowledgeResourceKindSkill,
	"context":        KnowledgeResourceKindContext,
	"contexts":       KnowledgeResourceKindContext,
	"brief":          KnowledgeResourceKindContext,
	"briefs":         KnowledgeResourceKindContext,
	"reference":      KnowledgeResourceKindContext,
	"references":     KnowledgeResourceKindContext,
	"constraint":     KnowledgeResourceKindConstraint,
	"constraints":    KnowledgeResourceKindConstraint,
	"guardrail":      KnowledgeResourceKindConstraint,
	"guardrails":     KnowledgeResourceKindConstraint,
	"requirement":    KnowledgeResourceKindConstraint,
	"requirements":   KnowledgeResourceKindConstraint,
	"bestpractice":   KnowledgeResourceKindBestPractice,
	"best_practice":  KnowledgeResourceKindBestPractice,
	"best-practice":  KnowledgeResourceKindBestPractice,
	"bestpractices":  KnowledgeResourceKindBestPractice,
	"best_practices": KnowledgeResourceKindBestPractice,
	"best-practices": KnowledgeResourceKindBestPractice,
	"standard":       KnowledgeResourceKindBestPractice,
	"standards":      KnowledgeResourceKindBestPractice,
	"convention":     KnowledgeResourceKindBestPractice,
	"conventions":    KnowledgeResourceKindBestPractice,
	"template":       KnowledgeResourceKindTemplate,
	"templates":      KnowledgeResourceKindTemplate,
	"snippet":        KnowledgeResourceKindTemplate,
	"snippets":       KnowledgeResourceKindTemplate,
	"boilerplate":    KnowledgeResourceKindTemplate,
	"checklist":      KnowledgeResourceKindChecklist,
	"checklists":     KnowledgeResourceKindChecklist,
	"todo":           KnowledgeResourceKindChecklist,
	"decision":       KnowledgeResourceKindDecision,
	"decisions":      KnowledgeResourceKindDecision,
	"rfc":            KnowledgeResourceKindDecision,
	"rfcs":           KnowledgeResourceKindDecision,
	"adr":            KnowledgeResourceKindDecision,
	"adrs":           KnowledgeResourceKindDecision,
	"idea":           KnowledgeResourceKindIdea,
	"ideas":          KnowledgeResourceKindIdea,
	"proposal":       KnowledgeResourceKindIdea,
	"proposals":      KnowledgeResourceKindIdea,
	"brainstorm":     KnowledgeResourceKindIdea,
	"brainstorms":    KnowledgeResourceKindIdea,
}

var knowledgeResourceKinds = []KnowledgeResourceKind{
	KnowledgeResourceKindPolicy,
	KnowledgeResourceKindGuide,
	KnowledgeResourceKindSkill,
	KnowledgeResourceKindContext,
	KnowledgeResourceKindConstraint,
	KnowledgeResourceKindBestPractice,
	KnowledgeResourceKindTemplate,
	KnowledgeResourceKindChecklist,
	KnowledgeResourceKindDecision,
	KnowledgeResourceKindIdea,
}

type KnowledgeResourceSourceKind string

const (
	KnowledgeResourceSourceKindWorkspace KnowledgeResourceSourceKind = "workspace"
	KnowledgeResourceSourceKindExtension KnowledgeResourceSourceKind = "extension"
	KnowledgeResourceSourceKindImported  KnowledgeResourceSourceKind = "imported"
)

type KnowledgeResourceTrustLevel string

const (
	KnowledgeResourceTrustLevelWorkspace KnowledgeResourceTrustLevel = "workspace"
	KnowledgeResourceTrustLevelReviewed  KnowledgeResourceTrustLevel = "reviewed"
	KnowledgeResourceTrustLevelSystem    KnowledgeResourceTrustLevel = "system"
)

type KnowledgeResourceStatus string

const (
	KnowledgeResourceStatusDraft    KnowledgeResourceStatus = "draft"
	KnowledgeResourceStatusActive   KnowledgeResourceStatus = "active"
	KnowledgeResourceStatusArchived KnowledgeResourceStatus = "archived"
)

type KnowledgeSurface string

const (
	KnowledgeSurfacePrivate       KnowledgeSurface = "private"
	KnowledgeSurfacePublished     KnowledgeSurface = "published"
	KnowledgeSurfaceWorkspaceWide KnowledgeSurface = "workspace_shared"
)

type KnowledgeReviewStatus string

const (
	KnowledgeReviewStatusDraft    KnowledgeReviewStatus = "draft"
	KnowledgeReviewStatusReviewed KnowledgeReviewStatus = "reviewed"
	KnowledgeReviewStatusApproved KnowledgeReviewStatus = "approved"
)

type KnowledgeResource struct {
	ID          string
	WorkspaceID string
	OwnerTeamID string

	Slug               string
	Title              string
	Kind               KnowledgeResourceKind
	ConceptSpecKey     string
	ConceptSpecVersion string
	SourceKind         KnowledgeResourceSourceKind
	SourceRef          string
	PathRef            string
	Summary            string
	BodyMarkdown       string
	Frontmatter        shareddomain.TypedSchema

	SupportedChannels []string
	SharedWithTeamIDs []string
	Surface           KnowledgeSurface
	TrustLevel        KnowledgeResourceTrustLevel
	SearchKeywords    []string
	Status            KnowledgeResourceStatus
	ReviewStatus      KnowledgeReviewStatus
	ContentHash       string
	ReviewedAt        *time.Time
	ArtifactPath      string
	RevisionRef       string
	PublishedRevision string
	PublishedAt       *time.Time
	PublishedBy       string

	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewKnowledgeResource(workspaceID, ownerTeamID, slug, title string) *KnowledgeResource {
	now := time.Now().UTC()
	return &KnowledgeResource{
		WorkspaceID:        workspaceID,
		OwnerTeamID:        strings.TrimSpace(ownerTeamID),
		Slug:               NormalizeKnowledgeSlug(slug, title),
		Title:              strings.TrimSpace(title),
		Kind:               KnowledgeResourceKindGuide,
		ConceptSpecKey:     "core/guide",
		ConceptSpecVersion: "1",
		SourceKind:         KnowledgeResourceSourceKindWorkspace,
		Frontmatter:        shareddomain.NewTypedSchema(),
		SupportedChannels:  []string{},
		SharedWithTeamIDs:  []string{},
		Surface:            KnowledgeSurfacePrivate,
		TrustLevel:         KnowledgeResourceTrustLevelWorkspace,
		SearchKeywords:     []string{},
		Status:             KnowledgeResourceStatusDraft,
		ReviewStatus:       KnowledgeReviewStatusDraft,
		ArtifactPath:       defaultKnowledgeArtifactPath(strings.TrimSpace(ownerTeamID), NormalizeKnowledgeSlug(slug, title), KnowledgeSurfacePrivate),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (r *KnowledgeResource) Validate() error {
	if strings.TrimSpace(r.WorkspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(r.OwnerTeamID) == "" {
		return fmt.Errorf("owner_team_id is required")
	}
	if strings.TrimSpace(r.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(r.Slug) == "" {
		return fmt.Errorf("slug is required")
	}
	if r.Slug != NormalizeKnowledgeSlug(r.Slug, "") {
		return fmt.Errorf("slug must contain only lowercase letters, numbers, and hyphens")
	}
	if !IsValidKnowledgeResourceKind(r.Kind) {
		return fmt.Errorf("kind is invalid")
	}
	if strings.TrimSpace(r.ConceptSpecKey) == "" {
		return fmt.Errorf("concept_spec_key is required")
	}
	if r.ConceptSpecKey != NormalizeConceptSpecKey(r.ConceptSpecKey) {
		return fmt.Errorf("concept_spec_key is invalid")
	}
	if strings.TrimSpace(r.ConceptSpecVersion) == "" {
		return fmt.Errorf("concept_spec_version is required")
	}
	if r.ConceptSpecVersion != NormalizeConceptSpecVersion(r.ConceptSpecVersion) {
		return fmt.Errorf("concept_spec_version is invalid")
	}
	switch r.SourceKind {
	case KnowledgeResourceSourceKindWorkspace, KnowledgeResourceSourceKindExtension, KnowledgeResourceSourceKindImported:
	default:
		return fmt.Errorf("source_kind is invalid")
	}
	switch r.TrustLevel {
	case KnowledgeResourceTrustLevelWorkspace, KnowledgeResourceTrustLevelReviewed, KnowledgeResourceTrustLevelSystem:
	default:
		return fmt.Errorf("trust_level is invalid")
	}
	switch r.Surface {
	case KnowledgeSurfacePrivate, KnowledgeSurfacePublished, KnowledgeSurfaceWorkspaceWide:
	default:
		return fmt.Errorf("surface is invalid")
	}
	switch r.Status {
	case KnowledgeResourceStatusDraft, KnowledgeResourceStatusActive, KnowledgeResourceStatusArchived:
	default:
		return fmt.Errorf("status is invalid")
	}
	switch r.ReviewStatus {
	case KnowledgeReviewStatusDraft, KnowledgeReviewStatusReviewed, KnowledgeReviewStatusApproved:
	default:
		return fmt.Errorf("review_status is invalid")
	}
	if strings.TrimSpace(r.ArtifactPath) == "" {
		return fmt.Errorf("artifact_path is required")
	}
	return nil
}

func NormalizeKnowledgeResourceKind(value string) KnowledgeResourceKind {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if kind, ok := knowledgeResourceKindAliases[normalized]; ok {
		return kind
	}
	return KnowledgeResourceKind(normalized)
}

func ParseKnowledgeResourceKind(value string) (KnowledgeResourceKind, bool) {
	kind := NormalizeKnowledgeResourceKind(value)
	return kind, IsValidKnowledgeResourceKind(kind)
}

func IsValidKnowledgeResourceKind(kind KnowledgeResourceKind) bool {
	for _, candidate := range knowledgeResourceKinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func KnowledgeResourceKinds() []KnowledgeResourceKind {
	out := make([]KnowledgeResourceKind, len(knowledgeResourceKinds))
	copy(out, knowledgeResourceKinds)
	return out
}

func (r *KnowledgeResource) SetSlug(slug string) error {
	normalized := NormalizeKnowledgeSlug(slug, r.Title)
	if normalized == "" {
		return fmt.Errorf("slug is required")
	}
	r.Slug = normalized
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func NormalizeKnowledgeSlug(slug, fallbackTitle string) string {
	source := strings.TrimSpace(slug)
	if source == "" {
		source = strings.TrimSpace(fallbackTitle)
	}
	source = strings.ToLower(source)

	var b strings.Builder
	lastDash := false
	for _, r := range source {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "knowledge"
	}
	return result
}

type KnowledgeResourceFilter struct {
	TeamID       string
	Kind         KnowledgeResourceKind
	Status       KnowledgeResourceStatus
	Surface      KnowledgeSurface
	ReviewStatus KnowledgeReviewStatus
	Search       string
	Limit        int
	Offset       int
}

type CaseKnowledgeResourceLink struct {
	ID                  string
	CaseID              string
	KnowledgeResourceID string

	LinkedByID      string
	LinkedAt        time.Time
	IsAutoSuggested bool
	RelevanceScore  int

	WasHelpful      *bool
	FeedbackBy      string
	FeedbackAt      *time.Time
	FeedbackComment string
}

func defaultKnowledgeArtifactPath(teamID, slug string, surface KnowledgeSurface) string {
	return strings.Join([]string{"knowledge", "teams", strings.TrimSpace(teamID), string(surface), NormalizeKnowledgeSlug(slug, "") + ".md"}, "/")
}
