package observabilitydomain

import (
	"strings"
	"testing"
)

func TestGenerateAdvancedFingerprintStable(t *testing.T) {
	event := NewErrorEvent("project_1", "event_1")
	event.Platform = "go"
	event.Logger = "app"
	event.Message = "Order 1234 failed for user 9999"

	first := GenerateAdvancedFingerprint(event)
	second := GenerateAdvancedFingerprint(event)
	if first == "" || first != second {
		t.Fatalf("expected stable non-empty fingerprint, got %q and %q", first, second)
	}
}

func TestGenerateIssueTitleAndCulprit(t *testing.T) {
	event := NewErrorEvent("project_1", "event_1")
	event.Exception = []ExceptionData{{
		Type:  "TypeError",
		Value: "Cannot read property length of undefined",
		Stacktrace: &StacktraceData{Frames: []FrameData{
			{Filename: "vendor.js", Function: "wrapped", InApp: false},
			{Filename: "handler.go", Function: "HandleRequest", InApp: true},
		}},
	}}

	title := GenerateIssueTitle(event)
	if title == "" || title[:9] != "TypeError" {
		t.Fatalf("expected exception-based title, got %q", title)
	}

	culprit := ExtractCulprit(event)
	if culprit != "HandleRequest in handler.go" {
		t.Fatalf("unexpected culprit %q", culprit)
	}
}

func TestCalculateIssueSimilarity(t *testing.T) {
	event := NewErrorEvent("project_1", "event_1")
	event.Platform = "go"
	event.Logger = "app"
	event.Exception = []ExceptionData{{
		Type: "TypeError",
		Stacktrace: &StacktraceData{Frames: []FrameData{
			{Filename: "handler.go", Function: "HandleRequest", InApp: true},
		}},
	}}

	issue := &Issue{
		Title:    "TypeError: bad request",
		Platform: "go",
		Logger:   "app",
		Culprit:  "HandleRequest in handler.go",
	}

	if got := CalculateIssueSimilarity(event, issue); got <= 0.5 {
		t.Fatalf("expected meaningful similarity score, got %f", got)
	}
}

func TestFingerprintComponentHelpers(t *testing.T) {
	event := NewErrorEvent("project_1", "event_1")
	event.Platform = "go"
	event.Logger = "worker"
	event.Message = "Order 1234 failed in /srv/app/orders/"
	event.Exception = []ExceptionData{{
		Type:   "TypeError",
		Value:  "Order 1234 failed for 550e8400-e29b-41d4-a716-446655440000",
		Module: "orders",
		Stacktrace: &StacktraceData{Frames: []FrameData{
			{Filename: "vendor.js", Function: "wrap", LineNumber: 10, InApp: false},
			{Filename: "handler.go", Function: "Handle", LineNumber: 42, InApp: true},
		}},
	}}

	components := ExtractFingerprintComponents(event)
	if components.ErrorType != "TypeError" {
		t.Fatalf("unexpected error type %q", components.ErrorType)
	}
	if components.Module != "orders" {
		t.Fatalf("unexpected module %q", components.Module)
	}
	if len(components.StackTrace) != 1 || !strings.Contains(components.StackTrace[0], "handler.go:Handle:42") {
		t.Fatalf("unexpected stack trace components %#v", components.StackTrace)
	}

	normalized := NormalizeErrorMessage("Order 1234 failed for abcdef1234567890 at /srv/app/orders/ and 550e8400-e29b-41d4-a716-446655440000")
	if strings.Contains(normalized, "1234") || strings.Contains(normalized, "abcdef1234567890") || strings.Contains(normalized, "550e8400") {
		t.Fatalf("expected normalized message to scrub identifiers, got %q", normalized)
	}

	hash := HashFingerprintComponents(components)
	if hash == "" || hash != HashFingerprintComponents(components) {
		t.Fatalf("expected stable hash, got %q", hash)
	}

	if got := TruncateMessage("abcdef", 5); got != "ab..." {
		t.Fatalf("unexpected truncation result %q", got)
	}
	if got := ReplacePattern("job-123", `\d+`, "N"); got != "job-N" {
		t.Fatalf("unexpected replace result %q", got)
	}
}

func TestIssueTitleAndCulpritFallbacks(t *testing.T) {
	typeOnly := NewErrorEvent("project_1", "event_1")
	typeOnly.Exception = []ExceptionData{{Type: "TypeError"}}
	if got := GenerateIssueTitle(typeOnly); got != "TypeError" {
		t.Fatalf("expected type-only title, got %q", got)
	}

	messageOnly := NewErrorEvent("project_1", "event_2")
	messageOnly.Message = "plain message"
	if got := GenerateIssueTitle(messageOnly); got != "plain message" {
		t.Fatalf("expected message title, got %q", got)
	}

	unknown := NewErrorEvent("project_1", "event_3")
	if got := GenerateIssueTitle(unknown); got != "Unknown Error" {
		t.Fatalf("expected unknown fallback, got %q", got)
	}

	stacktraceOnly := NewErrorEvent("project_1", "event_4")
	stacktraceOnly.Stacktrace = &StacktraceData{Frames: []FrameData{
		{Filename: "bg.go", Function: "Run", InApp: true},
	}}
	if got := ExtractCulprit(stacktraceOnly); got != "Run in bg.go" {
		t.Fatalf("expected stacktrace culprit, got %q", got)
	}

	loggerOnly := NewErrorEvent("project_1", "event_5")
	loggerOnly.Logger = "worker"
	if got := ExtractCulprit(loggerOnly); got != "worker" {
		t.Fatalf("expected logger culprit, got %q", got)
	}

	platformOnly := NewErrorEvent("project_1", "event_6")
	platformOnly.Platform = "go"
	if got := ExtractCulprit(platformOnly); got != "go" {
		t.Fatalf("expected platform culprit, got %q", got)
	}
}
