package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/cliapi"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

const (
	knowledgeCheckoutSchemaVersion = "mbr-knowledge-checkout-v1"
	knowledgeCheckoutMetadataDir   = ".mbr-knowledge"
	knowledgeCheckoutManifestFile  = "checkout.json"
)

type knowledgeCheckoutFilter struct {
	WorkspaceID  string  `json:"workspaceID"`
	TeamID       *string `json:"teamID,omitempty"`
	Surface      *string `json:"surface,omitempty"`
	Kind         *string `json:"kind,omitempty"`
	ReviewStatus *string `json:"reviewStatus,omitempty"`
	Status       *string `json:"status,omitempty"`
	Search       *string `json:"search,omitempty"`
}

type knowledgeCheckoutManifestEntry struct {
	ID                string  `json:"id"`
	OwnerTeamID       string  `json:"ownerTeamID"`
	Surface           string  `json:"surface"`
	Slug              string  `json:"slug"`
	Title             string  `json:"title"`
	Kind              string  `json:"kind"`
	ArtifactPath      string  `json:"artifactPath"`
	RevisionRef       string  `json:"revisionRef"`
	ContentHash       string  `json:"contentHash"`
	RenderedHash      string  `json:"renderedHash"`
	PublishedRevision *string `json:"publishedRevision,omitempty"`
}

type knowledgeCheckoutManifest struct {
	SchemaVersion string                           `json:"schemaVersion"`
	InstanceURL   string                           `json:"instanceURL"`
	WorkspaceID   string                           `json:"workspaceID"`
	Filters       knowledgeCheckoutFilter          `json:"filters"`
	CheckedOutAt  string                           `json:"checkedOutAt"`
	Resources     []knowledgeCheckoutManifestEntry `json:"resources"`
}

type knowledgeCheckoutResult struct {
	RootPath      string `json:"rootPath"`
	ManifestPath  string `json:"manifestPath"`
	ResourceCount int    `json:"resourceCount"`
}

type knowledgeCheckoutStatusSummary struct {
	Clean           int `json:"clean"`
	Ahead           int `json:"ahead"`
	Behind          int `json:"behind"`
	Diverged        int `json:"diverged"`
	LocalOnly       int `json:"localOnly"`
	NewOnServer     int `json:"newOnServer"`
	DeletedLocal    int `json:"deletedLocal"`
	DeletedOnServer int `json:"deletedOnServer"`
}

type knowledgeCheckoutStatusEntry struct {
	Path              string  `json:"path"`
	State             string  `json:"state"`
	ResourceID        *string `json:"resourceID,omitempty"`
	RevisionRef       *string `json:"revisionRef,omitempty"`
	ServerRevisionRef *string `json:"serverRevisionRef,omitempty"`
}

type knowledgeCheckoutStatusResult struct {
	RootPath string                         `json:"rootPath"`
	Status   string                         `json:"status"`
	Summary  knowledgeCheckoutStatusSummary `json:"summary"`
	Entries  []knowledgeCheckoutStatusEntry `json:"entries"`
}

type knowledgeCheckoutPullSummary struct {
	Added              int `json:"added"`
	Updated            int `json:"updated"`
	Deleted            int `json:"deleted"`
	PreservedLocalOnly int `json:"preservedLocalOnly"`
}

type knowledgeCheckoutPullEntry struct {
	Path        string  `json:"path"`
	Action      string  `json:"action"`
	ResourceID  *string `json:"resourceID,omitempty"`
	RevisionRef *string `json:"revisionRef,omitempty"`
}

type knowledgeCheckoutPullResult struct {
	RootPath     string                       `json:"rootPath"`
	ManifestPath string                       `json:"manifestPath"`
	Status       string                       `json:"status"`
	Summary      knowledgeCheckoutPullSummary `json:"summary"`
	Entries      []knowledgeCheckoutPullEntry `json:"entries"`
}

type knowledgeCheckoutPushSummary struct {
	Updated int `json:"updated"`
}

type knowledgeCheckoutPushEntry struct {
	Path        string  `json:"path"`
	Action      string  `json:"action"`
	ResourceID  *string `json:"resourceID,omitempty"`
	RevisionRef *string `json:"revisionRef,omitempty"`
}

