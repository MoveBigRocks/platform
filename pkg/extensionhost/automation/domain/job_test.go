package automationdomain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJob(t *testing.T) {
	workspaceID := "ws-123"
	payload := map[string]any{
		"key": "value",
		"num": 42,
	}

	job, err := NewJob(workspaceID, "test_job", payload)
	require.NoError(t, err)

	assert.Empty(t, job.ID)
	assert.Equal(t, workspaceID, job.WorkspaceID)
	assert.NotEmpty(t, job.PublicID)
	assert.Equal(t, "test_job", job.Name)
	assert.Equal(t, "default", job.Queue)
	assert.Equal(t, JobPriorityNormal, job.Priority)
	assert.Equal(t, JobStatusPending, job.Status)
	// Verify payload was marshaled
	var unmarshaled map[string]any
	require.NoError(t, job.UnmarshalPayload(&unmarshaled))
	assert.Equal(t, "value", unmarshaled["key"])
	assert.Equal(t, float64(42), unmarshaled["num"]) // JSON numbers are float64
	assert.Equal(t, 0, job.Attempts)
	assert.Equal(t, 3, job.MaxAttempts)
	assert.NotNil(t, job.ScheduledFor)
	assert.False(t, job.CreatedAt.IsZero())
	assert.False(t, job.UpdatedAt.IsZero())
}

func TestNewWorkspaceJob(t *testing.T) {
	workspaceID := "ws-123"
	job, err := NewWorkspaceJob(workspaceID, "workspace_task", nil)
	require.NoError(t, err)

	assert.Equal(t, workspaceID, job.WorkspaceID)
	assert.False(t, job.IsGlobal())
}

