package observabilitydomain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// FingerprintComponents holds the components used for generating error fingerprints.
type FingerprintComponents struct {
	ErrorType    string
	ErrorMessage string
	StackTrace   []string
	Platform     string
	Logger       string
	Module       string
}

// GenerateAdvancedFingerprint produces a normalized fingerprint for grouping.
func GenerateAdvancedFingerprint(event *ErrorEvent) string {
	components := extractFingerprintComponents(event)
	return hashFingerprintComponents(components)
}

func ExtractFingerprintComponents(event *ErrorEvent) *FingerprintComponents {
	return extractFingerprintComponents(event)
}

// GenerateIssueTitle derives the canonical issue title from an event.
func GenerateIssueTitle(event *ErrorEvent) string {
	if len(event.Exception) > 0 {
		exc := event.Exception[0]
		if exc.Type != "" && exc.Value != "" {
			return fmt.Sprintf("%s: %s", exc.Type, truncateMessage(exc.Value, 100))
		}
		if exc.Type != "" {
			return exc.Type
		}
	}

	if event.Message != "" {
		return truncateMessage(event.Message, 100)
	}

	return "Unknown Error"
}

// ExtractCulprit derives the best culprit string for an event.
func ExtractCulprit(event *ErrorEvent) string {
	if len(event.Exception) > 0 && event.Exception[0].Stacktrace != nil {
		frames := event.Exception[0].Stacktrace.Frames
		for i := len(frames) - 1; i >= 0; i-- {
			frame := frames[i]
			if frame.InApp && frame.Function != "" {
				return fmt.Sprintf("%s in %s", frame.Function, frame.Filename)
			}
		}
	}

	if event.Stacktrace != nil {
		for i := len(event.Stacktrace.Frames) - 1; i >= 0; i-- {
			frame := event.Stacktrace.Frames[i]
			if frame.InApp && frame.Function != "" {
				return fmt.Sprintf("%s in %s", frame.Function, frame.Filename)
			}
		}
	}

	if event.Logger != "" {
		return event.Logger
	}

	return event.Platform
}

// CalculateIssueSimilarity scores how likely an event belongs to an existing issue.
func CalculateIssueSimilarity(event *ErrorEvent, issue *Issue) float64 {
	var score float64
	var factors int

	if len(event.Exception) > 0 && issue.Title != "" {
		eventType := event.Exception[0].Type
		if eventType != "" && strings.Contains(issue.Title, eventType) {
			score += 0.4
		}
		factors++
	}

	if event.Platform == issue.Platform {
		score += 0.2
	}
	factors++

	if event.Logger == issue.Logger {
		score += 0.2
	}
	factors++

	if CompareIssueStackTrace(event, issue) {
		score += 0.3
	}
	factors++

	if factors == 0 {
		return 0
	}
	return score
}

func NormalizeErrorMessage(message string) string {
	return normalizeErrorMessage(message)
}

func HashFingerprintComponents(components *FingerprintComponents) string {
	return hashFingerprintComponents(components)
}

func TruncateMessage(msg string, maxLen int) string {
	return truncateMessage(msg, maxLen)
}

func ReplacePattern(text, pattern, replacement string) string {
	return replacePattern(text, pattern, replacement)
}

// CompareIssueStackTrace checks whether the event and issue share the same top stack frame.
func CompareIssueStackTrace(event *ErrorEvent, issue *Issue) bool {
	if len(event.Exception) > 0 && event.Exception[0].Stacktrace != nil {
		frames := event.Exception[0].Stacktrace.Frames
		if len(frames) > 0 {
			topFrame := frames[len(frames)-1]
			if topFrame.Filename != "" && strings.Contains(issue.Culprit, topFrame.Filename) {
				return true
			}
		}
	}
	return false
}

func extractFingerprintComponents(event *ErrorEvent) *FingerprintComponents {
	components := &FingerprintComponents{
		Platform: event.Platform,
		Logger:   event.Logger,
	}

	if len(event.Exception) > 0 {
		exc := event.Exception[0]
		components.ErrorType = exc.Type
		components.ErrorMessage = normalizeErrorMessage(exc.Value)
		components.Module = exc.Module

		if exc.Stacktrace != nil {
			for _, frame := range exc.Stacktrace.Frames {
				if frame.InApp {
					components.StackTrace = append(components.StackTrace, fmt.Sprintf("%s:%s:%d", frame.Filename, frame.Function, frame.LineNumber))
				}
			}
		}
	} else {
		components.ErrorMessage = normalizeErrorMessage(event.Message)
	}

	if len(components.StackTrace) == 0 && event.Stacktrace != nil {
		for _, frame := range event.Stacktrace.Frames {
			if frame.InApp {
				components.StackTrace = append(components.StackTrace, fmt.Sprintf("%s:%s:%d", frame.Filename, frame.Function, frame.LineNumber))
			}
		}
	}

	return components
}

func normalizeErrorMessage(message string) string {
	message = strings.ToLower(message)
	message = replacePattern(message, `\b\d+\b`, "N")
	message = replacePattern(message, `\b[0-9a-f]{8,}\b`, "HEX")
	message = replacePattern(message, `\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`, "UUID")
	message = replacePattern(message, `[/\\][\w/\\.-]+[/\\]`, "")
	return message
}

func hashFingerprintComponents(components *FingerprintComponents) string {
	var parts []string

	if components.ErrorType != "" {
		parts = append(parts, "type:"+components.ErrorType)
	}
	if components.ErrorMessage != "" {
		parts = append(parts, "msg:"+components.ErrorMessage)
	}
	if components.Platform != "" {
		parts = append(parts, "platform:"+components.Platform)
	}
	if components.Logger != "" {
		parts = append(parts, "logger:"+components.Logger)
	}
	if components.Module != "" {
		parts = append(parts, "module:"+components.Module)
	}

	stackFrames := components.StackTrace
	if len(stackFrames) > 3 {
		stackFrames = stackFrames[:3]
	}
	for i, frame := range stackFrames {
		parts = append(parts, fmt.Sprintf("frame%d:%s", i, frame))
	}

	sort.Strings(parts)
	content := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen-3] + "..."
}

func replacePattern(text, pattern, replacement string) string {
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(text, replacement)
}
