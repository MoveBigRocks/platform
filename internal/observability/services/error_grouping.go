package observabilityservices

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
)

// ErrorGroupingService handles error event grouping using event-driven architecture
// KEY DIFFERENCE: Only depends on OutboxPublisher (1 method) instead of entire Store interface (40+ methods)
// This makes testing 10x easier - only need to mock 1 method!
type ErrorGroupingService struct {
	// Read-only store access (for queries only)
	issueStore   shared.IssueStore
	projectStore shared.ProjectStore

	// Event publishing (the ONLY write dependency!)
	outbox contracts.OutboxPublisher

	similarity SimilarityEngine
	logger     *log.Logger
}

// SimilarityEngine is simplified - no cache dependency
type SimilarityEngine struct{}

type FingerprintComponents = observabilitydomain.FingerprintComponents

// NewErrorGroupingService creates the event-driven version
func NewErrorGroupingService(issueStore shared.IssueStore, projectStore shared.ProjectStore, outbox contracts.OutboxPublisher) *ErrorGroupingService {
	return &ErrorGroupingService{
		issueStore:   issueStore,
		projectStore: projectStore,
		outbox:       outbox,
		similarity:   SimilarityEngine{},
		logger:       log.New(log.Writer(), "[ErrorGrouping] ", log.LstdFlags),
	}
}

// GroupEvent finds or creates an issue for the given error event
// EVENT-DRIVEN: Publishes IssueCreated or IssueUpdated events instead of directly updating storage
func (e *ErrorGroupingService) GroupEvent(ctx context.Context, event *observabilitydomain.ErrorEvent) (*observabilitydomain.Issue, bool, error) {
	start := time.Now()

	// Generate fingerprint for this event
	fingerprint := e.generateAdvancedFingerprint(event)
	event.Fingerprint = []string{fingerprint}

	// Try to find existing issue with same fingerprint
	existingIssue, err := e.issueStore.GetIssueByFingerprint(ctx, event.ProjectID, fingerprint)
	if err == nil && existingIssue != nil {
		// EVENT: Publish IssueUpdated instead of directly updating
		if err := e.publishIssueUpdated(ctx, existingIssue, event, start); err != nil {
			return nil, false, fmt.Errorf("failed to publish issue updated event: %w", err)
		}
		return existingIssue, false, nil
	}

	// Try similarity matching for issues created in the last 24 hours
	similarIssue, confidence, err := e.findSimilarIssue(ctx, event)
	if err != nil {
		e.logger.Printf("Error finding similar issue: %v", err)
	}

	if similarIssue != nil && confidence > 0.8 {
		// EVENT: Publish IssueUpdated for similar issue merge
		if err := e.publishIssueUpdated(ctx, similarIssue, event, start); err != nil {
			return nil, false, fmt.Errorf("failed to publish issue updated event: %w", err)
		}
		return similarIssue, false, nil
	}

	// EVENT: Publish IssueCreated instead of directly creating
	newIssue, err := e.publishIssueCreated(ctx, event, start)
	if err != nil {
		return nil, false, fmt.Errorf("failed to publish issue created event: %w", err)
	}

	return newIssue, true, nil
}

