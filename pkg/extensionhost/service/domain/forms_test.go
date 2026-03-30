package servicedomain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shared "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// TestFormStatus tests form status constants
func TestFormStatus(t *testing.T) {
	assert.Equal(t, FormStatus("draft"), FormStatusDraft)
	assert.Equal(t, FormStatus("active"), FormStatusActive)
	assert.Equal(t, FormStatus("inactive"), FormStatusInactive)
	assert.Equal(t, FormStatus("archived"), FormStatusArchived)
}

// TestSubmissionStatus tests submission status constants
func TestSubmissionStatus(t *testing.T) {
	assert.Equal(t, SubmissionStatus("pending"), SubmissionStatusPending)
	assert.Equal(t, SubmissionStatus("processing"), SubmissionStatusProcessing)
	assert.Equal(t, SubmissionStatus("completed"), SubmissionStatusCompleted)
	assert.Equal(t, SubmissionStatus("failed"), SubmissionStatusFailed)
}

// TestNewFormSchema tests form definition creation.
func TestNewFormSchema(t *testing.T) {
	workspaceID := "workspace-1"
	name := "Customer Feedback Form"
	slug := "customer-feedback"
	createdByID := "user-123"

	schema := NewFormSchema(workspaceID, name, slug, createdByID)

	assert.Empty(t, schema.ID)
	assert.Equal(t, workspaceID, schema.WorkspaceID)
	assert.Equal(t, name, schema.Name)
	assert.Equal(t, slug, schema.Slug)
	assert.Equal(t, createdByID, schema.CreatedByID)
	assert.Equal(t, FormStatusDraft, schema.Status)
	assert.False(t, schema.IsPublic)
	assert.True(t, schema.RequiresAuth)
	assert.True(t, schema.AllowMultiple)
	assert.True(t, schema.CollectEmail)
	assert.True(t, schema.AutoCreateCase)
	assert.NotNil(t, schema.SchemaData)
	assert.NotNil(t, schema.UISchema)
	assert.NotNil(t, schema.ValidationRules)
	assert.NotNil(t, schema.NotificationEmails)
	assert.NotNil(t, schema.AutoTags)
	assert.NotNil(t, schema.AllowedDomains)
	assert.NotNil(t, schema.BlockedDomains)
	assert.False(t, schema.CreatedAt.IsZero())
	assert.False(t, schema.UpdatedAt.IsZero())
	assert.NotEmpty(t, schema.CryptoID)
}

// TestNewFormSubmission tests form submission creation
func TestNewPublicFormSubmission(t *testing.T) {
	workspaceID := "workspace-1"
	formID := "form-123"
	data := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	submission := NewPublicFormSubmission(workspaceID, formID, shared.MetadataFromMap(data))

	assert.Empty(t, submission.ID)
	assert.Equal(t, workspaceID, submission.WorkspaceID)
	assert.Equal(t, formID, submission.FormID)
	assert.Equal(t, data, submission.Data.ToInterfaceMap())
	assert.Equal(t, SubmissionStatusPending, submission.Status)
	assert.True(t, submission.IsValid)
	assert.NotNil(t, submission.ProcessedData)
	assert.NotNil(t, submission.ValidationErrors)
	assert.NotNil(t, submission.SpamReasons)
	assert.NotNil(t, submission.AttachmentIDs)
	assert.False(t, submission.CreatedAt.IsZero())
	assert.False(t, submission.UpdatedAt.IsZero())
}

