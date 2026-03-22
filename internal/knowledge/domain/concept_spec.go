package knowledgedomain

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

type ConceptSpecSourceKind string

const (
	ConceptSpecSourceKindCore      ConceptSpecSourceKind = "core"
	ConceptSpecSourceKindWorkspace ConceptSpecSourceKind = "workspace"
	ConceptSpecSourceKindExtension ConceptSpecSourceKind = "extension"
)

type ConceptSpecStatus string

const (
	ConceptSpecStatusDraft    ConceptSpecStatus = "draft"
	ConceptSpecStatusActive   ConceptSpecStatus = "active"
	ConceptSpecStatusArchived ConceptSpecStatus = "archived"
)

type ConceptSpec struct {
	ID          string
	WorkspaceID string
	OwnerTeamID string

	Key             string
	Version         string
	Name            string
	Description     string
	ExtendsKey      string
	ExtendsVersion  string
	InstanceKind    KnowledgeResourceKind
	MetadataSchema  shareddomain.TypedSchema
	SectionsSchema  shareddomain.TypedSchema
	WorkflowSchema  shareddomain.TypedSchema
	AgentGuidanceMD string

	ArtifactPath string
	RevisionRef  string
	SourceKind   ConceptSpecSourceKind
	SourceRef    string
	Status       ConceptSpecStatus

	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewConceptSpec(workspaceID, ownerTeamID, key, version, name string) *ConceptSpec {
	now := time.Now().UTC()
	normalizedKey := NormalizeConceptSpecKey(key)
	normalizedVersion := NormalizeConceptSpecVersion(version)
	return &ConceptSpec{
		WorkspaceID:    strings.TrimSpace(workspaceID),
		OwnerTeamID:    strings.TrimSpace(ownerTeamID),
		Key:            normalizedKey,
		Version:        normalizedVersion,
		Name:           strings.TrimSpace(name),
		InstanceKind:   KnowledgeResourceKindGuide,
		MetadataSchema: shareddomain.NewTypedSchema(),
		SectionsSchema: shareddomain.NewTypedSchema(),
		WorkflowSchema: shareddomain.NewTypedSchema(),
		ArtifactPath:   DefaultConceptSpecArtifactPath(normalizedKey, normalizedVersion),
		SourceKind:     ConceptSpecSourceKindWorkspace,
		Status:         ConceptSpecStatusDraft,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func NewBuiltInConceptSpec(key, version, name, description string, instanceKind KnowledgeResourceKind) *ConceptSpec {
	spec := NewConceptSpec("", "", key, version, name)
	spec.Description = strings.TrimSpace(description)
	spec.InstanceKind = instanceKind
	spec.SourceKind = ConceptSpecSourceKindCore
	spec.Status = ConceptSpecStatusActive
	return spec
}

func (s *ConceptSpec) Validate() error {
	if s.SourceKind != ConceptSpecSourceKindCore && strings.TrimSpace(s.WorkspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(s.Key) == "" {
		return fmt.Errorf("key is required")
	}
	if s.Key != NormalizeConceptSpecKey(s.Key) {
		return fmt.Errorf("key is invalid")
	}
	if strings.TrimSpace(s.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if s.Version != NormalizeConceptSpecVersion(s.Version) {
		return fmt.Errorf("version is invalid")
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !IsValidKnowledgeResourceKind(s.InstanceKind) {
		return fmt.Errorf("instance_kind is invalid")
	}
	switch s.SourceKind {
	case ConceptSpecSourceKindCore, ConceptSpecSourceKindWorkspace, ConceptSpecSourceKindExtension:
	default:
		return fmt.Errorf("source_kind is invalid")
	}
	switch s.Status {
	case ConceptSpecStatusDraft, ConceptSpecStatusActive, ConceptSpecStatusArchived:
	default:
		return fmt.Errorf("status is invalid")
	}
	if strings.TrimSpace(s.ExtendsKey) != "" && s.ExtendsKey != NormalizeConceptSpecKey(s.ExtendsKey) {
		return fmt.Errorf("extends_key is invalid")
	}
	if strings.TrimSpace(s.ExtendsKey) != "" && strings.TrimSpace(s.ExtendsVersion) == "" {
		return fmt.Errorf("extends_version is required when extends_key is set")
	}
	if strings.TrimSpace(s.ArtifactPath) == "" {
		return fmt.Errorf("artifact_path is required")
	}
	return nil
}

func NormalizeConceptSpecKey(value string) string {
	raw := strings.TrimSpace(strings.ToLower(value))
	if raw == "" {
		return ""
	}

	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizeConceptSpecSegment(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

func NormalizeConceptSpecVersion(value string) string {
	raw := strings.TrimSpace(strings.ToLower(value))
	if raw == "" {
		return "1"
	}
	raw = strings.TrimPrefix(raw, "v")
	var b strings.Builder
	for _, r := range raw {
		switch {
		case unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case unicode.IsLetter(r):
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), ".-_")
	if result == "" {
		return "1"
	}
	return result
}

func DefaultConceptSpecArtifactPath(key, version string) string {
	return strings.Join([]string{"concepts", NormalizeConceptSpecKey(key), "v" + NormalizeConceptSpecVersion(version), "spec.yaml"}, "/")
}

func normalizeConceptSpecSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
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
	return strings.Trim(b.String(), "-")
}

func DefaultConceptSpecForKind(kind KnowledgeResourceKind) (string, string) {
	switch kind {
	case KnowledgeResourceKindPolicy:
		return "core/policy", "1"
	case KnowledgeResourceKindGuide:
		return "core/guide", "1"
	case KnowledgeResourceKindSkill:
		return "core/skill", "1"
	case KnowledgeResourceKindContext:
		return "core/context", "1"
	case KnowledgeResourceKindConstraint:
		return "core/constraint", "1"
	case KnowledgeResourceKindBestPractice:
		return "core/best-practice", "1"
	case KnowledgeResourceKindTemplate:
		return "core/template", "1"
	case KnowledgeResourceKindChecklist:
		return "core/checklist", "1"
	case KnowledgeResourceKindDecision:
		return "core/rfc", "1"
	case KnowledgeResourceKindIdea:
		return "core/idea", "1"
	default:
		return "core/guide", "1"
	}
}