// publishIssueCreated publishes an IssueCreated event
// This replaces the direct CreateIssue store call in the original version
func (e *ErrorGroupingService) publishIssueCreated(ctx context.Context, event *observabilitydomain.ErrorEvent, start time.Time) (*observabilitydomain.Issue, error) {
	title := e.generateIssueTitle(event)
	culprit := e.extractCulprit(event)

	issue := observabilitydomain.NewIssue(event.ProjectID, title, culprit, event)
	event.IssueID = issue.ID

	// Get workspace ID for the event
	project, err := e.projectStore.GetProject(ctx, event.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Create the domain event using constructor
	issueCreatedEvent := shareddomain.NewIssueCreatedEvent(
		issue.ID,
		issue.ProjectID,
		project.WorkspaceID,
		issue.Title,
		issue.Level,
		issue.Fingerprint,
		event.EventID,
		issue.Platform,
		issue.Culprit,
	)

	// PUBLISH EVENT: This is the only write operation!
	// Event handlers will:
	// 1. Store the issue
	// 2. Update metrics
	// 3. Check alert rules
	// 4. Send notifications
	if err := e.outbox.PublishEvent(ctx, eventbus.StreamIssueEvents, issueCreatedEvent); err != nil {
		return nil, fmt.Errorf("failed to publish IssueCreated event: %w", err)
	}

	e.logger.Printf("Published IssueCreated event for issue %s (took %v)", issue.ID, time.Since(start))
	return issue, nil
}

// publishIssueUpdated publishes an IssueUpdated event
// This replaces the direct UpdateIssue store call in the original version.
// Uses atomic increment in the handler to prevent race conditions with concurrent updates.
func (e *ErrorGroupingService) publishIssueUpdated(ctx context.Context, issue *observabilitydomain.Issue, event *observabilitydomain.ErrorEvent, start time.Time) error {
	event.IssueID = issue.ID

	// Determine if this event has a new user that should increment user_count.
	// The handler will use atomic increment (event_count = event_count + 1)
	// instead of the old optimistic update that caused race conditions.
	hasNewUser := event.User != nil && event.User.ID != ""

	// Create the domain event using the new constructor with user flag.
	// This signals to the handler whether to atomically increment user_count.
	issueUpdatedEvent := shareddomain.NewIssueUpdatedEventWithUserFlag(
		issue.ID,
		issue.ProjectID,
		issue.WorkspaceID,
		event.EventID,
		event.Timestamp,
		hasNewUser,
	)

	// PUBLISH EVENT: Event handlers will atomically update storage
	if err := e.outbox.PublishEvent(ctx, eventbus.StreamIssueEvents, issueUpdatedEvent); err != nil {
		return fmt.Errorf("failed to publish IssueUpdated event: %w", err)
	}

	e.logger.Printf("Published IssueUpdated event for issue %s (has_new_user=%v, took %v)", issue.ID, hasNewUser, time.Since(start))
	return nil
}

// findSimilarIssue finds issues similar to the given event (unchanged - read-only)
func (e *ErrorGroupingService) findSimilarIssue(ctx context.Context, event *observabilitydomain.ErrorEvent) (*observabilitydomain.Issue, float64, error) {
	since := time.Now().Add(-24 * time.Hour)
	filter := shared.IssueFilter{
		Since: &since,
		Limit: 50,
	}

	issues, err := e.issueStore.ListProjectIssues(ctx, event.ProjectID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list recent issues: %w", err)
	}

	var bestMatch *observabilitydomain.Issue
	var bestScore float64

	for _, issue := range issues {
		if issue.IsResolved() || issue.IsMuted() {
			continue
		}

		score := e.similarity.CalculateSimilarity(event, issue)
		if score > bestScore {
			bestScore = score
			bestMatch = issue
		}
	}

	return bestMatch, bestScore, nil
}

// Fingerprinting methods (unchanged from original)

func (e *ErrorGroupingService) generateAdvancedFingerprint(event *observabilitydomain.ErrorEvent) string {
	return observabilitydomain.GenerateAdvancedFingerprint(event)
}

func (e *ErrorGroupingService) extractFingerprintComponents(event *observabilitydomain.ErrorEvent) *FingerprintComponents {
	return observabilitydomain.ExtractFingerprintComponents(event)
}

func (e *ErrorGroupingService) normalizeErrorMessage(message string) string {
	return observabilitydomain.NormalizeErrorMessage(message)
}

func (e *ErrorGroupingService) hashComponents(components *FingerprintComponents) string {
	return observabilitydomain.HashFingerprintComponents(components)
}

func (e *ErrorGroupingService) generateIssueTitle(event *observabilitydomain.ErrorEvent) string {
	return observabilitydomain.GenerateIssueTitle(event)
}

func (e *ErrorGroupingService) extractCulprit(event *observabilitydomain.ErrorEvent) string {
	return observabilitydomain.ExtractCulprit(event)
}

func (e *ErrorGroupingService) truncateMessage(msg string, maxLen int) string {
	return observabilitydomain.TruncateMessage(msg, maxLen)
}

// SimilarityEngine methods

func (s *SimilarityEngine) CalculateSimilarity(event *observabilitydomain.ErrorEvent, issue *observabilitydomain.Issue) float64 {
	return observabilitydomain.CalculateIssueSimilarity(event, issue)
}

func (s *SimilarityEngine) compareStackTraces(event *observabilitydomain.ErrorEvent, issue *observabilitydomain.Issue) bool {
	return observabilitydomain.CompareIssueStackTrace(event, issue)
}

func replacePattern(text, pattern, replacement string) string {
	return observabilitydomain.ReplacePattern(text, pattern, replacement)
}