// TestFormSubmission_IsExpired tests expiration checking
func TestFormSubmission_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		status    SubmissionStatus
		createdAt time.Time
		expected  bool
	}{
		{
			name:      "Processing and old - expired",
			status:    SubmissionStatusProcessing,
			createdAt: time.Now().Add(-25 * time.Hour),
			expected:  true,
		},
		{
			name:      "Processing and recent - not expired",
			status:    SubmissionStatusProcessing,
			createdAt: time.Now().Add(-1 * time.Hour),
			expected:  false,
		},
		{
			name:      "Completed and old - not expired",
			status:    SubmissionStatusCompleted,
			createdAt: time.Now().Add(-25 * time.Hour),
			expected:  false,
		},
		{
			name:      "Pending and old - not expired",
			status:    SubmissionStatusPending,
			createdAt: time.Now().Add(-25 * time.Hour),
			expected:  false,
		},
		{
			name:      "Failed and old - not expired",
			status:    SubmissionStatusFailed,
			createdAt: time.Now().Add(-25 * time.Hour),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			submission := &PublicFormSubmission{
				Status:    tt.status,
				CreatedAt: tt.createdAt,
			}

			result := submission.IsExpired()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormSubmission_GetFieldValue tests field value retrieval
func TestFormSubmission_GetFieldValue(t *testing.T) {
	data := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}

	submission := NewPublicFormSubmission("workspace-1", "form-1", shared.MetadataFromMap(data))

	t.Run("Existing field", func(t *testing.T) {
		value, exists := submission.GetFieldValue("name")
		assert.True(t, exists)
		assert.Equal(t, "John Doe", value.ToInterface())
	})

	t.Run("Non-existing field", func(t *testing.T) {
		value, exists := submission.GetFieldValue("phone")
		assert.False(t, exists)
		assert.True(t, value.IsNull())
	})

	t.Run("Number field", func(t *testing.T) {
		value, exists := submission.GetFieldValue("age")
		assert.True(t, exists)
		assert.Equal(t, int64(30), value.ToInterface())
	})
}

// TestFormSubmission_GetStringField tests string field retrieval
func TestFormSubmission_GetStringField(t *testing.T) {
	data := map[string]interface{}{
		"name":   "John Doe",
		"email":  "john@example.com",
		"age":    30,
		"active": true,
	}

	submission := NewPublicFormSubmission("workspace-1", "form-1", shared.MetadataFromMap(data))

	t.Run("Existing string field", func(t *testing.T) {
		value := submission.GetStringField("name")
		assert.Equal(t, "John Doe", value)
	})

	t.Run("Non-existing field", func(t *testing.T) {
		value := submission.GetStringField("phone")
		assert.Equal(t, "", value)
	})

	t.Run("Non-string field - number", func(t *testing.T) {
		value := submission.GetStringField("age")
		// GetStringField now returns string representation of numeric values
		assert.Equal(t, "30", value)
	})

	t.Run("Non-string field - bool", func(t *testing.T) {
		value := submission.GetStringField("active")
		// GetStringField now returns string representation of boolean values
		assert.Equal(t, "true", value)
	})
}

// TestFormSchema_MarshalSchemaData tests JSON marshaling
func TestFormSchema_MarshalSchemaData(t *testing.T) {
	schema := NewFormSchema("workspace-1", "Test Form", "test-form", "user-1")
	schema.SchemaData = shared.TypedSchemaFromMap(map[string]interface{}{
		"fields": []map[string]interface{}{
			{
				"name": "email",
				"type": "email",
			},
		},
	})

	data, err := schema.MarshalSchemaData()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Contains(t, parsed, "fields")
}

// TestFormSchema_UnmarshalSchemaData tests JSON unmarshaling
func TestFormSchema_UnmarshalSchemaData(t *testing.T) {
	schema := NewFormSchema("workspace-1", "Test Form", "test-form", "user-1")

	jsonData := []byte(`{
		"fields": [
			{
				"name": "email",
				"type": "email",
				"required": true
			}
		]
	}`)

	err := schema.UnmarshalSchemaData(jsonData)
	require.NoError(t, err)
	assert.False(t, schema.SchemaData.IsEmpty())
	assert.Contains(t, schema.SchemaData.ToMap(), "fields")
}

// TestFormSchema_UnmarshalSchemaData_InvalidJSON tests invalid JSON
func TestFormSchema_UnmarshalSchemaData_InvalidJSON(t *testing.T) {
	schema := NewFormSchema("workspace-1", "Test Form", "test-form", "user-1")

	invalidJSON := []byte(`{invalid json}`)

	err := schema.UnmarshalSchemaData(invalidJSON)
	assert.Error(t, err)
}

func TestFormSchemaSchemaHelpers(t *testing.T) {
	schema := NewFormSchema("workspace-1", "Test Form", "test-form", "user-1")
	schema.SchemaData = shared.TypedSchemaFromMap(map[string]interface{}{
		"type":     "object",
		"required": []string{"email"},
		"properties": map[string]interface{}{
			"email": map[string]interface{}{
				"type":   "string",
				"title":  "Email",
				"format": "email",
			},
			"age": map[string]interface{}{
				"type":    "number",
				"minimum": 18,
			},
		},
	})

	parsed, err := schema.ParseSchemaData()
	require.NoError(t, err)
	require.Equal(t, "object", parsed.Type)
	require.Equal(t, []string{"email"}, parsed.Required)

	required := schema.GetRequiredFields()
	require.Equal(t, []string{"email"}, required)

	emailField, ok := schema.GetFieldSchema("email")
	require.True(t, ok)
	require.Equal(t, "Email", emailField.Title)
	require.Equal(t, "email", emailField.Format)

	_, ok = schema.GetFieldSchema("missing")
	require.False(t, ok)

	emptySchema := NewFormSchema("workspace-1", "Empty Form", "empty-form", "user-1")
	parsed, err = emptySchema.ParseSchemaData()
	require.NoError(t, err)
	require.Empty(t, parsed.Properties)
	require.Empty(t, emptySchema.GetRequiredFields())
}

// TestFormField tests form field structure
func TestFormField(t *testing.T) {
	field := FormField{
		Name:        "email",
		Type:        "email",
		Label:       "Email Address",
		Description: "Enter your email",
		Required:    true,
		Placeholder: "you@example.com",
		Order:       1,
		Validation: shared.MetadataFromMap(map[string]interface{}{
			"pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
		}),
	}

	assert.Equal(t, "email", field.Name)
	assert.Equal(t, "email", field.Type)
	assert.Equal(t, "Email Address", field.Label)
	assert.True(t, field.Required)
	assert.NotNil(t, field.Validation)
}

// TestFormFieldOption tests form field options
func TestFormFieldOption(t *testing.T) {
	option := FormFieldOption{
		Value: "option1",
		Label: "Option 1",
		Order: 1,
	}

	assert.Equal(t, "option1", option.Value)
	assert.Equal(t, "Option 1", option.Label)
	assert.Equal(t, 1, option.Order)
}

// TestFormAnalytics tests analytics structure
func TestFormAnalytics(t *testing.T) {
	analytics := FormAnalytics{
		FormID:                "form-123",
		WorkspaceID:           "workspace-1",
		TotalSubmissions:      100,
		ValidSubmissions:      85,
		InvalidSubmissions:    10,
		SpamSubmissions:       5,
		SubmissionsToday:      5,
		SubmissionsWeek:       35,
		SubmissionsMonth:      100,
		AverageProcessingTime: 1500,
		ConversionRate:        0.85,
		AbandonmentRate:       0.15,
		FieldCompletionRates: map[string]float64{
			"email": 1.0,
			"phone": 0.8,
		},
		FieldErrorRates: map[string]float64{
			"email": 0.05,
			"phone": 0.15,
		},
		ReferrerStats: map[string]int{
			"direct":  50,
			"google":  30,
			"twitter": 20,
		},
		DeviceStats: map[string]int{
			"desktop": 60,
			"mobile":  40,
		},
		GeographicStats: map[string]int{
			"US": 70,
			"UK": 30,
		},
		LastUpdated: time.Now(),
	}

	assert.Equal(t, "form-123", analytics.FormID)
	assert.Equal(t, 100, analytics.TotalSubmissions)
	assert.Equal(t, 85, analytics.ValidSubmissions)
	assert.Equal(t, 0.85, analytics.ConversionRate)
	assert.NotNil(t, analytics.FieldCompletionRates)
	assert.NotNil(t, analytics.ReferrerStats)
}

// TestFormSchema_SubmissionFields tests submission data access in a realistic configuration.
func TestFormSchema_SubmissionFields(t *testing.T) {
	// Create form
	schema := NewFormSchema("workspace-1", "Contact Form", "contact", "user-1")
	schema.Status = FormStatusActive
	schema.IsPublic = true
	schema.AutoCreateCase = true
	schema.AutoAssignTeamID = "team-support"
	schema.NotifyOnSubmission = true
	schema.NotificationEmails = []string{"support@example.com"}

	// Create submission
	data := map[string]interface{}{
		"name":    "Jane Doe",
		"email":   "jane@example.com",
		"message": "I need help with my account",
	}
	submission := NewPublicFormSubmission(schema.WorkspaceID, schema.ID, shared.MetadataFromMap(data))
	submission.Status = SubmissionStatusProcessing

	// Get field values
	name := submission.GetStringField("name")
	assert.Equal(t, "Jane Doe", name)

	email := submission.GetStringField("email")
	assert.Equal(t, "jane@example.com", email)

	// Check expiration
	assert.False(t, submission.IsExpired())
}

// TestFormSubmission_EdgeCases tests edge cases
func TestFormSubmission_EdgeCases(t *testing.T) {
	t.Run("Empty data", func(t *testing.T) {
		submission := NewPublicFormSubmission("workspace-1", "form-1", shared.NewMetadata())
		value, exists := submission.GetFieldValue("anything")
		assert.False(t, exists)
		assert.True(t, value.IsNull())
	})

	t.Run("Nil processed data", func(t *testing.T) {
		submission := NewPublicFormSubmission("workspace-1", "form-1", shared.NewMetadata())
		assert.NotNil(t, submission.ProcessedData)
	})

	t.Run("Multiple validation errors", func(t *testing.T) {
		submission := NewPublicFormSubmission("workspace-1", "form-1", shared.NewMetadata())
		submission.ValidationErrors = []string{"error1", "error2", "error3"}
		assert.Len(t, submission.ValidationErrors, 3)
	})

	t.Run("Spam detection", func(t *testing.T) {
		submission := NewPublicFormSubmission("workspace-1", "form-1", shared.NewMetadata())
		submission.IsSpam = true
		submission.SpamScore = 0.95
		submission.SpamReasons = []string{"suspicious email", "blacklisted IP"}
		assert.True(t, submission.IsSpam)
		assert.Greater(t, submission.SpamScore, 0.9)
		assert.Len(t, submission.SpamReasons, 2)
	})
}

// TestFormSchema_ConfigurationOptions tests various configuration combinations
func TestFormSchema_ConfigurationOptions(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*FormSchema)
		verify func(*testing.T, *FormSchema)
	}{
		{
			name: "Public form without auth",
			setup: func(fs *FormSchema) {
				fs.IsPublic = true
				fs.RequiresAuth = false
			},
			verify: func(t *testing.T, fs *FormSchema) {
				assert.True(t, fs.IsPublic)
				assert.False(t, fs.RequiresAuth)
			},
		},
		{
			name: "Single submission form",
			setup: func(fs *FormSchema) {
				fs.AllowMultiple = false
			},
			verify: func(t *testing.T, fs *FormSchema) {
				assert.False(t, fs.AllowMultiple)
			},
		},
		{
			name: "Form with auto-case creation",
			setup: func(fs *FormSchema) {
				fs.AutoCreateCase = true
				fs.AutoAssignTeamID = "team-1"
				fs.AutoCasePriority = "high"
				fs.AutoTags = []string{"form", "urgent"}
			},
			verify: func(t *testing.T, fs *FormSchema) {
				assert.True(t, fs.AutoCreateCase)
				assert.Equal(t, "team-1", fs.AutoAssignTeamID)
				assert.Equal(t, "high", fs.AutoCasePriority)
				assert.Len(t, fs.AutoTags, 2)
			},
		},
		{
			name: "Form with domain restrictions",
			setup: func(fs *FormSchema) {
				fs.AllowedDomains = []string{"company.com", "partner.com"}
				fs.BlockedDomains = []string{"spam.com"}
			},
			verify: func(t *testing.T, fs *FormSchema) {
				assert.Len(t, fs.AllowedDomains, 2)
				assert.Len(t, fs.BlockedDomains, 1)
			},
		},
		{
			name: "Form with captcha and rate limiting",
			setup: func(fs *FormSchema) {
				fs.RequiresCaptcha = true
				fs.MaxSubmissionsPerDay = 10
			},
			verify: func(t *testing.T, fs *FormSchema) {
				assert.True(t, fs.RequiresCaptcha)
				assert.Equal(t, 10, fs.MaxSubmissionsPerDay)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := NewFormSchema("workspace-1", "Test Form", "test", "user-1")
			tt.setup(schema)
			tt.verify(t, schema)
		})
	}
}

