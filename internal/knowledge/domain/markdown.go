package knowledgedomain

import (
	"strings"
	"time"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"

	"gopkg.in/yaml.v3"
)

// RenderKnowledgeMarkdown renders a knowledge resource into the canonical
// Markdown artifact format stored in the Git-backed artifact repository.
func RenderKnowledgeMarkdown(resource *KnowledgeResource) (string, error) {
	type markdownFrontmatter struct {
		Title             string         `yaml:"title"`
		Slug              string         `yaml:"slug"`
		TeamID            string         `yaml:"team_id"`
		Surface           string         `yaml:"surface"`
		Kind              string         `yaml:"kind"`
		ConceptSpec       string         `yaml:"concept_spec"`
		ConceptVersion    string         `yaml:"concept_spec_version"`
		ReviewStatus      string         `yaml:"review_status"`
		Status            string         `yaml:"status"`
		Summary           string         `yaml:"summary,omitempty"`
		SharedWithTeamIDs []string       `yaml:"shared_with_team_ids,omitempty"`
		SupportedChannels []string       `yaml:"supported_channels,omitempty"`
		SearchKeywords    []string       `yaml:"search_keywords,omitempty"`
		SourceKind        string         `yaml:"source_kind,omitempty"`
		SourceRef         string         `yaml:"source_ref,omitempty"`
		PathRef           string         `yaml:"path_ref,omitempty"`
		TrustLevel        string         `yaml:"trust_level,omitempty"`
		PublishedAt       *time.Time     `yaml:"published_at,omitempty"`
		PublishedBy       string         `yaml:"published_by,omitempty"`
		Custom            map[string]any `yaml:"custom,omitempty"`
	}

	if resource == nil {
		resource = &KnowledgeResource{Frontmatter: shareddomain.NewTypedSchema()}
	}

	frontmatter := markdownFrontmatter{
		Title:             resource.Title,
		Slug:              resource.Slug,
		TeamID:            resource.OwnerTeamID,
		Surface:           string(resource.Surface),
		Kind:              string(resource.Kind),
		ConceptSpec:       resource.ConceptSpecKey,
		ConceptVersion:    resource.ConceptSpecVersion,
		ReviewStatus:      string(resource.ReviewStatus),
		Status:            string(resource.Status),
		Summary:           resource.Summary,
		SharedWithTeamIDs: resource.SharedWithTeamIDs,
		SupportedChannels: resource.SupportedChannels,
		SearchKeywords:    resource.SearchKeywords,
		SourceKind:        string(resource.SourceKind),
		SourceRef:         resource.SourceRef,
		PathRef:           resource.PathRef,
		TrustLevel:        string(resource.TrustLevel),
		PublishedAt:       resource.PublishedAt,
		PublishedBy:       resource.PublishedBy,
	}
	if len(resource.Frontmatter.ToMap()) > 0 {
		frontmatter.Custom = resource.Frontmatter.ToMap()
	}

	rawFrontmatter, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", err
	}

	body := strings.TrimSpace(resource.BodyMarkdown)
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.Write(rawFrontmatter)
	builder.WriteString("---\n")
	if body != "" {
		builder.WriteString("\n")
		builder.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			builder.WriteByte('\n')
		}
	}
	return builder.String(), nil
}
