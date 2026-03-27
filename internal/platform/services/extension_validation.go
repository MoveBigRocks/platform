package platformservices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

type ExtensionSeededResourcesReport struct {
	Queues          []ExtensionSeededQueueState
	Forms           []ExtensionSeededFormState
	AutomationRules []ExtensionSeededAutomationRuleState
}

type ExtensionSeededQueueState struct {
	Slug        string
	ResourceID  string
	Exists      bool
	MatchesSeed bool
	Problems    []string
	Expected    map[string]any
	Actual      map[string]any
}

type ExtensionSeededFormState struct {
	Slug        string
	ResourceID  string
	Exists      bool
	MatchesSeed bool
	Problems    []string
	Expected    map[string]any
	Actual      map[string]any
}

type ExtensionSeededAutomationRuleState struct {
	Key         string
	ResourceID  string
	Exists      bool
	MatchesSeed bool
	Problems    []string
	Expected    map[string]any
	Actual      map[string]any
}

func (s *ExtensionService) ValidateManifestSource(ctx context.Context, workspaceID string, manifest platformdomain.ExtensionManifest, assets []ExtensionAssetInput) (bool, string, error) {
	extension := &platformdomain.InstalledExtension{
		ID:          "ext_source_validation",
		WorkspaceID: strings.TrimSpace(workspaceID),
		Slug:        manifest.Slug,
		Name:        manifest.Name,
		Publisher:   manifest.Publisher,
		Version:     manifest.Version,
		Description: manifest.Description,
		Manifest:    manifest,
	}

	installedAssets := make([]*platformdomain.ExtensionAsset, 0, len(assets))
	for _, asset := range assets {
		installedAsset, err := platformdomain.NewExtensionAsset(extension.ID, asset.Path, asset.ContentType, asset.Content, asset.IsCustomizable)
		if err != nil {
			return false, "", apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid extension asset")
		}
		installedAssets = append(installedAssets, installedAsset)
	}

	valid, message := s.validateInstallation(ctx, extension, installedAssets)
	return valid, message, nil
}

func (s *ExtensionService) ListExtensionResolvedAdminNavigation(ctx context.Context, extensionID string) ([]ResolvedExtensionAdminNavigationItem, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	return buildResolvedAdminNavigation([]*platformdomain.InstalledExtension{extension}, false), nil
}

func (s *ExtensionService) ListExtensionResolvedDashboardWidgets(ctx context.Context, extensionID string) ([]ResolvedExtensionDashboardWidget, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	return buildResolvedDashboardWidgets([]*platformdomain.InstalledExtension{extension}, false), nil
}

func (s *ExtensionService) InspectExtensionSeededResources(ctx context.Context, extensionID string) (ExtensionSeededResourcesReport, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return ExtensionSeededResourcesReport{}, err
	}
	return s.inspectExtensionSeededResources(ctx, extension)
}

func buildSeededQueue(extension *platformdomain.InstalledExtension, seed platformdomain.ExtensionQueueSeed) (*servicedomain.Queue, error) {
	queue := servicedomain.NewQueue(extension.WorkspaceID, seed.Name, seed.Slug, seed.Description)
	if err := queue.Validate(); err != nil {
		return nil, err
	}
	return queue, nil
}

func buildSeededForm(extension *platformdomain.InstalledExtension, seed platformdomain.ExtensionFormSeed) *servicedomain.FormSchema {
	form := servicedomain.NewFormSchema(extension.WorkspaceID, seed.Name, seed.Slug, seededBy(extension))
	form.Description = seed.Description
	if seed.Status != "" {
		form.Status = servicedomain.FormStatus(seed.Status)
	}
	form.IsPublic = seed.IsPublic
	form.RequiresAuth = seed.RequiresAuth
	form.AllowMultiple = seed.AllowMultiple
	form.CollectEmail = seed.CollectEmail
	form.AutoCreateCase = seed.AutoCreateCase
	form.AutoCasePriority = seed.AutoCasePriority
	form.AutoCaseType = seed.AutoCaseType
	form.AutoTags = append([]string{}, seed.AutoTags...)
	form.SubmissionMessage = seed.SubmissionMessage
	form.RedirectURL = seed.RedirectURL
	form.Theme = seed.Theme
	form.SchemaData = seed.Schema.Clone()
	form.UISchema = seed.UISchema.Clone()
	form.ValidationRules = seed.ValidationRules.Clone()
	return form
}