// TestFormSubmission_StatusTransitions tests submission status changes
func TestFormSubmission_StatusTransitions(t *testing.T) {
	submission := NewPublicFormSubmission("workspace-1", "form-1", shared.NewMetadata())

	// Initial state
	assert.Equal(t, SubmissionStatusPending, submission.Status)

	// Transition to processing
	submission.Status = SubmissionStatusProcessing
	assert.Equal(t, SubmissionStatusProcessing, submission.Status)

	// Transition to completed
	submission.Status = SubmissionStatusCompleted
	processedTime := time.Now()
	submission.ProcessedAt = &processedTime
	submission.ProcessedByID = "user-123"
	submission.ProcessingTime = 1500
	assert.Equal(t, SubmissionStatusCompleted, submission.Status)
	assert.NotNil(t, submission.ProcessedAt)
	assert.Equal(t, "user-123", submission.ProcessedByID)
	assert.Equal(t, int64(1500), submission.ProcessingTime)
}

// TestFormSubmission_ValidationAndSpam tests validation and spam detection
func TestFormSubmission_ValidationAndSpam(t *testing.T) {
	submission := NewPublicFormSubmission("workspace-1", "form-1", shared.MetadataFromMap(map[string]interface{}{
		"email": "test@example.com",
	}))

	t.Run("Valid submission", func(t *testing.T) {
		assert.True(t, submission.IsValid)
		assert.Empty(t, submission.ValidationErrors)
		assert.False(t, submission.IsSpam)
	})

	t.Run("Invalid submission", func(t *testing.T) {
		submission.IsValid = false
		submission.ValidationErrors = []string{
			"email is required",
			"name must be at least 2 characters",
		}
		assert.False(t, submission.IsValid)
		assert.Len(t, submission.ValidationErrors, 2)
	})

	t.Run("Spam submission", func(t *testing.T) {
		submission.IsSpam = true
		submission.SpamScore = 0.87
		submission.SpamReasons = []string{
			"contains suspicious links",
			"blacklisted email domain",
		}
		assert.True(t, submission.IsSpam)
		assert.Greater(t, submission.SpamScore, 0.8)
		assert.Len(t, submission.SpamReasons, 2)
	})
}

