package servicedomain

import (
	"strings"
	"testing"
	"time"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"

	"github.com/stretchr/testify/assert"
)

// TestNewCase verifies case constructor creates valid instances
func TestNewCase(t *testing.T) {
	workspaceID := "ws-123"
	subject := "Test Issue"
	contactEmail := "user@example.com"

	c := NewCase(workspaceID, subject, contactEmail)

	assert.Empty(t, c.ID, "PostgreSQL should generate the case row ID on insert")
	assert.Equal(t, workspaceID, c.WorkspaceID)
	assert.Equal(t, subject, c.Subject)
	assert.Equal(t, contactEmail, c.ContactEmail)
	assert.Equal(t, CaseStatusNew, c.Status)
	assert.Equal(t, CasePriorityMedium, c.Priority)
	assert.Equal(t, CaseChannelWeb, c.Channel)
	assert.NotNil(t, c.Tags)
	assert.NotNil(t, c.CustomFields)
	assert.False(t, c.CreatedAt.IsZero())
	assert.False(t, c.UpdatedAt.IsZero())
}

// TestGenerateHumanID verifies human-readable ID generation
func TestGenerateHumanID(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "Standard prefix",
			prefix: "ACME",
		},
		{
			name:   "Short prefix",
			prefix: "TP",
		},
		{
			name:   "Lowercase prefix",
			prefix: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Case{}
			c.GenerateHumanID(tt.prefix)

			// Verify format: prefix-yymm-random (e.g., ac-2512-a3e9ef)
			assert.NotEmpty(t, c.HumanID)
			parts := strings.Split(c.HumanID, "-")
			assert.Equal(t, 3, len(parts), "HumanID should have 3 parts: prefix-yymm-random")

			// Verify prefix is lowercase
			assert.Equal(t, strings.ToLower(tt.prefix), parts[0])

			// Verify YYMM format (4 digits)
			assert.Len(t, parts[1], 4, "YYMM should be 4 characters")

			// Verify random component (6 characters, base58)
			assert.Len(t, parts[2], 6, "Random component should be 6 characters")
		})
	}
}