func TestJob_Validate(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid job",
			job: &Job{
				WorkspaceID: "ws-123",
				Name:        "test_job",
				Priority:    JobPriorityNormal,
			},
			wantErr: false,
		},
		{
			name: "missing workspace",
			job: &Job{
				Name:     "job_without_workspace",
				Priority: JobPriorityNormal,
			},
			wantErr: true,
			errMsg:  "workspace_id is required",
		},
		{
			name: "missing name",
			job: &Job{
				Name:     "",
				Priority: JobPriorityNormal,
			},
			wantErr: true,
			errMsg:  "job name is required",
		},
		{
			name: "invalid priority too low",
			job: &Job{
				WorkspaceID: "ws-123",
				Name:        "test_job",
				Priority:    -1,
			},
			wantErr: true,
			errMsg:  "invalid job priority",
		},
		{
			name: "invalid priority too high",
			job: &Job{
				WorkspaceID: "ws-123",
				Name:        "test_job",
				Priority:    100,
			},
			wantErr: true,
			errMsg:  "invalid job priority",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJob_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		job      *Job
		expected bool
	}{
		{
			name: "pending and scheduled in past",
			job: &Job{
				Status:       JobStatusPending,
				ScheduledFor: timePtr(time.Now().Add(-1 * time.Hour)),
			},
			expected: true,
		},
		{
			name: "pending and scheduled now",
			job: &Job{
				Status:       JobStatusPending,
				ScheduledFor: timePtr(time.Now()),
			},
			expected: true,
		},
		{
			name: "pending with nil scheduled",
			job: &Job{
				Status:       JobStatusPending,
				ScheduledFor: nil,
			},
			expected: true,
		},
		{
			name: "pending but scheduled in future",
			job: &Job{
				Status:       JobStatusPending,
				ScheduledFor: timePtr(time.Now().Add(1 * time.Hour)),
			},
			expected: false,
		},
		{
			name: "running status",
			job: &Job{
				Status:       JobStatusRunning,
				ScheduledFor: timePtr(time.Now().Add(-1 * time.Hour)),
			},
			expected: false,
		},
		{
			name: "completed status",
			job: &Job{
				Status:       JobStatusCompleted,
				ScheduledFor: timePtr(time.Now().Add(-1 * time.Hour)),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.job.IsReady()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJob_CanRetry(t *testing.T) {
	tests := []struct {
		name     string
		job      *Job
		expected bool
	}{
		{
			name: "failed and can retry",
			job: &Job{
				Status:      JobStatusFailed,
				Attempts:    1,
				MaxAttempts: 3,
			},
			expected: true,
		},
		{
			name: "failed and at max attempts",
			job: &Job{
				Status:      JobStatusFailed,
				Attempts:    3,
				MaxAttempts: 3,
			},
			expected: false,
		},
		{
			name: "failed and exceeded max attempts",
			job: &Job{
				Status:      JobStatusFailed,
				Attempts:    5,
				MaxAttempts: 3,
			},
			expected: false,
		},
		{
			name: "pending status",
			job: &Job{
				Status:      JobStatusPending,
				Attempts:    0,
				MaxAttempts: 3,
			},
			expected: false,
		},
		{
			name: "running status",
			job: &Job{
				Status:      JobStatusRunning,
				Attempts:    1,
				MaxAttempts: 3,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.job.CanRetry()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJob_CanCancel(t *testing.T) {
	tests := []struct {
		name     string
		status   JobStatus
		expected bool
	}{
		{"pending", JobStatusPending, true},
		{"retrying", JobStatusRetrying, true},
		{"running", JobStatusRunning, false},
		{"completed", JobStatusCompleted, false},
		{"failed", JobStatusFailed, false},
		{"canceled", JobStatusCanceled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{Status: tt.status}
			result := job.CanCancel()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJob_MarkRunning(t *testing.T) {
	job, err := NewJob("ws-123", "test", nil)
	require.NoError(t, err)
	workerID := "worker-123"

	job.MarkRunning(workerID)

	assert.Equal(t, JobStatusRunning, job.Status)
	assert.Equal(t, workerID, job.WorkerID)
	assert.NotNil(t, job.StartedAt)
	assert.NotNil(t, job.LockedUntil)
	assert.True(t, job.LockedUntil.After(time.Now()))
}

func TestJob_MarkCompleted(t *testing.T) {
	job, err := NewJob("ws-123", "test", nil)
	require.NoError(t, err)
	job.MarkRunning("worker-123")

	result := map[string]any{"output": "success"}
	err = job.MarkCompleted(result)
	require.NoError(t, err)

	assert.Equal(t, JobStatusCompleted, job.Status)
	var unmarshaled map[string]any
	require.NoError(t, job.UnmarshalResult(&unmarshaled))
	assert.Equal(t, "success", unmarshaled["output"])
	assert.NotNil(t, job.CompletedAt)
	assert.Nil(t, job.LockedUntil)
	assert.Empty(t, job.WorkerID)
}

func TestJob_MarkFailed(t *testing.T) {
	job, err := NewJob("ws-123", "test", nil)
	require.NoError(t, err)
	job.MarkRunning("worker-123")

	errorMsg := "something went wrong"
	err = job.MarkFailed(errorMsg)
	require.NoError(t, err)

	assert.Equal(t, JobStatusFailed, job.Status)
	assert.Equal(t, errorMsg, job.Error)
	assert.NotNil(t, job.CompletedAt)
	assert.Nil(t, job.LockedUntil)
	assert.Empty(t, job.WorkerID)
}

func TestJob_MarkRetrying(t *testing.T) {
	job, err := NewJob("ws-123", "test", nil)
	require.NoError(t, err)
	job.MarkRunning("worker-123")
	err = job.MarkFailed("error")
	require.NoError(t, err)
	initialAttempts := job.Attempts

	job.MarkRetrying()

	assert.Equal(t, JobStatusRetrying, job.Status)
	assert.Equal(t, initialAttempts+1, job.Attempts)
	assert.NotNil(t, job.ScheduledFor)
	assert.True(t, job.ScheduledFor.After(time.Now()))
	assert.Nil(t, job.LockedUntil)
	assert.Empty(t, job.WorkerID)
}

func TestJob_MarkCanceled(t *testing.T) {
	job, err := NewJob("ws-123", "test", nil)
	require.NoError(t, err)

	job.MarkCanceled()

	assert.Equal(t, JobStatusCanceled, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.Nil(t, job.LockedUntil)
	assert.Empty(t, job.WorkerID)
}

func TestJob_IsLocked(t *testing.T) {
	tests := []struct {
		name        string
		lockedUntil *time.Time
		expected    bool
	}{
		{
			name:        "locked in future",
			lockedUntil: timePtr(time.Now().Add(10 * time.Minute)),
			expected:    true,
		},
		{
			name:        "lock expired",
			lockedUntil: timePtr(time.Now().Add(-10 * time.Minute)),
			expected:    false,
		},
		{
			name:        "no lock",
			lockedUntil: nil,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{LockedUntil: tt.lockedUntil}
			result := job.IsLocked()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJob_IsExpired(t *testing.T) {
	tests := []struct {
		name        string
		status      JobStatus
		lockedUntil *time.Time
		expected    bool
	}{
		{
			name:        "running with expired lock",
			status:      JobStatusRunning,
			lockedUntil: timePtr(time.Now().Add(-10 * time.Minute)),
			expected:    true,
		},
		{
			name:        "running with valid lock",
			status:      JobStatusRunning,
			lockedUntil: timePtr(time.Now().Add(10 * time.Minute)),
			expected:    false,
		},
		{
			name:        "pending with expired lock",
			status:      JobStatusPending,
			lockedUntil: timePtr(time.Now().Add(-10 * time.Minute)),
			expected:    false,
		},
		{
			name:        "running with no lock",
			status:      JobStatusRunning,
			lockedUntil: nil,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{
				Status:      tt.status,
				LockedUntil: tt.lockedUntil,
			}
			result := job.IsExpired()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJob_GetPayloadString(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"string_val": "hello",
		"int_val":    42,
		"bool_val":   true,
	})
	job := &Job{Payload: payload}

	assert.Equal(t, "hello", job.GetPayloadString("string_val"))
	assert.Equal(t, "", job.GetPayloadString("int_val"))
	assert.Equal(t, "", job.GetPayloadString("missing"))
}

func TestJob_GetPayloadInt(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"int_val":    42,
		"float_val":  3.14,
		"string_val": "hello",
	})
	job := &Job{Payload: payload}

	assert.Equal(t, 42, job.GetPayloadInt("int_val"))
	assert.Equal(t, 3, job.GetPayloadInt("float_val"))
	assert.Equal(t, 0, job.GetPayloadInt("string_val"))
	assert.Equal(t, 0, job.GetPayloadInt("missing"))
}

func TestJob_GetPayloadBool(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"true_val":   true,
		"false_val":  false,
		"string_val": "true",
	})
	job := &Job{Payload: payload}

	assert.True(t, job.GetPayloadBool("true_val"))
	assert.False(t, job.GetPayloadBool("false_val"))
	assert.False(t, job.GetPayloadBool("string_val"))
	assert.False(t, job.GetPayloadBool("missing"))
}

func TestJob_GetPayloadSlice(t *testing.T) {
	expectedSlice := []any{"a", "b", "c"}
	payload, _ := json.Marshal(map[string]any{
		"slice_val":  expectedSlice,
		"string_val": "hello",
	})
	job := &Job{Payload: payload}

	slice := job.GetPayloadSlice("slice_val")
	assert.Len(t, slice, 3)
	assert.Equal(t, "a", slice[0])
	assert.Equal(t, "b", slice[1])
	assert.Equal(t, "c", slice[2])
	assert.Nil(t, job.GetPayloadSlice("string_val"))
	assert.Nil(t, job.GetPayloadSlice("missing"))
}

func TestJob_NextRetryTime(t *testing.T) {
	job, err := NewJob("ws-123", "test", nil)
	require.NoError(t, err)

	// First retry (attempts=0): 60 * (1 << 0) = 60 seconds
	job.Attempts = 0
	next := job.NextRetryTime()
	assert.WithinDuration(t, time.Now().Add(60*time.Second), next, 2*time.Second)

	// Second retry (attempts=1): 60 * (1 << 1) = 120 seconds
	job.Attempts = 1
	next = job.NextRetryTime()
	assert.WithinDuration(t, time.Now().Add(120*time.Second), next, 2*time.Second)

	// Third retry (attempts=2): 60 * (1 << 2) = 240 seconds
	job.Attempts = 2
	next = job.NextRetryTime()
	assert.WithinDuration(t, time.Now().Add(240*time.Second), next, 2*time.Second)
}

func TestJobPriority_String(t *testing.T) {
	tests := []struct {
		priority JobPriority
		expected string
	}{
		{JobPriorityLow, "low"},
		{JobPriorityNormal, "normal"},
		{JobPriorityHigh, "high"},
		{JobPriorityCritical, "critical"},
		{JobPriority(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority.String())
		})
	}
}

func TestJobStatusConstants(t *testing.T) {
	assert.Equal(t, JobStatus("pending"), JobStatusPending)
	assert.Equal(t, JobStatus("running"), JobStatusRunning)
	assert.Equal(t, JobStatus("completed"), JobStatusCompleted)
	assert.Equal(t, JobStatus("failed"), JobStatusFailed)
	assert.Equal(t, JobStatus("canceled"), JobStatusCanceled)
	assert.Equal(t, JobStatus("retrying"), JobStatusRetrying)
}

func TestJobPriorityValues(t *testing.T) {
	assert.Equal(t, JobPriority(0), JobPriorityLow)
	assert.Equal(t, JobPriority(5), JobPriorityNormal)
	assert.Equal(t, JobPriority(10), JobPriorityHigh)
	assert.Equal(t, JobPriority(20), JobPriorityCritical)
}

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}
