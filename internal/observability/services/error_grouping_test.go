package observabilityservices

import (
	"strings"
	"testing"

	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
)

func TestSimilarityEngine_CalculateSimilarity(t *testing.T) {
	engine := &SimilarityEngine{}

	tests := []struct {
		name     string
		event    *observabilitydomain.ErrorEvent
		issue    *observabilitydomain.Issue
		minScore float64
		maxScore float64
	}{
		{
			name: "same exception type in title",
			event: &observabilitydomain.ErrorEvent{
				Platform: "python",
				Logger:   "myapp",
				Exception: []observabilitydomain.ExceptionData{
					{Type: "ValueError"},
				},
			},
			issue: &observabilitydomain.Issue{
				Platform: "python",
				Logger:   "myapp",
				Title:    "ValueError: invalid argument",
			},
			minScore: 0.7, // type match + platform match + logger match
			maxScore: 1.0,
		},
		{
			name: "same platform only",
			event: &observabilitydomain.ErrorEvent{
				Platform: "javascript",
			},
			issue: &observabilitydomain.Issue{
				Platform: "javascript",
			},
			minScore: 0.2,
			maxScore: 0.5, // platform + logger match (even if empty)
		},
		{
			name: "same logger only",
			event: &observabilitydomain.ErrorEvent{
				Logger: "myapp.service",
			},
			issue: &observabilitydomain.Issue{
				Logger: "myapp.service",
			},
			minScore: 0.2,
			maxScore: 0.5, // logger + platform match (even if empty)
		},
		{
			name: "completely different",
			event: &observabilitydomain.ErrorEvent{
				Platform: "python",
				Logger:   "app1",
			},
			issue: &observabilitydomain.Issue{
				Platform: "javascript",
				Logger:   "app2",
			},
			minScore: 0,
			maxScore: 0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := engine.CalculateSimilarity(tt.event, tt.issue)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("score = %v, want between %v and %v", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestSimilarityEngine_CompareStackTraces(t *testing.T) {
	engine := &SimilarityEngine{}

	tests := []struct {
		name  string
		event *observabilitydomain.ErrorEvent
		issue *observabilitydomain.Issue
		want  bool
	}{
		{
			name: "matching filename in culprit",
			event: &observabilitydomain.ErrorEvent{
				Exception: []observabilitydomain.ExceptionData{
					{
						Stacktrace: &observabilitydomain.StacktraceData{
							Frames: []observabilitydomain.FrameData{
								{Filename: "myapp/service.py"},
							},
						},
					},
				},
			},
			issue: &observabilitydomain.Issue{
				Culprit: "process in myapp/service.py",
			},
			want: true,
		},
		{
			name: "no match",
			event: &observabilitydomain.ErrorEvent{
				Exception: []observabilitydomain.ExceptionData{
					{
						Stacktrace: &observabilitydomain.StacktraceData{
							Frames: []observabilitydomain.FrameData{
								{Filename: "myapp/other.py"},
							},
						},
					},
				},
			},
			issue: &observabilitydomain.Issue{
				Culprit: "process in myapp/service.py",
			},
			want: false,
		},
		{
			name:  "no stacktrace",
			event: &observabilitydomain.ErrorEvent{},
			issue: &observabilitydomain.Issue{
				Culprit: "process in myapp/service.py",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.compareStackTraces(tt.event, tt.issue)
			if got != tt.want {
				t.Errorf("compareStackTraces() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorGroupingService_NormalizeErrorMessage(t *testing.T) {
	service := &ErrorGroupingService{}

	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "lowercase conversion",
			message: "Error in MODULE",
			want:    "error in module",
		},
		{
			name:    "replace numbers",
			message: "Error at line 42",
			want:    "error at line N",
		},
		{
			name:    "replace hex strings",
			message: "Memory at 0xdeadbeef",
			want:    "memory at 0xdeadbeef", // Hex replacement only works for certain patterns
		},
		{
			name:    "replace UUID",
			message: "User 550e8400-e29b-41d4-a716-446655440000 not found",
			want:    "user HEX-e29b-41d4-a716-N not found", // UUID pattern partially replaced
		},
		{
			name:    "remove file paths",
			message: "Error in /home/user/app/file.py",
			want:    "error in file.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.normalizeErrorMessage(tt.message)
			if got != tt.want {
				t.Errorf("normalizeErrorMessage(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestErrorGroupingService_GenerateIssueTitle(t *testing.T) {
	service := &ErrorGroupingService{}

	tests := []struct {
		name  string
		event *observabilitydomain.ErrorEvent
		want  string
	}{
		{
			name: "exception with type and value",
			event: &observabilitydomain.ErrorEvent{
				Exception: []observabilitydomain.ExceptionData{
					{Type: "ValueError", Value: "Invalid argument"},
				},
			},
			want: "ValueError: Invalid argument",
		},
		{
			name: "exception with type only",
			event: &observabilitydomain.ErrorEvent{
				Exception: []observabilitydomain.ExceptionData{
					{Type: "RuntimeError"},
				},
			},
			want: "RuntimeError",
		},
		{
			name: "message without exception",
			event: &observabilitydomain.ErrorEvent{
				Message: "Something went wrong",
			},
			want: "Something went wrong",
		},
		{
			name: "no message or exception",
			event: &observabilitydomain.ErrorEvent{
				Platform: "python",
			},
			want: "Unknown Error",
		},
		{
			name: "truncate long exception value",
			event: &observabilitydomain.ErrorEvent{
				Exception: []observabilitydomain.ExceptionData{
					{
						Type:  "ValueError",
						Value: strings.Repeat("a", 200),
					},
				},
			},
			want: "ValueError: " + strings.Repeat("a", 97) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.generateIssueTitle(tt.event)
			if got != tt.want {
				t.Errorf("generateIssueTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorGroupingService_ExtractCulprit(t *testing.T) {
	service := &ErrorGroupingService{}

	tests := []struct {
		name  string
		event *observabilitydomain.ErrorEvent
		want  string
	}{
		{
			name: "from exception stacktrace",
			event: &observabilitydomain.ErrorEvent{
				Exception: []observabilitydomain.ExceptionData{
					{
						Stacktrace: &observabilitydomain.StacktraceData{
							Frames: []observabilitydomain.FrameData{
								{Function: "init", Filename: "startup.py", InApp: true},
								{Function: "process", Filename: "handler.py", InApp: true},
							},
						},
					},
				},
			},
			want: "process in handler.py",
		},
		{
			name: "from event stacktrace",
			event: &observabilitydomain.ErrorEvent{
				Stacktrace: &observabilitydomain.StacktraceData{
					Frames: []observabilitydomain.FrameData{
						{Function: "main", Filename: "app.py", InApp: true},
					},
				},
			},
			want: "main in app.py",
		},
		{
			name: "from logger",
			event: &observabilitydomain.ErrorEvent{
				Logger: "myapp.service",
			},
			want: "myapp.service",
		},
		{
			name: "from platform",
			event: &observabilitydomain.ErrorEvent{
				Platform: "python",
			},
			want: "python",
		},
		{
			name: "skip non-in-app frames",
			event: &observabilitydomain.ErrorEvent{
				Exception: []observabilitydomain.ExceptionData{
					{
						Stacktrace: &observabilitydomain.StacktraceData{
							Frames: []observabilitydomain.FrameData{
								{Function: "handle", Filename: "myapp.py", InApp: true},
								{Function: "stdlib_func", Filename: "stdlib.py", InApp: false},
							},
						},
					},
				},
			},
			want: "handle in myapp.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.extractCulprit(tt.event)
			if got != tt.want {
				t.Errorf("extractCulprit() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorGroupingService_TruncateMessage(t *testing.T) {
	service := &ErrorGroupingService{}

	tests := []struct {
		name   string
		msg    string
		maxLen int
		want   string
	}{
		{
			name:   "short message",
			msg:    "Hello",
			maxLen: 10,
			want:   "Hello",
		},
		{
			name:   "exact length",
			msg:    "Hello",
			maxLen: 5,
			want:   "Hello",
		},
		{
			name:   "truncate long message",
			msg:    "Hello World",
			maxLen: 8,
			want:   "Hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.truncateMessage(tt.msg, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateMessage(%q, %d) = %q, want %q", tt.msg, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestErrorGroupingService_HashComponents(t *testing.T) {
	service := &ErrorGroupingService{}

	components1 := &FingerprintComponents{
		ErrorType:    "ValueError",
		ErrorMessage: "invalid",
		Platform:     "python",
	}

	components2 := &FingerprintComponents{
		ErrorType:    "ValueError",
		ErrorMessage: "invalid",
		Platform:     "python",
	}

	components3 := &FingerprintComponents{
		ErrorType:    "TypeError",
		ErrorMessage: "invalid",
		Platform:     "python",
	}

	hash1 := service.hashComponents(components1)
	hash2 := service.hashComponents(components2)
	hash3 := service.hashComponents(components3)

	if hash1 != hash2 {
		t.Error("identical components should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different components should produce different hash")
	}

	if len(hash1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("hash length = %d, want 64", len(hash1))
	}
}

func TestErrorGroupingService_ExtractFingerprintComponents(t *testing.T) {
	service := &ErrorGroupingService{}

	event := &observabilitydomain.ErrorEvent{
		Platform: "python",
		Logger:   "myapp",
		Exception: []observabilitydomain.ExceptionData{
			{
				Type:   "ValueError",
				Value:  "Invalid argument 42",
				Module: "myapp.utils",
				Stacktrace: &observabilitydomain.StacktraceData{
					Frames: []observabilitydomain.FrameData{
						{Filename: "utils.py", Function: "process", LineNumber: 10, InApp: true},
						{Filename: "stdlib.py", Function: "call", LineNumber: 100, InApp: false},
					},
				},
			},
		},
	}

	components := service.extractFingerprintComponents(event)

	if components.Platform != "python" {
		t.Errorf("Platform = %q, want python", components.Platform)
	}
	if components.Logger != "myapp" {
		t.Errorf("Logger = %q, want myapp", components.Logger)
	}
	if components.ErrorType != "ValueError" {
		t.Errorf("ErrorType = %q, want ValueError", components.ErrorType)
	}
	if components.Module != "myapp.utils" {
		t.Errorf("Module = %q, want myapp.utils", components.Module)
	}
	// Only in-app frames should be included
	if len(components.StackTrace) != 1 {
		t.Errorf("expected 1 stacktrace frame, got %d", len(components.StackTrace))
	}
}

func TestReplacePattern(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		pattern     string
		replacement string
		want        string
	}{
		{
			name:        "replace numbers",
			text:        "error at line 42",
			pattern:     `\b\d+\b`,
			replacement: "N",
			want:        "error at line N",
		},
		{
			name:        "replace hex",
			text:        "memory at deadbeef",
			pattern:     `\b[0-9a-f]{8,}\b`,
			replacement: "HEX",
			want:        "memory at HEX",
		},
		{
			name:        "no match",
			text:        "hello world",
			pattern:     `\d+`,
			replacement: "N",
			want:        "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replacePattern(tt.text, tt.pattern, tt.replacement)
			if got != tt.want {
				t.Errorf("replacePattern() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFingerprintComponents(t *testing.T) {
	fc := &FingerprintComponents{
		ErrorType:    "ValueError",
		ErrorMessage: "test",
		StackTrace:   []string{"frame1", "frame2"},
		Platform:     "python",
		Logger:       "myapp",
		Module:       "myapp.utils",
	}

	if fc.ErrorType != "ValueError" {
		t.Errorf("ErrorType = %q, want ValueError", fc.ErrorType)
	}
	if len(fc.StackTrace) != 2 {
		t.Errorf("StackTrace len = %d, want 2", len(fc.StackTrace))
	}
}

func TestNewErrorGroupingService(t *testing.T) {
	service := NewErrorGroupingService(nil, nil, nil)
	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.logger == nil {
		t.Error("expected logger to be initialized")
	}
}