// TestIsOverdue verifies SLA violation detection
func TestIsOverdue(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		caseObj  *Case
		expected bool
	}{
		{
			name: "Response overdue - no first response",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResponseDueAt:   ptr(now.Add(-1 * time.Hour)),
					FirstResponseAt: nil,
				},
			},
			expected: true,
		},
		{
			name: "Response not overdue - has first response",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResponseDueAt:   ptr(now.Add(-1 * time.Hour)),
					FirstResponseAt: ptr(now.Add(-2 * time.Hour)),
				},
			},
			expected: false,
		},
		{
			name: "Response not overdue - future due date",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResponseDueAt:   ptr(now.Add(1 * time.Hour)),
					FirstResponseAt: nil,
				},
			},
			expected: false,
		},
		{
			name: "Resolution overdue - no resolution",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResolutionDueAt: ptr(now.Add(-1 * time.Hour)),
					ResolvedAt:      nil,
				},
			},
			expected: true,
		},
		{
			name: "Resolution not overdue - has resolution",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResolutionDueAt: ptr(now.Add(-1 * time.Hour)),
					ResolvedAt:      ptr(now.Add(-2 * time.Hour)),
				},
			},
			expected: false,
		},
		{
			name: "Resolution not overdue - future due date",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResolutionDueAt: ptr(now.Add(1 * time.Hour)),
					ResolvedAt:      nil,
				},
			},
			expected: false,
		},
		{
			name: "Not overdue - no SLA set",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResponseDueAt:   nil,
					ResolutionDueAt: nil,
				},
			},
			expected: false,
		},
		{
			name: "Both overdue - response takes priority",
			caseObj: &Case{
				CaseSLA: CaseSLA{
					ResponseDueAt:   ptr(now.Add(-2 * time.Hour)),
					ResolutionDueAt: ptr(now.Add(-1 * time.Hour)),
					FirstResponseAt: nil,
					ResolvedAt:      nil,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.caseObj.IsOverdue()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCanBeReopened verifies reopening eligibility
func TestCanBeReopened(t *testing.T) {
	tests := []struct {
		name     string
		status   CaseStatus
		expected bool
	}{
		{
			name:     "Can reopen resolved case",
			status:   CaseStatusResolved,
			expected: true,
		},
		{
			name:     "Can reopen closed case",
			status:   CaseStatusClosed,
			expected: true,
		},
		{
			name:     "Cannot reopen new case",
			status:   CaseStatusNew,
			expected: false,
		},
		{
			name:     "Cannot reopen open case",
			status:   CaseStatusOpen,
			expected: false,
		},
		{
			name:     "Cannot reopen pending case",
			status:   CaseStatusPending,
			expected: false,
		},
		{
			name:     "Cannot reopen spam case",
			status:   CaseStatusSpam,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Case{Status: tt.status}
			result := c.CanBeReopened()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNewCommunication verifies communication constructor
func TestNewCommunication(t *testing.T) {
	caseID := "case-123"
	workspaceID := "ws-456"
	commType := shareddomain.CommTypeNote
	body := "This is a test note"

	comm := NewCommunication(caseID, workspaceID, commType, body)

	assert.Empty(t, comm.ID, "PostgreSQL should generate the communication row ID on insert")
	assert.Equal(t, caseID, comm.CaseID)
	assert.Equal(t, workspaceID, comm.WorkspaceID)
	assert.Equal(t, commType, comm.Type)
	assert.Equal(t, body, comm.Body)
	assert.Equal(t, shareddomain.DirectionInternal, comm.Direction)
	assert.True(t, comm.IsInternal)
	assert.False(t, comm.CreatedAt.IsZero())
	assert.False(t, comm.UpdatedAt.IsZero())
}

// TestCaseStatusConstants verifies status constants are defined
func TestCaseStatusConstants(t *testing.T) {
	statuses := []CaseStatus{
		CaseStatusNew,
		CaseStatusOpen,
		CaseStatusPending,
		CaseStatusResolved,
		CaseStatusClosed,
		CaseStatusSpam,
	}

	// Verify no duplicates and all have values
	seen := make(map[CaseStatus]bool)
	for _, status := range statuses {
		assert.NotEmpty(t, string(status), "Status should have value")
		assert.False(t, seen[status], "Status should be unique: %s", status)
		seen[status] = true
	}
}

// TestCasePriorityConstants verifies priority constants
func TestCasePriorityConstants(t *testing.T) {
	priorities := []CasePriority{
		CasePriorityLow,
		CasePriorityMedium,
		CasePriorityHigh,
		CasePriorityUrgent,
	}

	seen := make(map[CasePriority]bool)
	for _, priority := range priorities {
		assert.NotEmpty(t, string(priority), "Priority should have value")
		assert.False(t, seen[priority], "Priority should be unique: %s", priority)
		seen[priority] = true
	}
}

// TestCaseChannelConstants verifies channel constants
func TestCaseChannelConstants(t *testing.T) {
	channels := []CaseChannel{
		CaseChannelEmail,
		CaseChannelWeb,
		CaseChannelAPI,
		CaseChannelPhone,
		CaseChannelChat,
		CaseChannelInternal,
	}

	seen := make(map[CaseChannel]bool)
	for _, channel := range channels {
		assert.NotEmpty(t, string(channel), "Channel should have value")
		assert.False(t, seen[channel], "Channel should be unique: %s", channel)
		seen[channel] = true
	}
}

// TestCase_ComplexScenarios tests realistic case workflows
func TestCase_ComplexScenarios(t *testing.T) {
	t.Run("Case lifecycle - open to resolved to reopened", func(t *testing.T) {
		c := NewCase("ws-123", "Bug report", "user@test.com")

		// Initial state
		assert.Equal(t, CaseStatusNew, c.Status)
		assert.False(t, c.CanBeReopened())

		// Move to resolved
		c.Status = CaseStatusResolved
		now := time.Now()
		c.ResolvedAt = &now

		assert.True(t, c.CanBeReopened())

		// Simulate reopen
		c.Status = CaseStatusOpen
		c.ResolvedAt = nil
		c.ReopenCount++

		assert.Equal(t, 1, c.ReopenCount)
		assert.False(t, c.CanBeReopened())
	})

	t.Run("SLA tracking workflow", func(t *testing.T) {
		c := NewCase("ws-123", "Urgent issue", "customer@example.com")
		c.Priority = CasePriorityUrgent

		now := time.Now()

		// Set SLA targets
		c.ResponseDueAt = ptr(now.Add(1 * time.Hour))
		c.ResolutionDueAt = ptr(now.Add(4 * time.Hour))

		// Not overdue yet
		assert.False(t, c.IsOverdue())

		// Simulate time passing - response overdue
		c.ResponseDueAt = ptr(now.Add(-30 * time.Minute))
		assert.True(t, c.IsOverdue())

		// First response given
		c.FirstResponseAt = &now
		c.ResponseTimeMinutes = 90     // 1.5 hours
		assert.False(t, c.IsOverdue()) // No longer overdue for response

		// Resolution still pending, now overdue
		c.ResolutionDueAt = ptr(now.Add(-1 * time.Hour))
		assert.True(t, c.IsOverdue())

		// Case resolved
		c.ResolvedAt = &now
		c.ResolutionTimeMinutes = 300  // 5 hours
		assert.False(t, c.IsOverdue()) // No SLA violations active
	})

	t.Run("Case with linked issue", func(t *testing.T) {
		c := NewCase("ws-123", "Error in checkout", "support@shop.com")
		c.Source = shareddomain.SourceTypeAutoMonitor
		c.AutoCreated = true
		c.RootCauseIssueID = "issue-456"
		c.LinkedIssueIDs = []string{"issue-456"}

		assert.True(t, c.AutoCreated)
		assert.Equal(t, shareddomain.SourceTypeAutoMonitor, c.Source)
		assert.False(t, c.IssueResolved)

		// Issue gets resolved
		c.IssueResolved = true
		now := time.Now()
		c.IssueResolvedAt = &now
		c.ContactNotified = true
		c.ContactNotifiedAt = &now

		assert.True(t, c.IssueResolved)
		assert.True(t, c.ContactNotified)
	})
}

// TestCommunication_EmailFields verifies email-specific fields
func TestCommunication_EmailFields(t *testing.T) {
	comm := NewCommunication("case-1", "ws-1", shareddomain.CommTypeEmail, "Test body")

	// Override defaults for email
	comm.Direction = shareddomain.DirectionInbound
	comm.IsInternal = false
	comm.FromEmail = "customer@example.com"
	comm.FromName = "John Doe"
	comm.ToEmails = []string{"support@company.com"}
	comm.Subject = "Re: Order #1234"
	comm.MessageID = "<abc123@example.com>"
	comm.InReplyTo = "<xyz789@example.com>"
	comm.References = []string{"<xyz789@example.com>"}

	assert.Equal(t, shareddomain.DirectionInbound, comm.Direction)
	assert.False(t, comm.IsInternal)
	assert.Equal(t, "customer@example.com", comm.FromEmail)
	assert.NotEmpty(t, comm.MessageID)
	assert.NotEmpty(t, comm.InReplyTo)
	assert.Len(t, comm.References, 1)
}

// Helper function to create time pointer
func ptr(t time.Time) *time.Time {
	return &t
}

// Benchmark tests for performance-critical methods
func BenchmarkIsOverdue(b *testing.B) {
	now := time.Now()
	c := &Case{
		CaseSLA: CaseSLA{
			ResponseDueAt:   ptr(now.Add(-1 * time.Hour)),
			ResolutionDueAt: ptr(now.Add(-30 * time.Minute)),
			FirstResponseAt: nil,
			ResolvedAt:      nil,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.IsOverdue()
	}
}

func BenchmarkCanBeReopened(b *testing.B) {
	c := &Case{Status: CaseStatusResolved}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.CanBeReopened()
	}
}

// Property-based test helpers
func TestCase_Invariants(t *testing.T) {
	t.Run("CreatedAt should always be before or equal to UpdatedAt", func(t *testing.T) {
		c := NewCase("ws-1", "Test", "user@test.com")

		// Simulate update
		c.UpdatedAt = time.Now()

		assert.True(t, c.CreatedAt.Before(c.UpdatedAt) || c.CreatedAt.Equal(c.UpdatedAt),
			"CreatedAt must be <= UpdatedAt")
	})

	t.Run("ResolvedAt should only be set when Status is resolved or closed", func(t *testing.T) {
		c := NewCase("ws-1", "Test", "user@test.com")
		now := time.Now()

		// Case 1: Valid - resolved status with resolved time
		c.Status = CaseStatusResolved
		c.ResolvedAt = &now
		assert.Equal(t, CaseStatusResolved, c.Status)
		assert.NotNil(t, c.ResolvedAt)

		// Case 2: Valid - closed status with resolved time
		c.Status = CaseStatusClosed
		c.ResolvedAt = &now
		assert.Equal(t, CaseStatusClosed, c.Status)
		assert.NotNil(t, c.ResolvedAt)

		// Note: Validation of invariants (e.g., preventing ResolvedAt on open cases)
		// would be enforced at the service/handler layer, not in the domain struct
	})
}

// Edge case tests
func TestCase_EdgeCases(t *testing.T) {
	t.Run("Empty subject", func(t *testing.T) {
		c := NewCase("ws-1", "", "user@test.com")
		assert.Empty(t, c.Subject)
		assert.Empty(t, c.ID) // Row ID is assigned on insert
	})

	t.Run("Empty contact email", func(t *testing.T) {
		c := NewCase("ws-1", "Test", "")
		assert.Empty(t, c.ContactEmail)
		assert.Empty(t, c.ID)
	})

	t.Run("Very long subject", func(t *testing.T) {
		longSubject := string(make([]byte, 10000))
		c := NewCase("ws-1", longSubject, "user@test.com")
		assert.Equal(t, longSubject, c.Subject)
	})

	t.Run("Nil time pointers", func(t *testing.T) {
		c := &Case{
			CaseSLA: CaseSLA{
				ResponseDueAt:   nil,
				ResolutionDueAt: nil,
				FirstResponseAt: nil,
				ResolvedAt:      nil,
			},
		}

		assert.False(t, c.IsOverdue()) // Should not panic
	})

	t.Run("Message count increment", func(t *testing.T) {
		c := NewCase("ws-1", "Test", "user@test.com")
		assert.Equal(t, 0, c.MessageCount)

		// Simulate adding messages
		c.MessageCount++
		c.MessageCount++
		assert.Equal(t, 2, c.MessageCount)
	})
}