// ==================== MULTI-STEP WORKFLOW TESTS ====================

// TestFormSchema_WorkflowMethods tests workflow-related FormSchema methods
func TestFormSchema_WorkflowMethods(t *testing.T) {
	schema := NewFormSchema("workspace-1", "Multi-step Form", "multistep", "user-1")
	schema.HasWorkflow = true

	// Add workflow states
	state1 := FormWorkflowState{
		ID:      "state-1",
		FormID:  schema.ID,
		Name:    "Contact Info",
		IsStart: true,
		Order:   1,
		Fields:  []string{"name", "email"},
	}
	state2 := FormWorkflowState{
		ID:     "state-2",
		FormID: schema.ID,
		Name:   "Details",
		Order:  2,
		Fields: []string{"message", "priority"},
	}
	state3 := FormWorkflowState{
		ID:     "state-3",
		FormID: schema.ID,
		Name:   "Confirmation",
		IsEnd:  true,
		Order:  3,
		Fields: []string{},
	}
	schema.WorkflowStates = []FormWorkflowState{state1, state2, state3}

	// Add transitions
	schema.Transitions = []FormWorkflowTransition{
		{ID: "t1", FormID: schema.ID, FromStateID: "state-1", ToStateID: "state-2", Label: "Next"},
		{ID: "t2", FormID: schema.ID, FromStateID: "state-2", ToStateID: "state-3", Label: "Submit"},
		{ID: "t3", FormID: schema.ID, FromStateID: "state-2", ToStateID: "state-1", Label: "Back"},
	}

	t.Run("GetStartState", func(t *testing.T) {
		startState := schema.GetStartState()
		require.NotNil(t, startState)
		assert.Equal(t, "state-1", startState.ID)
		assert.Equal(t, "Contact Info", startState.Name)
		assert.True(t, startState.IsStart)
	})

	t.Run("GetStateByID", func(t *testing.T) {
		state := schema.GetStateByID("state-2")
		require.NotNil(t, state)
		assert.Equal(t, "Details", state.Name)

		// Non-existent state
		nonExistent := schema.GetStateByID("state-99")
		assert.Nil(t, nonExistent)
	})

	t.Run("GetTransitionsFromState", func(t *testing.T) {
		transitions := schema.GetTransitionsFromState("state-2")
		assert.Len(t, transitions, 2) // "Submit" and "Back"

		// State with one transition
		transitions = schema.GetTransitionsFromState("state-1")
		assert.Len(t, transitions, 1)

		// End state with no transitions
		transitions = schema.GetTransitionsFromState("state-3")
		assert.Len(t, transitions, 0)
	})

	t.Run("No workflow", func(t *testing.T) {
		noWorkflowSchema := NewFormSchema("workspace-1", "Simple Form", "simple", "user-1")
		startState := noWorkflowSchema.GetStartState()
		assert.Nil(t, startState)
	})
}