func buildSeededAutomationRule(extension *platformdomain.InstalledExtension, seed platformdomain.ExtensionAutomationSeed) (*automationdomain.Rule, error) {
	conditions, err := decodeTypedConditions(seed.Conditions)
	if err != nil {
		return nil, err
	}
	actions, err := decodeTypedActions(seed.Actions)
	if err != nil {
		return nil, err
	}

	rule := automationdomain.NewRule(extension.WorkspaceID, seed.Title, seededBy(extension))
	rule.Description = seed.Description
	rule.IsActive = seed.IsActive
	rule.IsSystem = true
	rule.SystemRuleKey = seed.Key
	rule.Priority = seed.Priority
	rule.MaxExecutionsPerHour = seed.MaxExecutionsPerHour
	rule.MaxExecutionsPerDay = seed.MaxExecutionsPerDay
	rule.Conditions = conditions
	rule.Actions = actions
	return rule, nil
}

func (s *ExtensionService) inspectExtensionSeededResources(ctx context.Context, extension *platformdomain.InstalledExtension) (ExtensionSeededResourcesReport, error) {
	queues, err := s.inspectSeededQueues(ctx, extension)
	if err != nil {
		return ExtensionSeededResourcesReport{}, err
	}
	forms, err := s.inspectSeededForms(ctx, extension)
	if err != nil {
		return ExtensionSeededResourcesReport{}, err
	}
	rules, err := s.inspectSeededAutomationRules(ctx, extension)
	if err != nil {
		return ExtensionSeededResourcesReport{}, err
	}
	return ExtensionSeededResourcesReport{
		Queues:          queues,
		Forms:           forms,
		AutomationRules: rules,
	}, nil
}

func (s *ExtensionService) inspectSeededQueues(ctx context.Context, extension *platformdomain.InstalledExtension) ([]ExtensionSeededQueueState, error) {
	states := make([]ExtensionSeededQueueState, 0, len(extension.Manifest.Queues))
	if len(extension.Manifest.Queues) == 0 {
		return states, nil
	}
	if strings.TrimSpace(extension.WorkspaceID) == "" {
		return unavailableSeededQueues(extension, "workspace-scoped seeded queues are unavailable for instance-scoped extensions"), nil
	}
	if s.queueStore == nil {
		return unavailableSeededQueues(extension, "queue store not configured"), nil
	}

	for _, seed := range extension.Manifest.Queues {
		expected, err := buildSeededQueue(extension, seed)
		if err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid seeded queue")
		}

		state := ExtensionSeededQueueState{
			Slug:     expected.Slug,
			Problems: []string{},
			Expected: seededQueueMap(expected),
		}
		queue, err := s.queueStore.GetQueueBySlug(ctx, extension.WorkspaceID, expected.Slug)
		switch {
		case err == nil:
			state.ResourceID = queue.ID
			state.Exists = true
			state.Actual = seededQueueMap(queue)
			if queue.Name != expected.Name {
				state.Problems = append(state.Problems, fmt.Sprintf("name mismatch: expected %q, got %q", expected.Name, queue.Name))
			}
			if queue.Description != expected.Description {
				state.Problems = append(state.Problems, fmt.Sprintf("description mismatch: expected %q, got %q", expected.Description, queue.Description))
			}
		case errors.Is(err, shared.ErrNotFound):
			state.Problems = append(state.Problems, "queue not found")
		default:
			return nil, apierrors.DatabaseError("lookup seeded queue", err)
		}
		state.MatchesSeed = state.Exists && len(state.Problems) == 0
		states = append(states, state)
	}
	return states, nil
}

func (s *ExtensionService) inspectSeededForms(ctx context.Context, extension *platformdomain.InstalledExtension) ([]ExtensionSeededFormState, error) {
	states := make([]ExtensionSeededFormState, 0, len(extension.Manifest.Forms))
	if len(extension.Manifest.Forms) == 0 {
		return states, nil
	}
	if strings.TrimSpace(extension.WorkspaceID) == "" {
		return unavailableSeededForms(extension, "workspace-scoped seeded forms are unavailable for instance-scoped extensions"), nil
	}
	if s.formStore == nil {
		return unavailableSeededForms(extension, "form store not configured"), nil
	}

	for _, seed := range extension.Manifest.Forms {
		expected := buildSeededForm(extension, seed)
		state := ExtensionSeededFormState{
			Slug:     expected.Slug,
			Problems: []string{},
			Expected: seededFormMap(expected),
		}
		form, err := s.formStore.GetFormBySlug(ctx, extension.WorkspaceID, expected.Slug)
		switch {
		case err == nil:
			state.ResourceID = form.ID
			state.Exists = true
			state.Actual = seededFormMap(form)
			state.Problems = append(state.Problems, compareSeededForm(expected, form)...)
		case errors.Is(err, shared.ErrNotFound):
			state.Problems = append(state.Problems, "form not found")
		default:
			return nil, apierrors.DatabaseError("lookup seeded form", err)
		}
		state.MatchesSeed = state.Exists && len(state.Problems) == 0
		states = append(states, state)
	}
	return states, nil
}