type knowledgeCheckoutPushResult struct {
	RootPath     string                       `json:"rootPath"`
	ManifestPath string                       `json:"manifestPath"`
	Status       string                       `json:"status"`
	Summary      knowledgeCheckoutPushSummary `json:"summary"`
	Entries      []knowledgeCheckoutPushEntry `json:"entries"`
}

func executeKnowledgeCheckout(ctx context.Context, instanceURL, token, workspaceID, teamID, surface, kind, reviewStatus, status, search, destination string) (knowledgeCheckoutResult, error) {
	stored, err := cliapi.LoadStoredConfig()
	if err != nil {
		return knowledgeCheckoutResult{}, err
	}
	workspaceValue, err := requireWorkspaceID(workspaceID, stored)
	if err != nil {
		return knowledgeCheckoutResult{}, err
	}
	filter := knowledgeCheckoutFilter{
		WorkspaceID: workspaceValue,
	}
	if value := strings.TrimSpace(resolveStoredTeamID(teamID, stored)); value != "" {
		filter.TeamID = &value
	}
	if value := strings.TrimSpace(surface); value != "" {
		lower := strings.ToLower(value)
		filter.Surface = &lower
	}
	if value, err := normalizeKnowledgeKindValue(kind); err != nil {
		return knowledgeCheckoutResult{}, err
	} else if value != "" {
		filter.Kind = &value
	}
	if value := strings.TrimSpace(reviewStatus); value != "" {
		lower := strings.ToLower(value)
		filter.ReviewStatus = &lower
	}
	if value := strings.TrimSpace(status); value != "" {
		lower := strings.ToLower(value)
		filter.Status = &lower
	}
	if value := strings.TrimSpace(search); value != "" {
		filter.Search = &value
	}

	cfg, err := loadCLIConfig(instanceURL, token)
	if err != nil {
		return knowledgeCheckoutResult{}, err
	}
	client := newCLIClient(cfg)
	resources, err := listKnowledgeResourcesForCheckout(ctx, client, filter)
	if err != nil {
		return knowledgeCheckoutResult{}, err
	}

	rootPath, err := prepareKnowledgeCheckoutRoot(destination)
	if err != nil {
		return knowledgeCheckoutResult{}, err
	}
	manifest, err := materializeKnowledgeCheckoutResources(rootPath, cfg.InstanceURL, workspaceValue, filter, resources)
	if err != nil {
		return knowledgeCheckoutResult{}, err
	}
	manifestPath, err := writeKnowledgeCheckoutManifest(rootPath, manifest)
	if err != nil {
		return knowledgeCheckoutResult{}, err
	}

	return knowledgeCheckoutResult{
		RootPath:      rootPath,
		ManifestPath:  manifestPath,
		ResourceCount: len(resources),
	}, nil
}