// TestFormSubmission_WorkflowTransitions tests submission workflow state changes
func TestFormSubmission_WorkflowTransitions(t *testing.T) {
	// Create form with workflow
	schema := NewFormSchema("workspace-1", "Workflow Form", "workflow", "user-1")
	schema.HasWorkflow = true
	schema.WorkflowStates = []FormWorkflowState{
		{ID: "step1", FormID: schema.ID, Name: "Step 1", IsStart: true, Fields: []string{"name"}},
		{ID: "step2", FormID: schema.ID, Name: "Step 2", Fields: []string{"email"}},
		{ID: "step3", FormID: schema.ID, Name: "Done", IsEnd: true},
	}

	// Create submission
	submission := NewPublicFormSubmission(schema.WorkspaceID, schema.ID, shared.NewMetadata())
	submission.CurrentStateID = "step1"

	// Transition to step 2
	step1Data := shared.MetadataFromMap(map[string]interface{}{"name": "John Doe"})
	submission.TransitionToState("step2", step1Data, "user-1")

	assert.Equal(t, "step2", submission.CurrentStateID)
	assert.Len(t, submission.StateHistory, 1)
	assert.Equal(t, "step1", submission.StateHistory[0].FromStateID)
	assert.Equal(t, "step2", submission.StateHistory[0].ToStateID)
	assert.Equal(t, "John Doe", submission.Data.ToInterfaceMap()["name"])

	// Transition to final step
	step2Data := shared.MetadataFromMap(map[string]interface{}{"email": "john@example.com"})
	submission.TransitionToState("step3", step2Data, "user-1")

	assert.Equal(t, "step3", submission.CurrentStateID)
	assert.Len(t, submission.StateHistory, 2)
	assert.Equal(t, "john@example.com", submission.Data.ToInterfaceMap()["email"])

	// Verify IsInEndState
	assert.True(t, submission.IsInEndState(schema))
}