func (s *ExtensionService) inspectSeededAutomationRules(ctx context.Context, extension *platformdomain.InstalledExtension) ([]ExtensionSeededAutomationRuleState, error) {
	states := make([]ExtensionSeededAutomationRuleState, 0, len(extension.Manifest.AutomationRules))
	if len(extension.Manifest.AutomationRules) == 0 {
		return states, nil
	}
	if strings.TrimSpace(extension.WorkspaceID) == "" {
		return unavailableSeededAutomationRules(extension, "workspace-scoped seeded automation rules are unavailable for instance-scoped extensions"), nil
	}
	if s.ruleStore == nil {
		return unavailableSeededAutomationRules(extension, "rule store not configured"), nil
	}

	rules, err := s.ruleStore.ListWorkspaceRules(ctx, extension.WorkspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list workspace rules", err)
	}
	rulesByKey := make(map[string]*automationdomain.Rule, len(rules))
	for _, rule := range rules {
		if strings.TrimSpace(rule.SystemRuleKey) == "" {
			continue
		}
		rulesByKey[rule.SystemRuleKey] = rule
	}

	for _, seed := range extension.Manifest.AutomationRules {
		expected, err := buildSeededAutomationRule(extension, seed)
		if err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid seeded automation rule")
		}

		state := ExtensionSeededAutomationRuleState{
			Key:      expected.SystemRuleKey,
			Problems: []string{},
			Expected: seededAutomationRuleMap(expected),
		}
		rule, ok := rulesByKey[expected.SystemRuleKey]
		if !ok {
			state.Problems = append(state.Problems, "automation rule not found")
			states = append(states, state)
			continue
		}

		state.ResourceID = rule.ID
		state.Exists = true
		state.Actual = seededAutomationRuleMap(rule)
		state.Problems = append(state.Problems, compareSeededAutomationRule(expected, rule)...)
		state.MatchesSeed = len(state.Problems) == 0
		states = append(states, state)
	}
	return states, nil
}

func unavailableSeededQueues(extension *platformdomain.InstalledExtension, problem string) []ExtensionSeededQueueState {
	states := make([]ExtensionSeededQueueState, 0, len(extension.Manifest.Queues))
	for _, seed := range extension.Manifest.Queues {
		expected, err := buildSeededQueue(extension, seed)
		if err != nil {
			continue
		}
		states = append(states, ExtensionSeededQueueState{
			Slug:        expected.Slug,
			Exists:      false,
			MatchesSeed: false,
			Problems:    []string{problem},
			Expected:    seededQueueMap(expected),
		})
	}
	return states
}

func unavailableSeededForms(extension *platformdomain.InstalledExtension, problem string) []ExtensionSeededFormState {
	states := make([]ExtensionSeededFormState, 0, len(extension.Manifest.Forms))
	for _, seed := range extension.Manifest.Forms {
		expected := buildSeededForm(extension, seed)
		states = append(states, ExtensionSeededFormState{
			Slug:        expected.Slug,
			Exists:      false,
			MatchesSeed: false,
			Problems:    []string{problem},
			Expected:    seededFormMap(expected),
		})
	}
	return states
}

func unavailableSeededAutomationRules(extension *platformdomain.InstalledExtension, problem string) []ExtensionSeededAutomationRuleState {
	states := make([]ExtensionSeededAutomationRuleState, 0, len(extension.Manifest.AutomationRules))
	for _, seed := range extension.Manifest.AutomationRules {
		expected, err := buildSeededAutomationRule(extension, seed)
		if err != nil {
			continue
		}
		states = append(states, ExtensionSeededAutomationRuleState{
			Key:         expected.SystemRuleKey,
			Exists:      false,
			MatchesSeed: false,
			Problems:    []string{problem},
			Expected:    seededAutomationRuleMap(expected),
		})
	}
	return states
}

