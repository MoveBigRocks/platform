package shareddomain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

func TestCaseEventConstructorsProduceValidEvents(t *testing.T) {
	t.Parallel()

	created := NewCaseCreatedEvent(
		"case_1",
		"ws_1",
		"contact_1",
		"user@example.com",
		"Outage",
		"Customer reported outage",
		CasePriorityHigh,
		CaseChannelEmail,
		"automation",
		false,
		true,
	)
	if created.EventType != eventbus.TypeCaseCreated || created.EventID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("unexpected created event: %#v", created)
	}
	if err := created.Validate(); err != nil {
		t.Fatalf("expected created event to validate, got %v", err)
	}

	assigned := NewCaseAssignedEvent("case_1", "ws_1", "user_2", "user_1", "team_1")
	if assigned.EventType != eventbus.TypeCaseAssigned || assigned.AssignedAt.IsZero() {
		t.Fatalf("unexpected assigned event: %#v", assigned)
	}
	if err := assigned.Validate(); err != nil {
		t.Fatalf("expected assigned event to validate, got %v", err)
	}

	changed := NewCaseStatusChangedEvent("case_1", "ws_1", CaseStatusOpen, CaseStatusPending, "waiting", "user_1")
	if changed.EventType != eventbus.TypeCaseStatusChanged || changed.ChangedAt.IsZero() {
		t.Fatalf("unexpected status changed event: %#v", changed)
	}
	if err := changed.Validate(); err != nil {
		t.Fatalf("expected status changed event to validate, got %v", err)
	}

	resolved := NewCaseResolvedEvent("case_1", "ws_1", "fixed", "user_1", 300)
	if resolved.EventType != eventbus.TypeCaseResolved || resolved.ResolvedAt.IsZero() {
		t.Fatalf("unexpected resolved event: %#v", resolved)
	}
	if err := resolved.Validate(); err != nil {
		t.Fatalf("expected resolved event to validate, got %v", err)
	}
}