// TestFormSubmission_GetVisibleFields tests field visibility in workflow states
func TestFormSubmission_GetVisibleFields(t *testing.T) {
	schema := NewFormSchema("workspace-1", "Workflow Form", "workflow", "user-1")
	schema.HasWorkflow = true
	schema.WorkflowStates = []FormWorkflowState{
		{ID: "step1", FormID: schema.ID, Name: "Step 1", IsStart: true, Fields: []string{"name", "email"}},
		{ID: "step2", FormID: schema.ID, Name: "Step 2", Fields: []string{"phone", "address"}},
	}

	submission := NewPublicFormSubmission(schema.WorkspaceID, schema.ID, shared.NewMetadata())

	t.Run("Fields for step 1", func(t *testing.T) {
		submission.CurrentStateID = "step1"
		fields := submission.GetVisibleFields(schema)
		assert.Equal(t, []string{"name", "email"}, fields)
	})

	t.Run("Fields for step 2", func(t *testing.T) {
		submission.CurrentStateID = "step2"
		fields := submission.GetVisibleFields(schema)
		assert.Equal(t, []string{"phone", "address"}, fields)
	})

	t.Run("No workflow", func(t *testing.T) {
		noWorkflowSchema := NewFormSchema("workspace-1", "Simple", "simple", "user-1")
		fields := submission.GetVisibleFields(noWorkflowSchema)
		assert.Nil(t, fields) // All fields visible
	})
}

// TestFormSubmission_IsInEndState tests end state detection
func TestFormSubmission_IsInEndState(t *testing.T) {
	schema := NewFormSchema("workspace-1", "Workflow Form", "workflow", "user-1")
	schema.HasWorkflow = true
	schema.WorkflowStates = []FormWorkflowState{
		{ID: "start", FormID: schema.ID, Name: "Start", IsStart: true},
		{ID: "end", FormID: schema.ID, Name: "End", IsEnd: true},
	}

	submission := NewPublicFormSubmission(schema.WorkspaceID, schema.ID, shared.NewMetadata())

	t.Run("Not in end state", func(t *testing.T) {
		submission.CurrentStateID = "start"
		assert.False(t, submission.IsInEndState(schema))
	})

	t.Run("In end state", func(t *testing.T) {
		submission.CurrentStateID = "end"
		assert.True(t, submission.IsInEndState(schema))
	})

	t.Run("No workflow - completed status", func(t *testing.T) {
		noWorkflowSchema := NewFormSchema("workspace-1", "Simple", "simple", "user-1")
		submission.CurrentStateID = ""
		submission.Status = SubmissionStatusCompleted
		assert.True(t, submission.IsInEndState(noWorkflowSchema))
	})
}