func compareSeededForm(expected, actual *servicedomain.FormSchema) []string {
	problems := []string{}
	if actual.Name != expected.Name {
		problems = append(problems, fmt.Sprintf("name mismatch: expected %q, got %q", expected.Name, actual.Name))
	}
	if actual.Description != expected.Description {
		problems = append(problems, fmt.Sprintf("description mismatch: expected %q, got %q", expected.Description, actual.Description))
	}
	if actual.Status != expected.Status {
		problems = append(problems, fmt.Sprintf("status mismatch: expected %q, got %q", expected.Status, actual.Status))
	}
	if actual.IsPublic != expected.IsPublic {
		problems = append(problems, fmt.Sprintf("isPublic mismatch: expected %t, got %t", expected.IsPublic, actual.IsPublic))
	}
	if actual.RequiresAuth != expected.RequiresAuth {
		problems = append(problems, fmt.Sprintf("requiresAuth mismatch: expected %t, got %t", expected.RequiresAuth, actual.RequiresAuth))
	}
	if actual.AllowMultiple != expected.AllowMultiple {
		problems = append(problems, fmt.Sprintf("allowMultiple mismatch: expected %t, got %t", expected.AllowMultiple, actual.AllowMultiple))
	}
	if actual.CollectEmail != expected.CollectEmail {
		problems = append(problems, fmt.Sprintf("collectEmail mismatch: expected %t, got %t", expected.CollectEmail, actual.CollectEmail))
	}
	if actual.AutoCreateCase != expected.AutoCreateCase {
		problems = append(problems, fmt.Sprintf("autoCreateCase mismatch: expected %t, got %t", expected.AutoCreateCase, actual.AutoCreateCase))
	}
	if actual.AutoCasePriority != expected.AutoCasePriority {
		problems = append(problems, fmt.Sprintf("autoCasePriority mismatch: expected %q, got %q", expected.AutoCasePriority, actual.AutoCasePriority))
	}
	if actual.AutoCaseType != expected.AutoCaseType {
		problems = append(problems, fmt.Sprintf("autoCaseType mismatch: expected %q, got %q", expected.AutoCaseType, actual.AutoCaseType))
	}
	if !sameStringsSet(expected.AutoTags, actual.AutoTags) {
		problems = append(problems, fmt.Sprintf("autoTags mismatch: expected %v, got %v", expected.AutoTags, actual.AutoTags))
	}
	if actual.SubmissionMessage != expected.SubmissionMessage {
		problems = append(problems, fmt.Sprintf("submissionMessage mismatch: expected %q, got %q", expected.SubmissionMessage, actual.SubmissionMessage))
	}
	if actual.RedirectURL != expected.RedirectURL {
		problems = append(problems, fmt.Sprintf("redirectURL mismatch: expected %q, got %q", expected.RedirectURL, actual.RedirectURL))
	}
	if actual.Theme != expected.Theme {
		problems = append(problems, fmt.Sprintf("theme mismatch: expected %q, got %q", expected.Theme, actual.Theme))
	}
	if !jsonEquivalent(expected.SchemaData.ToMap(), actual.SchemaData.ToMap()) {
		problems = append(problems, "schema mismatch")
	}
	if !jsonEquivalent(expected.UISchema.ToMap(), actual.UISchema.ToMap()) {
		problems = append(problems, "ui schema mismatch")
	}
	if !jsonEquivalent(expected.ValidationRules.ToMap(), actual.ValidationRules.ToMap()) {
		problems = append(problems, "validation rules mismatch")
	}
	return problems
}

func compareSeededAutomationRule(expected, actual *automationdomain.Rule) []string {
	problems := []string{}
	if actual.Title != expected.Title {
		problems = append(problems, fmt.Sprintf("title mismatch: expected %q, got %q", expected.Title, actual.Title))
	}
	if actual.Description != expected.Description {
		problems = append(problems, fmt.Sprintf("description mismatch: expected %q, got %q", expected.Description, actual.Description))
	}
	if actual.IsActive != expected.IsActive {
		problems = append(problems, fmt.Sprintf("isActive mismatch: expected %t, got %t", expected.IsActive, actual.IsActive))
	}
	if actual.IsSystem != expected.IsSystem {
		problems = append(problems, fmt.Sprintf("isSystem mismatch: expected %t, got %t", expected.IsSystem, actual.IsSystem))
	}
	if actual.Priority != expected.Priority {
		problems = append(problems, fmt.Sprintf("priority mismatch: expected %d, got %d", expected.Priority, actual.Priority))
	}
	if actual.MaxExecutionsPerHour != expected.MaxExecutionsPerHour {
		problems = append(problems, fmt.Sprintf("maxExecutionsPerHour mismatch: expected %d, got %d", expected.MaxExecutionsPerHour, actual.MaxExecutionsPerHour))
	}
	if actual.MaxExecutionsPerDay != expected.MaxExecutionsPerDay {
		problems = append(problems, fmt.Sprintf("maxExecutionsPerDay mismatch: expected %d, got %d", expected.MaxExecutionsPerDay, actual.MaxExecutionsPerDay))
	}
	if !jsonEquivalent(seededAutomationConditionsMap(expected.Conditions), seededAutomationConditionsMap(actual.Conditions)) {
		problems = append(problems, "conditions mismatch")
	}
	if !jsonEquivalent(seededAutomationActionsMap(expected.Actions), seededAutomationActionsMap(actual.Actions)) {
		problems = append(problems, "actions mismatch")
	}
	return problems
}