func TestAdditionalCaseJobAndIntegrationValidation(t *testing.T) {
	t.Parallel()

	validAssigned := CaseAssigned{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseAssigned),
		CaseID:      "case_1",
		WorkspaceID: "ws_1",
		AssignedTo:  "user_2",
		AssignedBy:  "user_1",
		AssignedAt:  time.Now(),
	}
	if err := validAssigned.Validate(); err != nil {
		t.Fatalf("expected assigned event to validate, got %v", err)
	}
	invalidAssigned := validAssigned
	invalidAssigned.AssignedAt = time.Time{}
	if err := invalidAssigned.Validate(); err == nil || !strings.Contains(err.Error(), "assigned_at") {
		t.Fatalf("expected assigned_at validation error, got %v", err)
	}

	jobEvents := []struct {
		name      string
		validate  func() error
		errSubstr string
	}{
		{
			name: "job enqueued valid",
			validate: func() error {
				return JobEnqueued{
					BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:       "job_1",
					JobType:     "sync",
					WorkspaceID: "ws_1",
					EnqueuedAt:  time.Now(),
				}.Validate()
			},
		},
		{
			name: "job enqueued missing workspace",
			validate: func() error {
				return JobEnqueued{
					BaseEvent:  eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:      "job_1",
					JobType:    "sync",
					EnqueuedAt: time.Now(),
				}.Validate()
			},
			errSubstr: "workspace_id",
		},
		{
			name: "job started valid",
			validate: func() error {
				return JobStarted{
					BaseEvent: eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:     "job_1",
					JobType:   "sync",
					StartedAt: time.Now(),
				}.Validate()
			},
		},
		{
			name: "job started missing started_at",
			validate: func() error {
				return JobStarted{
					BaseEvent: eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:     "job_1",
					JobType:   "sync",
				}.Validate()
			},
			errSubstr: "started_at",
		},
		{
			name: "job completed negative duration",
			validate: func() error {
				return JobCompleted{
					BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:       "job_1",
					JobType:     "sync",
					CompletedAt: time.Now(),
					Duration:    -1,
				}.Validate()
			},
			errSubstr: "duration",
		},
		{
			name: "job completed valid",
			validate: func() error {
				return JobCompleted{
					BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:       "job_1",
					JobType:     "sync",
					CompletedAt: time.Now(),
					Duration:    5,
				}.Validate()
			},
		},
		{
			name: "job failed missing error",
			validate: func() error {
				return JobFailed{
					BaseEvent: eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:     "job_1",
					JobType:   "sync",
					FailedAt:  time.Now(),
				}.Validate()
			},
			errSubstr: "error",
		},
		{
			name: "job failed valid",
			validate: func() error {
				return JobFailed{
					BaseEvent: eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:     "job_1",
					JobType:   "sync",
					Error:     "boom",
					FailedAt:  time.Now(),
				}.Validate()
			},
		},
		{
			name: "job retrying invalid attempt",
			validate: func() error {
				return JobRetrying{
					BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:       "job_1",
					JobType:     "sync",
					Attempt:     0,
					NextRetryAt: time.Now(),
				}.Validate()
			},
			errSubstr: "attempt",
		},
		{
			name: "job retrying valid",
			validate: func() error {
				return JobRetrying{
					BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:       "job_1",
					JobType:     "sync",
					Attempt:     1,
					NextRetryAt: time.Now(),
				}.Validate()
			},
		},
		{
			name: "job canceled missing canceled_at",
			validate: func() error {
				return JobCanceled{
					BaseEvent:  eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:      "job_1",
					JobType:    "sync",
					CanceledBy: "user_1",
				}.Validate()
			},
			errSubstr: "canceled_at",
		},
		{
			name: "job canceled valid",
			validate: func() error {
				return JobCanceled{
					BaseEvent:  eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
					JobID:      "job_1",
					JobType:    "sync",
					CanceledBy: "user_1",
					CanceledAt: time.Now(),
				}.Validate()
			},
		},
	}

	for _, tt := range jobEvents {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validate()
			if tt.errSubstr == "" {
				if err != nil {
					t.Fatalf("unexpected job validation error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.errSubstr) {
				t.Fatalf("expected %q error, got %v", tt.errSubstr, err)
			}
		})
	}

	linked := IssueCaseLinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseLinked),
		IssueID:     "issue_1",
		CaseID:      "case_1",
		ProjectID:   "project_1",
		WorkspaceID: "ws_1",
		LinkedBy:    "user_1",
		LinkedAt:    time.Now(),
	}
	if err := linked.Validate(); err != nil {
		t.Fatalf("expected link event to validate, got %v", err)
	}
	invalidLinked := linked
	invalidLinked.LinkedBy = ""
	if err := invalidLinked.Validate(); err == nil || !strings.Contains(err.Error(), "linked_by") {
		t.Fatalf("expected invalid linked_by error, got %v", err)
	}
	unlinked := IssueCaseUnlinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseUnlinked),
		IssueID:     "issue_1",
		CaseID:      "case_1",
		ProjectID:   "project_1",
		WorkspaceID: "ws_1",
		UnlinkedBy:  "user_1",
		UnlinkedAt:  time.Now(),
	}
	if err := unlinked.Validate(); err != nil {
		t.Fatalf("expected unlink event to validate, got %v", err)
	}
	invalidUnlinked := unlinked
	invalidUnlinked.UnlinkedAt = time.Time{}
	if err := invalidUnlinked.Validate(); err == nil || !strings.Contains(err.Error(), "unlinked_at") {
		t.Fatalf("expected invalid unlinked_at error, got %v", err)
	}
	createdForContact := CaseCreatedForContact{
		BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCaseCreatedForContact),
		ContactID:    "contact_1",
		ContactEmail: "user@example.com",
		IssueID:      "issue_1",
		WorkspaceID:  "ws_1",
		CreatedAt:    time.Now(),
	}
	if err := createdForContact.Validate(); err != nil {
		t.Fatalf("expected contact case event to validate, got %v", err)
	}
	bulkResolved := CasesBulkResolved{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCasesBulkResolved),
		IssueID:     "issue_1",
		WorkspaceID: "ws_1",
		Resolution:  "resolved",
		ResolvedAt:  time.Now(),
	}
	if err := bulkResolved.Validate(); err != nil {
		t.Fatalf("expected bulk resolved event to validate, got %v", err)
	}
	invalidBulkResolved := bulkResolved
	invalidBulkResolved.Resolution = ""
	if err := invalidBulkResolved.Validate(); err == nil || !strings.Contains(err.Error(), "resolution") {
		t.Fatalf("expected invalid resolution error, got %v", err)
	}

	invalidContactEvent := createdForContact
	invalidContactEvent.ContactEmail = "invalid-email"
	if err := invalidContactEvent.Validate(); err == nil || !strings.Contains(err.Error(), "contact_email") {
		t.Fatalf("expected invalid contact_email error, got %v", err)
	}

	emailContact := ContactCreatedFromEmail{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeSendEmailRequested),
		ContactID:   "contact_1",
		WorkspaceID: "ws_1",
		EmailID:     "email_1",
		Email:       "person@example.com",
		Name:        "Person",
		CreatedAt:   time.Now(),
	}
	if err := emailContact.Validate(); err != nil {
		t.Fatalf("expected email contact event to validate, got %v", err)
	}
	emailContact.CreatedAt = time.Time{}
	if err := emailContact.Validate(); err == nil || !strings.Contains(err.Error(), "created_at") {
		t.Fatalf("expected created_at validation error, got %v", err)
	}
}

func TestValidationHelpersAdditionalCoverage(t *testing.T) {
	t.Parallel()

	if err := validateEmail("email", "ops@example.com"); err != nil {
		t.Fatalf("expected valid email, got %v", err)
	}
	if err := validateEmail("email", ""); err != nil {
		t.Fatalf("expected empty optional email to pass, got %v", err)
	}
	if err := validateEmail("email", strings.Repeat("a", 255)+"@example.com"); err == nil {
		t.Fatal("expected overlong email to fail")
	}
	if err := validateEmail("email", "bad-email"); err == nil {
		t.Fatal("expected malformed email to fail")
	}
	if err := validateEmailRequired("email", ""); err == nil {
		t.Fatal("expected required email to fail when empty")
	}
	if err := validatePositiveInt("attempt", 1); err != nil {
		t.Fatalf("expected positive int to pass, got %v", err)
	}
	if err := validatePositiveInt("attempt", 0); err == nil {
		t.Fatal("expected non-positive int to fail")
	}

	meta := NewMetadata()
	meta.SetString("source", "automation")
	if _, err := json.Marshal(meta); err != nil {
		t.Fatalf("expected metadata to marshal, got %v", err)
	}
}