func executeKnowledgePull(ctx context.Context, path, instanceURL, token string) (knowledgeCheckoutPullResult, error) {
	rootPath, manifest, err := loadKnowledgeCheckoutManifest(path)
	if err != nil {
		return knowledgeCheckoutPullResult{}, err
	}
	status, err := executeKnowledgeStatus(ctx, rootPath, instanceURL, token)
	if err != nil {
		return knowledgeCheckoutPullResult{}, err
	}
	blockers := make([]string, 0)
	for _, entry := range status.Entries {
		switch entry.State {
		case "ahead", "diverged", "deleted_local":
			blockers = append(blockers, fmt.Sprintf("%s (%s)", entry.Path, entry.State))
		}
	}
	if len(blockers) > 0 {
		return knowledgeCheckoutPullResult{}, fmt.Errorf("knowledge working copy has local tracked changes; resolve them before pull: %s", strings.Join(blockers, ", "))
	}

	effectiveURL := strings.TrimSpace(instanceURL)
	if effectiveURL == "" {
		effectiveURL = manifest.InstanceURL
	}
	cfg, err := loadCLIConfig(effectiveURL, token)
	if err != nil {
		return knowledgeCheckoutPullResult{}, err
	}
	client := newCLIClient(cfg)
	currentResources, err := listKnowledgeResourcesForCheckout(ctx, client, manifest.Filters)
	if err != nil {
		return knowledgeCheckoutPullResult{}, err
	}

	newManifest, err := materializeKnowledgeCheckoutResources(rootPath, cfg.InstanceURL, manifest.WorkspaceID, manifest.Filters, currentResources)
	if err != nil {
		return knowledgeCheckoutPullResult{}, err
	}
	entries, summary, err := applyKnowledgeCheckoutDeletions(rootPath, manifest, &newManifest)
	if err != nil {
		return knowledgeCheckoutPullResult{}, err
	}
	added, updated := summarizeKnowledgeCheckoutUpserts(manifest, &newManifest, &entries)
	summary.Added = added
	summary.Updated = updated
	summary.PreservedLocalOnly = status.Summary.LocalOnly
	manifestPath, err := writeKnowledgeCheckoutManifest(rootPath, newManifest)
	if err != nil {
		return knowledgeCheckoutPullResult{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Action == entries[j].Action {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Path < entries[j].Path
	})
	statusValue := "clean"
	if summary.PreservedLocalOnly > 0 {
		statusValue = "local_only"
	}
	return knowledgeCheckoutPullResult{
		RootPath:     rootPath,
		ManifestPath: manifestPath,
		Status:       statusValue,
		Summary:      summary,
		Entries:      entries,
	}, nil
}

func executeKnowledgePush(ctx context.Context, path, instanceURL, token string) (knowledgeCheckoutPushResult, error) {
	rootPath, manifest, err := loadKnowledgeCheckoutManifest(path)
	if err != nil {
		return knowledgeCheckoutPushResult{}, err
	}
	status, err := executeKnowledgeStatus(ctx, rootPath, instanceURL, token)
	if err != nil {
		return knowledgeCheckoutPushResult{}, err
	}

	pullFirst := make([]string, 0)
	deletionBlockers := make([]string, 0)
	createBlockers := make([]string, 0)
	trackedChanges := make([]knowledgeCheckoutStatusEntry, 0)
	for _, entry := range status.Entries {
		switch entry.State {
		case "ahead":
			trackedChanges = append(trackedChanges, entry)
		case "behind", "diverged", "new_on_server", "deleted_on_server":
			pullFirst = append(pullFirst, fmt.Sprintf("%s (%s)", entry.Path, entry.State))
		case "deleted_local":
			deletionBlockers = append(deletionBlockers, entry.Path)
		case "local_only":
			createBlockers = append(createBlockers, entry.Path)
		}
	}
	if len(pullFirst) > 0 {
		return knowledgeCheckoutPushResult{}, fmt.Errorf("knowledge working copy is behind the server; run `mbr knowledge pull` first: %s", strings.Join(pullFirst, ", "))
	}
	if len(deletionBlockers) > 0 {
		return knowledgeCheckoutPushResult{}, fmt.Errorf("pushing tracked deletions is not supported yet; restore these files or use a server-side workflow: %s", strings.Join(deletionBlockers, ", "))
	}
	if len(createBlockers) > 0 {
		return knowledgeCheckoutPushResult{}, fmt.Errorf("pushing new local-only files is not supported yet; use `mbr knowledge sync` or `mbr knowledge import`: %s", strings.Join(createBlockers, ", "))
	}

	effectiveURL := strings.TrimSpace(instanceURL)
	if effectiveURL == "" {
		effectiveURL = manifest.InstanceURL
	}
	cfg, err := loadCLIConfig(effectiveURL, token)
	if err != nil {
		return knowledgeCheckoutPushResult{}, err
	}
	client := newCLIClient(cfg)

	manifestByID := make(map[string]knowledgeCheckoutManifestEntry, len(manifest.Resources))
	for _, item := range manifest.Resources {
		manifestByID[item.ID] = item
	}
	results := make([]knowledgeCheckoutPushEntry, 0, len(trackedChanges))
	for _, entry := range trackedChanges {
		if entry.ResourceID == nil || strings.TrimSpace(*entry.ResourceID) == "" {
			return knowledgeCheckoutPushResult{}, fmt.Errorf("tracked knowledge entry %s is missing a resource id", entry.Path)
		}
		manifestEntry, ok := manifestByID[strings.TrimSpace(*entry.ResourceID)]
		if !ok {
			return knowledgeCheckoutPushResult{}, fmt.Errorf("tracked knowledge entry %s is missing from the checkout manifest", entry.Path)
		}
		current, err := runKnowledgeShow(ctx, client, manifestEntry.ID, "", "", "")
		if err != nil {
			return knowledgeCheckoutPushResult{}, err
		}
		localPath, err := knowledgeCheckoutLocalPath(rootPath, manifestEntry.ArtifactPath)
		if err != nil {
			return knowledgeCheckoutPushResult{}, err
		}
		document, err := parseKnowledgeSyncDocument(localPath, manifestEntry.ArtifactPath)
		if err != nil {
			return knowledgeCheckoutPushResult{}, err
		}
		defaults := knowledgeSyncDefaults{
			WorkspaceID:        manifest.WorkspaceID,
			TeamID:             current.OwnerTeamID,
			Surface:            current.Surface,
			Kind:               current.Kind,
			ConceptSpecKey:     current.ConceptSpecKey,
			ConceptSpecVersion: current.ConceptSpecVersion,
			Status:             current.Status,
			ReviewStatus:       current.ReviewStatus,
			SharedWithTeamIDs:  append([]string(nil), current.SharedWithTeamIDs...),
			SourceKind:         current.SourceKind,
			SourceRef:          stringValue(current.SourceRef),
		}
		plan, err := planKnowledgeImport(defaults, document)
		if err != nil {
			return knowledgeCheckoutPushResult{}, err
		}
		if plan.TeamID != current.OwnerTeamID {
			return knowledgeCheckoutPushResult{}, fmt.Errorf("changing owner team through push is not supported for %s", entry.Path)
		}
		bodyValue := document.BodyMarkdown
		resource, err := runKnowledgeUpdate(ctx, client, current.ID, knowledgeMutationInput{
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
			return knowledgeCheckoutPushResult{}, err
		}
		if plan.ReviewStatus != "" && !strings.EqualFold(resource.ReviewStatus, plan.ReviewStatus) {
			resource, err = runKnowledgeReview(ctx, client, resource.ID, plan.ReviewStatus)
			if err != nil {
				return knowledgeCheckoutPushResult{}, err
			}
		}
		if plan.Surface != "" && !strings.EqualFold(resource.Surface, plan.Surface) {
			if strings.EqualFold(plan.Surface, "private") {
				return knowledgeCheckoutPushResult{}, fmt.Errorf("cannot push %s to private because the resource already lives on %s", entry.Path, resource.Surface)
			}
			resource, err = runKnowledgePublish(ctx, client, resource.ID, plan.Surface)
			if err != nil {
				return knowledgeCheckoutPushResult{}, err
			}
		}
		if len(plan.SharedWithTeamIDs) > 0 && !sameStringSet(resource.SharedWithTeamIDs, plan.SharedWithTeamIDs) {
			resource, err = runKnowledgeShare(ctx, client, resource.ID, plan.SharedWithTeamIDs)
			if err != nil {
				return knowledgeCheckoutPushResult{}, err
			}
		}
		results = append(results, knowledgeCheckoutPushEntry{
			Path:        entry.Path,
			Action:      "updated",
			ResourceID:  stringPtr(resource.ID),
			RevisionRef: stringPtr(resource.RevisionRef),
		})
	}

	currentResources, err := listKnowledgeResourcesForCheckout(ctx, client, manifest.Filters)
	if err != nil {
		return knowledgeCheckoutPushResult{}, err
	}
	newManifest, err := materializeKnowledgeCheckoutResources(rootPath, cfg.InstanceURL, manifest.WorkspaceID, manifest.Filters, currentResources)
	if err != nil {
		return knowledgeCheckoutPushResult{}, err
	}
	if _, _, err := applyKnowledgeCheckoutDeletions(rootPath, manifest, &newManifest); err != nil {
		return knowledgeCheckoutPushResult{}, err
	}
	manifestPath, err := writeKnowledgeCheckoutManifest(rootPath, newManifest)
	if err != nil {
		return knowledgeCheckoutPushResult{}, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	return knowledgeCheckoutPushResult{
		RootPath:     rootPath,
		ManifestPath: manifestPath,
		Status:       "clean",
		Summary: knowledgeCheckoutPushSummary{
			Updated: len(results),
		},
		Entries: results,
	}, nil
}

func executeKnowledgeStatus(ctx context.Context, path, instanceURL, token string) (knowledgeCheckoutStatusResult, error) {
	rootPath, manifest, err := loadKnowledgeCheckoutManifest(path)
	if err != nil {
		return knowledgeCheckoutStatusResult{}, err
	}
	effectiveURL := strings.TrimSpace(instanceURL)
	if effectiveURL == "" {
		effectiveURL = manifest.InstanceURL
	}
	cfg, err := loadCLIConfig(effectiveURL, token)
	if err != nil {
		return knowledgeCheckoutStatusResult{}, err
	}
	client := newCLIClient(cfg)

	currentResources, err := listKnowledgeResourcesForCheckout(ctx, client, manifest.Filters)
	if err != nil {
		return knowledgeCheckoutStatusResult{}, err
	}
	currentByID := make(map[string]knowledgeResourceOutput, len(currentResources))
	for _, resource := range currentResources {
		currentByID[resource.ID] = resource
	}

	entries := make([]knowledgeCheckoutStatusEntry, 0, len(manifest.Resources))
	summary := knowledgeCheckoutStatusSummary{}
	hasAhead := false
	hasBehind := false
	hasDiverged := false

	for _, item := range manifest.Resources {
		localPath, err := knowledgeCheckoutLocalPath(rootPath, item.ArtifactPath)
		if err != nil {
			return knowledgeCheckoutStatusResult{}, err
		}
		localHash, localExists, err := knowledgeCheckoutLocalRenderedHash(localPath)
		if err != nil {
			return knowledgeCheckoutStatusResult{}, err
		}

		current, serverExists := currentByID[item.ID]
		serverChanged := !serverExists || current.RevisionRef != item.RevisionRef || current.ArtifactPath != item.ArtifactPath
		localChanged := !localExists || localHash != item.RenderedHash

		state := classifyKnowledgeCheckoutState(localExists, localChanged, serverExists, serverChanged)
		entry := knowledgeCheckoutStatusEntry{
			Path:        item.ArtifactPath,
			State:       state,
			ResourceID:  stringPtr(item.ID),
			RevisionRef: stringPtr(item.RevisionRef),
		}
		if serverExists {
			entry.ServerRevisionRef = stringPtr(current.RevisionRef)
			delete(currentByID, item.ID)
		}
		switch state {
		case "clean":
			summary.Clean++
		case "ahead":
			summary.Ahead++
			hasAhead = true
		case "behind":
			summary.Behind++
			hasBehind = true
		case "diverged":
			summary.Diverged++
			hasAhead = true
			hasBehind = true
			hasDiverged = true
		case "deleted_local":
			summary.DeletedLocal++
			hasAhead = true
		case "deleted_on_server":
			summary.DeletedOnServer++
			hasBehind = true
		}
		entries = append(entries, entry)
	}

	localOnly, err := detectKnowledgeCheckoutLocalOnlyFiles(rootPath, manifest)
	if err != nil {
		return knowledgeCheckoutStatusResult{}, err
	}
	for _, artifactPath := range localOnly {
		summary.LocalOnly++
		hasAhead = true
		entries = append(entries, knowledgeCheckoutStatusEntry{
			Path:  artifactPath,
			State: "local_only",
		})
	}

	for _, resource := range currentByID {
		summary.NewOnServer++
		hasBehind = true
		entries = append(entries, knowledgeCheckoutStatusEntry{
			Path:              resource.ArtifactPath,
			State:             "new_on_server",
			ResourceID:        stringPtr(resource.ID),
			ServerRevisionRef: stringPtr(resource.RevisionRef),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].State == entries[j].State {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Path < entries[j].Path
	})

	statusValue := "clean"
	switch {
	case hasDiverged || (hasAhead && hasBehind):
		statusValue = "diverged"
	case hasBehind:
		statusValue = "behind"
	case hasAhead:
		statusValue = "ahead"
	}

	return knowledgeCheckoutStatusResult{
		RootPath: rootPath,
		Status:   statusValue,
		Summary:  summary,
		Entries:  entries,
	}, nil
}

func listKnowledgeResourcesForCheckout(ctx context.Context, client *cliapi.Client, filter knowledgeCheckoutFilter) ([]knowledgeResourceOutput, error) {
	results := make([]knowledgeResourceOutput, 0)
	const batchSize = 100
	for offset := 0; ; offset += batchSize {
		graphQLFilter := map[string]any{
			"limit":  batchSize,
			"offset": offset,
		}
		if filter.TeamID != nil && strings.TrimSpace(*filter.TeamID) != "" {
			graphQLFilter["teamID"] = strings.TrimSpace(*filter.TeamID)
		}
		if filter.Surface != nil && strings.TrimSpace(*filter.Surface) != "" {
			graphQLFilter["surface"] = strings.ToLower(strings.TrimSpace(*filter.Surface))
		}
		if filter.Kind != nil && strings.TrimSpace(*filter.Kind) != "" {
			graphQLFilter["kind"] = strings.TrimSpace(*filter.Kind)
		}
		if filter.ReviewStatus != nil && strings.TrimSpace(*filter.ReviewStatus) != "" {
			graphQLFilter["reviewStatus"] = strings.ToLower(strings.TrimSpace(*filter.ReviewStatus))
		}
		if filter.Status != nil && strings.TrimSpace(*filter.Status) != "" {
			graphQLFilter["status"] = strings.ToLower(strings.TrimSpace(*filter.Status))
		}
		if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
			graphQLFilter["search"] = strings.TrimSpace(*filter.Search)
		}

		var payload struct {
			KnowledgeResources []knowledgeResourceOutput `json:"knowledgeResources"`
		}
		err := client.Query(ctx, `
			query CLIKnowledgeCheckoutResources($workspaceID: ID!, $filter: KnowledgeResourceFilter) {
			  knowledgeResources(workspaceID: $workspaceID, filter: $filter) {
			    `+knowledgeSelection+`
			  }
			}
		`, map[string]any{
			"workspaceID": filter.WorkspaceID,
			"filter":      graphQLFilter,
		}, &payload)
		if err != nil {
			return nil, err
		}
		results = append(results, payload.KnowledgeResources...)
		if len(payload.KnowledgeResources) < batchSize {
			break
		}
	}
	return results, nil
}

func materializeKnowledgeCheckoutResources(rootPath, instanceURL, workspaceID string, filter knowledgeCheckoutFilter, resources []knowledgeResourceOutput) (knowledgeCheckoutManifest, error) {
	manifest := knowledgeCheckoutManifest{
		SchemaVersion: knowledgeCheckoutSchemaVersion,
		InstanceURL:   instanceURL,
		WorkspaceID:   workspaceID,
		Filters:       filter,
		CheckedOutAt:  time.Now().UTC().Format(time.RFC3339),
		Resources:     make([]knowledgeCheckoutManifestEntry, 0, len(resources)),
	}
	for _, resource := range resources {
		rendered, err := renderKnowledgeCheckoutMarkdown(resource)
		if err != nil {
			return knowledgeCheckoutManifest{}, err
		}
		targetPath, err := knowledgeCheckoutLocalPath(rootPath, resource.ArtifactPath)
		if err != nil {
			return knowledgeCheckoutManifest{}, err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return knowledgeCheckoutManifest{}, fmt.Errorf("create checkout directory: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(rendered), 0o644); err != nil {
			return knowledgeCheckoutManifest{}, fmt.Errorf("write knowledge checkout file: %w", err)
		}
		manifest.Resources = append(manifest.Resources, knowledgeCheckoutManifestEntry{
			ID:                resource.ID,
			OwnerTeamID:       resource.OwnerTeamID,
			Surface:           resource.Surface,
			Slug:              resource.Slug,
			Title:             resource.Title,
			Kind:              resource.Kind,
			ArtifactPath:      resource.ArtifactPath,
			RevisionRef:       resource.RevisionRef,
			ContentHash:       resource.ContentHash,
			RenderedHash:      knowledgeRenderedHash(rendered),
			PublishedRevision: resource.PublishedRevision,
		})
	}
	sort.Slice(manifest.Resources, func(i, j int) bool {
		return manifest.Resources[i].ArtifactPath < manifest.Resources[j].ArtifactPath
	})
	return manifest, nil
}

func writeKnowledgeCheckoutManifest(rootPath string, manifest knowledgeCheckoutManifest) (string, error) {
	manifestPath := filepath.Join(rootPath, knowledgeCheckoutMetadataDir, knowledgeCheckoutManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return "", fmt.Errorf("create checkout metadata directory: %w", err)
	}
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode checkout manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, append(rawManifest, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write checkout manifest: %w", err)
	}
	return manifestPath, nil
}

func applyKnowledgeCheckoutDeletions(rootPath string, previous, current *knowledgeCheckoutManifest) ([]knowledgeCheckoutPullEntry, knowledgeCheckoutPullSummary, error) {
	entries := make([]knowledgeCheckoutPullEntry, 0)
	summary := knowledgeCheckoutPullSummary{}
	currentByID := make(map[string]knowledgeCheckoutManifestEntry, len(current.Resources))
	for _, item := range current.Resources {
		currentByID[item.ID] = item
	}
	for _, item := range previous.Resources {
		currentItem, ok := currentByID[item.ID]
		if ok && currentItem.ArtifactPath == item.ArtifactPath {
			continue
		}
		localPath, err := knowledgeCheckoutLocalPath(rootPath, item.ArtifactPath)
		if err != nil {
			return nil, knowledgeCheckoutPullSummary{}, err
		}
		if err := os.Remove(localPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, knowledgeCheckoutPullSummary{}, fmt.Errorf("remove stale knowledge file: %w", err)
		}
		entries = append(entries, knowledgeCheckoutPullEntry{
			Path:       item.ArtifactPath,
			Action:     "deleted",
			ResourceID: stringPtr(item.ID),
		})
		summary.Deleted++
	}
	return entries, summary, nil
}

func summarizeKnowledgeCheckoutUpserts(previous, current *knowledgeCheckoutManifest, entries *[]knowledgeCheckoutPullEntry) (int, int) {
	previousByID := make(map[string]knowledgeCheckoutManifestEntry, len(previous.Resources))
	for _, item := range previous.Resources {
		previousByID[item.ID] = item
	}
	added := 0
	updated := 0
	for _, item := range current.Resources {
		prev, ok := previousByID[item.ID]
		action := ""
		switch {
		case !ok:
			action = "added"
			added++
		case prev.RevisionRef != item.RevisionRef || prev.ArtifactPath != item.ArtifactPath || prev.RenderedHash != item.RenderedHash:
			action = "updated"
			updated++
		}
		if action != "" {
			*entries = append(*entries, knowledgeCheckoutPullEntry{
				Path:        item.ArtifactPath,
				Action:      action,
				ResourceID:  stringPtr(item.ID),
				RevisionRef: stringPtr(item.RevisionRef),
			})
		}
	}
	return added, updated
}

func prepareKnowledgeCheckoutRoot(path string) (string, error) {
	rootPath := strings.TrimSpace(path)
	if rootPath == "" {
		return "", fmt.Errorf("checkout path is required")
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("resolve checkout path: %w", err)
	}
	if info, err := os.Stat(absRoot); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("checkout path must be a directory")
		}
		entries, err := os.ReadDir(absRoot)
		if err != nil {
			return "", fmt.Errorf("read checkout path: %w", err)
		}
		if len(entries) > 0 {
			return "", fmt.Errorf("checkout path must be empty")
		}
	} else if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(absRoot, 0o755); err != nil {
			return "", fmt.Errorf("create checkout path: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("stat checkout path: %w", err)
	}
	return absRoot, nil
}

func loadKnowledgeCheckoutManifest(path string) (string, *knowledgeCheckoutManifest, error) {
	rootPath := strings.TrimSpace(path)
	if rootPath == "" {
		rootPath = "."
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", nil, fmt.Errorf("resolve checkout path: %w", err)
	}
	manifestPath := filepath.Join(absRoot, knowledgeCheckoutMetadataDir, knowledgeCheckoutManifestFile)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, fmt.Errorf("knowledge checkout manifest not found at %s", manifestPath)
		}
		return "", nil, fmt.Errorf("read knowledge checkout manifest: %w", err)
	}
	var manifest knowledgeCheckoutManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", nil, fmt.Errorf("decode knowledge checkout manifest: %w", err)
	}
	if manifest.SchemaVersion != knowledgeCheckoutSchemaVersion {
		return "", nil, fmt.Errorf("unsupported knowledge checkout schema version %q", manifest.SchemaVersion)
	}
	return absRoot, &manifest, nil
}