func seededQueueMap(queue *servicedomain.Queue) map[string]any {
	return map[string]any{
		"slug":        queue.Slug,
		"name":        queue.Name,
		"description": queue.Description,
	}
}

func seededFormMap(form *servicedomain.FormSchema) map[string]any {
	return map[string]any{
		"slug":              form.Slug,
		"name":              form.Name,
		"description":       form.Description,
		"status":            string(form.Status),
		"isPublic":          form.IsPublic,
		"requiresAuth":      form.RequiresAuth,
		"allowMultiple":     form.AllowMultiple,
		"collectEmail":      form.CollectEmail,
		"autoCreateCase":    form.AutoCreateCase,
		"autoCasePriority":  form.AutoCasePriority,
		"autoCaseType":      form.AutoCaseType,
		"autoTags":          append([]string{}, form.AutoTags...),
		"submissionMessage": form.SubmissionMessage,
		"redirectURL":       form.RedirectURL,
		"theme":             form.Theme,
		"schema":            form.SchemaData.ToMap(),
		"uiSchema":          form.UISchema.ToMap(),
		"validationRules":   form.ValidationRules.ToMap(),
	}
}

func seededAutomationRuleMap(rule *automationdomain.Rule) map[string]any {
	return map[string]any{
		"key":                  rule.SystemRuleKey,
		"title":                rule.Title,
		"description":          rule.Description,
		"isActive":             rule.IsActive,
		"isSystem":             rule.IsSystem,
		"priority":             rule.Priority,
		"maxExecutionsPerHour": rule.MaxExecutionsPerHour,
		"maxExecutionsPerDay":  rule.MaxExecutionsPerDay,
		"conditions":           seededAutomationConditionsMap(rule.Conditions),
		"actions":              seededAutomationActionsMap(rule.Actions),
	}
}

func seededAutomationConditionsMap(conditions automationdomain.TypedConditions) map[string]any {
	out := map[string]any{}
	if conditions.Operator != "" {
		out["operator"] = string(conditions.Operator)
	}
	items := make([]any, 0, len(conditions.Conditions))
	for _, condition := range conditions.Conditions {
		entry := map[string]any{
			"type": condition.Type,
		}
		if condition.Field != "" {
			entry["field"] = condition.Field
		}
		if condition.Operator != "" {
			entry["operator"] = condition.Operator
		}
		if !condition.Value.IsZero() {
			entry["value"] = condition.Value.ToInterface()
		}
		if !condition.Options.IsEmpty() {
			entry["options"] = condition.Options.ToInterfaceMap()
		}
		items = append(items, entry)
	}
	if len(items) > 0 {
		out["conditions"] = items
	}
	return out
}

func seededAutomationActionsMap(actions automationdomain.TypedActions) map[string]any {
	out := map[string]any{}
	items := make([]any, 0, len(actions.Actions))
	for _, action := range actions.Actions {
		entry := map[string]any{
			"type": action.Type,
		}
		if action.Target != "" {
			entry["target"] = action.Target
		}
		if action.Field != "" {
			entry["field"] = action.Field
		}
		if !action.Value.IsZero() {
			entry["value"] = action.Value.ToInterface()
		}
		if !action.Options.IsEmpty() {
			entry["options"] = action.Options.ToInterfaceMap()
		}
		items = append(items, entry)
	}
	if len(items) > 0 {
		out["actions"] = items
	}
	return out
}

func sameStringsSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftSorted := append([]string{}, left...)
	rightSorted := append([]string{}, right...)
	sort.Strings(leftSorted)
	sort.Strings(rightSorted)
	return reflect.DeepEqual(leftSorted, rightSorted)
}

func jsonEquivalent(expected, actual any) bool {
	return reflect.DeepEqual(normalizeJSONValue(expected), normalizeJSONValue(actual))
}

func normalizeJSONValue(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return value
	}
	return normalized
}
