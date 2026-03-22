package automationdomain

import (
	"fmt"
	"time"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

// FieldChanges tracks what changed to trigger a rule.
type FieldChanges struct {
	data map[string]shareddomain.Value
}

func NewFieldChanges() *FieldChanges {
	return &FieldChanges{data: make(map[string]shareddomain.Value)}
}

func (fc *FieldChanges) Set(key string, value any) {
	fc.data[key] = shareddomain.ValueFromInterface(value)
}

func (fc *FieldChanges) SetValue(key string, value shareddomain.Value) {
	fc.data[key] = value
}

func (fc *FieldChanges) SetString(key, value string) {
	fc.data[key] = shareddomain.StringValue(value)
}

func (fc *FieldChanges) Get(key string) (shareddomain.Value, bool) {
	v, ok := fc.data[key]
	return v, ok
}

func (fc *FieldChanges) GetString(key string) (string, bool) {
	v, ok := fc.data[key]
	if !ok {
		return "", false
	}
	return v.AsString(), v.IsString()
}

// ActionChanges provides type-safe access to action result changes.
type ActionChanges struct {
	data map[string]shareddomain.Value
}

func NewActionChanges() *ActionChanges {
	return &ActionChanges{data: make(map[string]shareddomain.Value)}
}

func (ac *ActionChanges) SetString(key, value string) {
	ac.data[key] = shareddomain.StringValue(value)
}

func (ac *ActionChanges) SetInt(key string, value int) {
	ac.data[key] = shareddomain.IntValue(int64(value))
}

func (ac *ActionChanges) SetBool(key string, value bool) {
	ac.data[key] = shareddomain.BoolValue(value)
}

func (ac *ActionChanges) SetTime(key string, value time.Time) {
	ac.data[key] = shareddomain.TimeValue(value)
}

func (ac *ActionChanges) SetStrings(key string, value []string) {
	ac.data[key] = shareddomain.StringsValue(value)
}

func (ac *ActionChanges) SetValue(key string, value shareddomain.Value) {
	ac.data[key] = value
}

func (ac *ActionChanges) Set(key string, value any) {
	ac.data[key] = shareddomain.ValueFromInterface(value)
}

func (ac *ActionChanges) GetString(key string) (string, bool) {
	v, ok := ac.data[key]
	if !ok {
		return "", false
	}
	return v.AsString(), v.IsString()
}

func (ac *ActionChanges) GetInt(key string) (int, bool) {
	v, ok := ac.data[key]
	if !ok {
		return 0, false
	}
	return int(v.AsInt()), v.IsInt()
}

func (ac *ActionChanges) GetBool(key string) (bool, bool) {
	v, ok := ac.data[key]
	if !ok {
		return false, false
	}
	return v.AsBool(), v.IsBool()
}

func (ac *ActionChanges) Get(key string) (shareddomain.Value, bool) {
	v, ok := ac.data[key]
	return v, ok
}

func (ac *ActionChanges) ToMetadata() shareddomain.Metadata {
	m := shareddomain.NewMetadata()
	for k, v := range ac.data {
		m.Set(k, v)
	}
	return m
}

func (ac *ActionChanges) ToChangeSet() *shareddomain.ChangeSet {
	cs := shareddomain.NewChangeSet()
	for k, v := range ac.data {
		cs.RecordString(k, "", v.AsString())
	}
	return cs
}

// RuleMetadata provides type-safe access to rule context metadata.
type RuleMetadata struct {
	IssueID         string
	IssueTitle      string
	IssueLevel      string
	IssueStatus     string
	IssueCulprit    string
	IssuePlatform   string
	IssueEventCount int64
	IssueUserCount  int64
	ProjectID       string

	FormID         string
	FormSlug       string
	SubmissionID   string
	WorkspaceID    string
	SubmitterEmail string
	SubmitterName  string

	extensions map[string]shareddomain.Value
}

// IssueContextData captures the subset of issue data automation needs.
type IssueContextData struct {
	ID             string
	WorkspaceID    string
	ProjectID      string
	Title          string
	Type           string
	Culprit        string
	Platform       string
	Level          string
	Status         string
	AssignedTo     string
	EventCount     int
	UserCount      int
	HasRelatedCase bool
	RelatedCaseIDs []string
	FirstSeen      time.Time
	LastSeen       time.Time
}

func NewRuleMetadata() *RuleMetadata {
	return &RuleMetadata{extensions: make(map[string]shareddomain.Value)}
}

func (rm *RuleMetadata) SetExtension(key string, value shareddomain.Value) {
	if rm.extensions == nil {
		rm.extensions = make(map[string]shareddomain.Value)
	}
	rm.extensions[key] = value
}

func (rm *RuleMetadata) SetExtensionAny(key string, value any) {
	rm.SetExtension(key, shareddomain.ValueFromInterface(value))
}

func (rm *RuleMetadata) GetExtension(key string) (shareddomain.Value, bool) {
	v, ok := rm.extensions[key]
	return v, ok
}

func (rm *RuleMetadata) SetFormField(key string, value any) {
	rm.SetExtensionAny(fmt.Sprintf("form_%s", key), value)
}

func (rm *RuleMetadata) GetFormField(key string) (shareddomain.Value, bool) {
	return rm.GetExtension(fmt.Sprintf("form_%s", key))
}

func (rm *RuleMetadata) ToMap() map[string]any {
	return rm.ToMetadata().ToInterfaceMap()
}

func (rm *RuleMetadata) ToMetadata() shareddomain.Metadata {
	m := shareddomain.NewMetadata()

	if rm.IssueID != "" {
		m.SetString("issue_id", rm.IssueID)
	}
	if rm.IssueTitle != "" {
		m.SetString("issue_title", rm.IssueTitle)
	}
	if rm.IssueLevel != "" {
		m.SetString("issue_level", rm.IssueLevel)
	}
	if rm.IssueStatus != "" {
		m.SetString("issue_status", rm.IssueStatus)
	}
	if rm.IssueCulprit != "" {
		m.SetString("issue_culprit", rm.IssueCulprit)
	}
	if rm.IssuePlatform != "" {
		m.SetString("issue_platform", rm.IssuePlatform)
	}
	if rm.IssueEventCount > 0 {
		m.SetInt("issue_event_count", rm.IssueEventCount)
	}
	if rm.IssueUserCount > 0 {
		m.SetInt("issue_user_count", rm.IssueUserCount)
	}
	if rm.ProjectID != "" {
		m.SetString("project_id", rm.ProjectID)
	}

	if rm.FormID != "" {
		m.SetString("form_id", rm.FormID)
	}
	if rm.FormSlug != "" {
		m.SetString("form_slug", rm.FormSlug)
	}
	if rm.SubmissionID != "" {
		m.SetString("submission_id", rm.SubmissionID)
	}
	if rm.WorkspaceID != "" {
		m.SetString("workspace_id", rm.WorkspaceID)
	}
	if rm.SubmitterEmail != "" {
		m.SetString("submitter_email", rm.SubmitterEmail)
	}
	if rm.SubmitterName != "" {
		m.SetString("submitter_name", rm.SubmitterName)
	}

	for k, v := range rm.extensions {
		m.Set(k, v)
	}

	return m
}

// RuleContext provides context for rule evaluation.
type RuleContext struct {
	Case           *servicedomain.Case
	Issue          *IssueContextData
	FormSubmission *contracts.FormSubmittedEvent
	Contact        *platformdomain.Contact
	User           *platformdomain.User
	Event          string
	Changes        *FieldChanges
	Metadata       *RuleMetadata
	RuleID         string
}

func (rc *RuleContext) TargetID() string {
	if rc == nil {
		return "unknown"
	}
	if rc.Case != nil {
		return rc.Case.ID
	}
	if rc.Issue != nil {
		return rc.Issue.ID
	}
	if rc.FormSubmission != nil {
		return rc.FormSubmission.SubmissionID
	}
	return "unknown"
}

func (rc *RuleContext) TargetType() string {
	if rc == nil {
		return "unknown"
	}
	if rc.Case != nil {
		return "case"
	}
	if rc.Issue != nil {
		return "issue"
	}
	if rc.FormSubmission != nil {
		return "form_submission"
	}
	return "unknown"
}

func (rc *RuleContext) Validate() error {
	if rc == nil {
		return fmt.Errorf("rule context is nil")
	}
	if rc.Case == nil && rc.Issue == nil && rc.FormSubmission == nil {
		return fmt.Errorf("rule context has no target (requires case, issue, or form submission)")
	}
	return nil
}

func (rc *RuleContext) HasCase() bool {
	return rc != nil && rc.Case != nil
}

func (rc *RuleContext) HasIssue() bool {
	return rc != nil && rc.Issue != nil
}

func (rc *RuleContext) HasFormSubmission() bool {
	return rc != nil && rc.FormSubmission != nil
}