func renderKnowledgeCheckoutMarkdown(resource knowledgeResourceOutput) (string, error) {
	doc := &knowledgedomain.KnowledgeResource{
		ID:                 resource.ID,
		WorkspaceID:        resource.WorkspaceID,
		OwnerTeamID:        resource.OwnerTeamID,
		Slug:               resource.Slug,
		Title:              resource.Title,
		Kind:               knowledgedomain.KnowledgeResourceKind(resource.Kind),
		ConceptSpecKey:     resource.ConceptSpecKey,
		ConceptSpecVersion: resource.ConceptSpecVersion,
		SourceKind:         knowledgedomain.KnowledgeResourceSourceKind(resource.SourceKind),
		Summary:            stringValue(resource.Summary),
		BodyMarkdown:       resource.BodyMarkdown,
		Frontmatter:        shareddomain.TypedSchemaFromMap(resource.Frontmatter),
		SupportedChannels:  append([]string(nil), resource.SupportedChannels...),
		SharedWithTeamIDs:  append([]string(nil), resource.SharedWithTeamIDs...),
		Surface:            knowledgedomain.KnowledgeSurface(resource.Surface),
		TrustLevel:         knowledgedomain.KnowledgeResourceTrustLevel(resource.TrustLevel),
		SearchKeywords:     append([]string(nil), resource.SearchKeywords...),
		Status:             knowledgedomain.KnowledgeResourceStatus(resource.Status),
		ReviewStatus:       knowledgedomain.KnowledgeReviewStatus(resource.ReviewStatus),
		ArtifactPath:       resource.ArtifactPath,
		RevisionRef:        resource.RevisionRef,
		PublishedRevision:  stringValue(resource.PublishedRevision),
		PublishedBy:        stringValue(resource.PublishedByID),
		CreatedBy:          stringValue(resource.CreatedByID),
	}
	doc.SourceRef = stringValue(resource.SourceRef)
	doc.PathRef = stringValue(resource.PathRef)
	if publishedAt, ok := parseOptionalRFC3339(resource.PublishedAt); ok {
		doc.PublishedAt = publishedAt
	}
	if reviewedAt, ok := parseOptionalRFC3339(resource.ReviewedAt); ok {
		doc.ReviewedAt = reviewedAt
	}
	return knowledgedomain.RenderKnowledgeMarkdown(doc)
}

