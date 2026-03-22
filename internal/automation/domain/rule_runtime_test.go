package automationdomain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

func TestFieldChangesAccessors(t *testing.T) {
	changes := NewFieldChanges()
	changes.Set("priority", "high")
	changes.SetValue("attempts", shareddomain.IntValue(3))
	changes.SetString("status", "open")

	value, ok := changes.Get("priority")
	require.True(t, ok)
	assert.Equal(t, "high", value.AsString())

	status, ok := changes.GetString("status")
	require.True(t, ok)
	assert.Equal(t, "open", status)

	_, ok = changes.GetString("attempts")
	assert.False(t, ok)

	_, ok = changes.Get("missing")
	assert.False(t, ok)
}

func TestActionChangesConversions(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	changes := NewActionChanges()
	changes.SetString("status", "pending")
	changes.SetInt("attempts", 2)
	changes.SetBool("escalated", true)
	changes.SetTime("handled_at", now)
	changes.SetStrings("tags", []string{"vip", "sla"})
	changes.SetValue("score", shareddomain.FloatValue(9.5))
	changes.Set("subject", "Help needed")

	status, ok := changes.GetString("status")
	require.True(t, ok)
	assert.Equal(t, "pending", status)

	attempts, ok := changes.GetInt("attempts")
	require.True(t, ok)
	assert.Equal(t, 2, attempts)

	escalated, ok := changes.GetBool("escalated")
	require.True(t, ok)
	assert.True(t, escalated)

	handledAt, ok := changes.Get("handled_at")
	require.True(t, ok)
	assert.Equal(t, shareddomain.ValueTypeTime, handledAt.Type())
	assert.Equal(t, now, handledAt.AsTime())

	tags, ok := changes.Get("tags")
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"vip", "sla"}, tags.AsStrings())

	_, ok = changes.GetString("attempts")
	assert.False(t, ok)

	metadata := changes.ToMetadata()
	assert.Equal(t, "pending", metadata.GetString("status"))
	assert.Equal(t, int64(2), metadata.GetInt("attempts"))
	assert.True(t, metadata.GetBool("escalated"))

	changeSet := changes.ToChangeSet()
	assert.True(t, changeSet.HasChanges())

	statusChange, ok := changeSet.GetChange("status")
	require.True(t, ok)
	assert.Equal(t, "pending", statusChange.NewValue.AsString())
}

func TestRuleMetadataToMetadataAndMap(t *testing.T) {
	metadata := NewRuleMetadata()
	metadata.IssueID = "issue_1"
	metadata.IssueTitle = "Crash loop"
	metadata.IssueLevel = "error"
	metadata.IssueStatus = "unresolved"
	metadata.IssueCulprit = "worker.go"
	metadata.IssuePlatform = "go"
	metadata.IssueEventCount = 12
	metadata.IssueUserCount = 3
	metadata.ProjectID = "project_1"
	metadata.FormID = "form_1"
	metadata.FormSlug = "contact"
	metadata.SubmissionID = "sub_1"
	metadata.WorkspaceID = "ws_1"
	metadata.SubmitterEmail = "person@example.com"
	metadata.SubmitterName = "Person"
	metadata.SetExtension("custom_flag", shareddomain.BoolValue(true))
	metadata.SetFormField("severity", "high")

	formValue, ok := metadata.GetFormField("severity")
	require.True(t, ok)
	assert.Equal(t, "high", formValue.AsString())

	extValue, ok := metadata.GetExtension("custom_flag")
	require.True(t, ok)
	assert.True(t, extValue.AsBool())

	typed := metadata.ToMetadata()
	assert.Equal(t, "issue_1", typed.GetString("issue_id"))
	assert.Equal(t, int64(12), typed.GetInt("issue_event_count"))
	assert.True(t, typed.GetBool("custom_flag"))
	assert.Equal(t, "high", typed.GetString("form_severity"))

	asMap := metadata.ToMap()
	assert.Equal(t, "project_1", asMap["project_id"])
	assert.Equal(t, "person@example.com", asMap["submitter_email"])
	assert.Equal(t, true, asMap["custom_flag"])
}

func TestRuleContextHelpers(t *testing.T) {
	var nilContext *RuleContext
	assert.Equal(t, "unknown", nilContext.TargetID())
	assert.Equal(t, "unknown", nilContext.TargetType())
	assert.EqualError(t, nilContext.Validate(), "rule context is nil")
	assert.False(t, nilContext.HasCase())
	assert.False(t, nilContext.HasIssue())
	assert.False(t, nilContext.HasFormSubmission())

	empty := &RuleContext{}
	assert.EqualError(t, empty.Validate(), "rule context has no target (requires case, issue, or form submission)")

	caseObj := servicedomain.NewCase("ws_1", "Subject", "person@example.com")
	caseObj.ID = "case_1"
	caseContext := &RuleContext{Case: caseObj}
	require.NoError(t, caseContext.Validate())
	assert.Equal(t, "case_1", caseContext.TargetID())
	assert.Equal(t, "case", caseContext.TargetType())
	assert.True(t, caseContext.HasCase())

	issueContext := &RuleContext{Issue: &IssueContextData{ID: "issue_1"}}
	require.NoError(t, issueContext.Validate())
	assert.Equal(t, "issue_1", issueContext.TargetID())
	assert.Equal(t, "issue", issueContext.TargetType())
	assert.True(t, issueContext.HasIssue())

	formContext := &RuleContext{FormSubmission: &contracts.FormSubmittedEvent{SubmissionID: "sub_1"}}
	require.NoError(t, formContext.Validate())
	assert.Equal(t, "sub_1", formContext.TargetID())
	assert.Equal(t, "form_submission", formContext.TargetType())
	assert.True(t, formContext.HasFormSubmission())
}