func knowledgeCheckoutLocalPath(rootPath, artifactPath string) (string, error) {
	cleanRelative := filepath.Clean(filepath.FromSlash(strings.TrimSpace(artifactPath)))
	if cleanRelative == "." || cleanRelative == "" {
		return "", fmt.Errorf("artifact path is required")
	}
	if filepath.IsAbs(cleanRelative) || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) || cleanRelative == ".." {
		return "", fmt.Errorf("artifact path must remain within the checkout root")
	}
	return filepath.Join(rootPath, cleanRelative), nil
}

func knowledgeCheckoutLocalRenderedHash(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read local knowledge file: %w", err)
	}
	return knowledgeRenderedHash(string(data)), true, nil
}

func knowledgeRenderedHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func classifyKnowledgeCheckoutState(localExists, localChanged, serverExists, serverChanged bool) string {
	switch {
	case !localExists && !serverExists:
		return "diverged"
	case !localExists:
		if serverChanged {
			return "diverged"
		}
		return "deleted_local"
	case !serverExists:
		if localChanged {
			return "diverged"
		}
		return "deleted_on_server"
	case localChanged && serverChanged:
		return "diverged"
	case localChanged:
		return "ahead"
	case serverChanged:
		return "behind"
	default:
		return "clean"
	}
}

func detectKnowledgeCheckoutLocalOnlyFiles(rootPath string, manifest *knowledgeCheckoutManifest) ([]string, error) {
	tracked := make(map[string]struct{}, len(manifest.Resources))
	for _, entry := range manifest.Resources {
		tracked[filepath.Clean(filepath.FromSlash(entry.ArtifactPath))] = struct{}{}
	}
	localOnly := make([]string, 0)
	err := filepath.WalkDir(rootPath, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(rootPath, current)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if rel == knowledgeCheckoutMetadataDir {
				return fs.SkipDir
			}
			return nil
		}
		if _, ok := tracked[rel]; ok {
			return nil
		}
		localOnly = append(localOnly, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan checkout root: %w", err)
	}
	sort.Strings(localOnly)
	return localOnly, nil
}

func parseOptionalRFC3339(value *string) (*time.Time, bool) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, false
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
