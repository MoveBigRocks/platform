package observabilityhandlers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

type sentryEventStore interface {
	CreateErrorEvent(ctx context.Context, event *observabilitydomain.ErrorEvent) error
}

type sentryProjectStore interface {
	GetProjectByKey(ctx context.Context, projectKey string) (*observabilitydomain.Project, error)
	IncrementEventCount(ctx context.Context, workspaceID, projectID string, lastEventAt *time.Time) (int64, error)
}

type sentryEventProcessor interface {
	ProcessEvent(ctx context.Context, event *observabilitydomain.ErrorEvent) error
}

type SentryIngestHandler struct {
	projectStore    sentryProjectStore
	errorEventStore sentryEventStore
	processor       sentryEventProcessor
	logger          *logger.Logger
}

func NewSentryIngestHandler(
	projectStore sentryProjectStore,
	errorEventStore sentryEventStore,
	processor sentryEventProcessor,
	log *logger.Logger,
) *SentryIngestHandler {
	if log == nil {
		log = logger.NewNop()
	}

	return &SentryIngestHandler{
		projectStore:    projectStore,
		errorEventStore: errorEventStore,
		processor:       processor,
		logger:          log,
	}
}

func (h *SentryIngestHandler) HandleEnvelope(c *gin.Context) {
	h.handleEnvelope(c, "")
}

func (h *SentryIngestHandler) HandleEnvelopeWithProject(c *gin.Context) {
	h.handleEnvelope(c, c.Param("projectNumber"))
}

func (h *SentryIngestHandler) handleEnvelope(c *gin.Context, expectedProjectNumber string) {
	if err := validateSentryEnvelopeRequest(c); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	publicKey, ok := parseSentryAuth(c.GetHeader("X-Sentry-Auth"))
	if !ok {
		middleware.RespondWithError(c, http.StatusUnauthorized, "invalid sentry auth header")
		return
	}

	ctx := c.Request.Context()
	project, err := h.projectStore.GetProjectByKey(ctx, publicKey)
	if err != nil || project == nil || !project.IsActive() {
		middleware.RespondWithError(c, http.StatusUnauthorized, "invalid project credentials")
		return
	}

	if expectedProjectNumber != "" {
		numberFromPath, err := strconv.ParseInt(expectedProjectNumber, 10, 64)
		if err != nil {
			middleware.RespondWithError(c, http.StatusBadRequest, "invalid project number")
			return
		}

		if project.ProjectNumber != numberFromPath {
			middleware.RespondWithError(c, http.StatusUnauthorized, "project number mismatch")
			return
		}
	}

	binaryBody, err := readSentryEnvelopeBody(c.Request.Body, c.GetHeader("Content-Encoding"))
	if err != nil {
		h.logger.WithError(err).Warn("failed to read sentry envelope body")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	eventData, err := parseSentryEnvelope(binaryBody)
	if err != nil {
		h.logger.WithError(err).Warn("failed to parse sentry envelope")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid envelope format")
		return
	}

	event, err := convertSentryEvent(eventData, project.ID)
	if err != nil {
		h.logger.WithError(err).Warn("failed to convert sentry event")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid event payload")
		return
	}

	if err := h.errorEventStore.CreateErrorEvent(ctx, event); err != nil {
		h.logger.WithError(err).Error("failed to persist error event")
		middleware.RespondWithError(c, http.StatusInternalServerError, "failed to store event")
		return
	}

	if err := h.processor.ProcessEvent(ctx, event); err != nil {
		h.logger.WithError(err).Error("failed to process error event")
		middleware.RespondWithError(c, http.StatusInternalServerError, "failed to process event")
		return
	}

	if _, err := h.projectStore.IncrementEventCount(ctx, project.WorkspaceID, project.ID, &event.Timestamp); err != nil {
		h.logger.WithError(err).Warn("failed to update project event count")
	}

	c.JSON(http.StatusOK, contracts.IngestResponse{
		Success: true,
		EventID: event.EventID,
		IssueID: event.IssueID,
	})
}

func validateSentryEnvelopeRequest(c *gin.Context) error {
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if contentType == "" {
		return fmt.Errorf("invalid content type: expected application/x-sentry-envelope")
	}

	if !strings.Contains(contentType, "application/x-sentry-envelope") {
		return fmt.Errorf("invalid content type: expected application/x-sentry-envelope")
	}

	return nil
}

func readSentryEnvelopeBody(body io.ReadCloser, contentEncoding string) ([]byte, error) {
	defer body.Close()

	encoded := strings.ToLower(contentEncoding)
	if strings.Contains(encoded, "gzip") {
		gz, err := gzip.NewReader(body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		return io.ReadAll(gz)
	}

	return io.ReadAll(body)
}

func parseSentryAuth(header string) (string, bool) {
	if header == "" || !strings.HasPrefix(header, "Sentry ") {
		return "", false
	}

	parts := strings.Split(strings.TrimPrefix(header, "Sentry "), ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == "sentry_key" {
			return kv[1], true
		}
	}

	return "", false
}

func parseSentryEnvelope(body []byte) (map[string]interface{}, error) {
	lines := strings.Split(string(body), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("malformed envelope")
	}

	// Header line is required for project-level metadata.
	var envelopeHeader map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(lines[0])), &envelopeHeader); err != nil {
		return nil, fmt.Errorf("invalid envelope header")
	}

	_ = envelopeHeader

	for i := 1; i < len(lines); i += 2 {
		headerLine := strings.TrimSpace(lines[i])
		if headerLine == "" {
			continue
		}

		if i+1 >= len(lines) {
			break
		}
		dataLine := strings.TrimSpace(lines[i+1])
		if dataLine == "" {
			continue
		}

		var itemHeader map[string]interface{}
		if err := json.Unmarshal([]byte(headerLine), &itemHeader); err != nil {
			continue
		}

		itemType := asString(itemHeader["type"])
		if itemType == "" {
			itemType = "event"
		}

		if itemType != "event" && itemType != "transaction" {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(dataLine), &payload); err != nil {
			continue
		}

		if eventID, ok := payload["event_id"].(string); ok && eventID != "" {
			// keep normalized event IDs when present
			payload["event_id"] = strings.ReplaceAll(eventID, "-", "")
		}

		return payload, nil
	}

	return nil, fmt.Errorf("no event item found")
}

func convertSentryEvent(data map[string]interface{}, projectID string) (*observabilitydomain.ErrorEvent, error) {
	eventID := asString(data["event_id"])
	if eventID == "" {
		eventID = strings.ReplaceAll(id.New(), "-", "")
	}

	event := observabilitydomain.NewErrorEvent(projectID, eventID)

	event.Message = firstNonEmptyString(
		asString(data["message"]),
		func() string {
			if logEntry, ok := data["logentry"].(map[string]interface{}); ok {
				return asString(logEntry["formatted"])
			}
			return ""
		}(),
	)
	event.Level = asString(data["level"])
	event.Platform = asString(data["platform"])
	event.Logger = asString(data["logger"])
	event.Environment = asString(data["environment"])
	event.Release = asString(data["release"])
	event.Dist = asString(data["dist"])

	if timestamp := asString(data["timestamp"]); timestamp != "" {
		if parsed, err := parseSentryTimestamp(timestamp); err == nil {
			event.Timestamp = parsed
		}
	}

	if exceptions, ok := data["exception"].([]interface{}); ok {
		for _, exception := range exceptions {
			exceptionData, ok := exception.(map[string]interface{})
			if !ok {
				continue
			}

			exc := observabilitydomain.ExceptionData{
				Type:  asString(exceptionData["type"]),
				Value: asString(exceptionData["value"]),
			}

			if stackData, ok := exceptionData["stacktrace"].(map[string]interface{}); ok {
				if frames, ok := stackData["frames"].([]interface{}); ok {
					framesData := make([]observabilitydomain.FrameData, 0, len(frames))
					for _, frame := range frames {
						frameData, ok := frame.(map[string]interface{})
						if !ok {
							continue
						}
						framesData = append(framesData, observabilitydomain.FrameData{
							Filename:    asString(frameData["filename"]),
							Function:    asString(frameData["function"]),
							Module:      asString(frameData["module"]),
							LineNumber:  asInt(frameData["lineno"]),
							ColNumber:   asInt(frameData["colno"]),
							AbsPath:     asString(frameData["abs_path"]),
							ContextLine: asString(frameData["context_line"]),
							InApp:       asBool(frameData["in_app"]),
							Vars:        shareddomain.MetadataFromMap(asMetadataMap(frameData, "vars")),
						})
					}
					exc.Stacktrace = &observabilitydomain.StacktraceData{Frames: framesData}
				}
			}

			event.Exception = append(event.Exception, exc)
		}
	}

	if userData, ok := data["user"].(map[string]interface{}); ok {
		event.User = &observabilitydomain.UserContext{
			ID:       asString(userData["id"]),
			Email:    asString(userData["email"]),
			Username: asString(userData["username"]),
			IPAddr:   asString(userData["ip_address"]),
		}
	}

	event.Tags = asStringMap(data["tags"])
	event.Extra = shareddomain.MetadataFromMap(asMetadataMap(data, "extra"))
	event.Contexts = shareddomain.MetadataFromMap(asMetadataMap(data, "contexts"))

	if breadcrumbs, ok := data["breadcrumbs"].([]interface{}); ok {
		for _, breadcrumb := range breadcrumbs {
			breadcrumbData, ok := breadcrumb.(map[string]interface{})
			if !ok {
				continue
			}

			ts := time.Time{}
			if tsStr, ok := breadcrumbData["timestamp"].(string); ok && tsStr != "" {
				if parsed, err := parseSentryTimestamp(tsStr); err == nil {
					ts = parsed
				}
			}

			event.Breadcrumbs = append(event.Breadcrumbs, observabilitydomain.Breadcrumb{
				Timestamp: ts,
				Message:   asString(breadcrumbData["message"]),
				Category:  asString(breadcrumbData["category"]),
				Level:     asString(breadcrumbData["level"]),
				Type:      asString(breadcrumbData["type"]),
				Data:      shareddomain.MetadataFromMap(asMetadataMap(breadcrumbData, "data")),
			})
		}
	}

	if requestData, ok := data["request"].(map[string]interface{}); ok {
		event.Request = &observabilitydomain.RequestContext{
			URL:         asString(requestData["url"]),
			Method:      asString(requestData["method"]),
			Data:        shareddomain.MetadataFromMap(asMetadataMap(requestData, "data")),
			Cookies:     asStringMap(requestData["cookies"]),
			QueryString: asString(requestData["query_string"]),
			Headers:     asStringMap(requestData["headers"]),
		}
	}

	return event, nil
}

func parseSentryTimestamp(raw string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid timestamp")
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}

	if value, ok := v.(string); ok {
		return value
	}

	return fmt.Sprintf("%v", v)
}

func asBool(v interface{}) bool {
	if v == nil {
		return false
	}

	if b, ok := v.(bool); ok {
		return b
	}

	if value, ok := v.(string); ok {
		return value == "1" || strings.EqualFold(value, "true")
	}

	if num, ok := v.(float64); ok {
		return num != 0
	}

	return false
}

func asInt(v interface{}) int {
	if v == nil {
		return 0
	}

	if value, ok := v.(float64); ok {
		return int(value)
	}

	if value, ok := v.(int); ok {
		return value
	}

	if value, ok := v.(int64); ok {
		return int(value)
	}

	if value, ok := v.(string); ok {
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			return n
		}
	}

	return 0
}

func asMetadataMap(src map[string]interface{}, key string) map[string]interface{} {
	if val, ok := src[key]; ok {
		if metadata, ok := val.(map[string]interface{}); ok {
			return metadata
		}
	}

	if inner, ok := src[key].(map[string]interface{}); ok {
		return inner
	}

	return map[string]interface{}{}
}

func asStringMap(v interface{}) map[string]string {
	if v == nil {
		return map[string]string{}
	}

	if rawMap, ok := v.(map[string]interface{}); ok {
		result := make(map[string]string, len(rawMap))
		for key, value := range rawMap {
			result[key] = asString(value)
		}
		return result
	}

	if tags, ok := v.([]interface{}); ok {
		result := make(map[string]string)
		for _, tag := range tags {
			tuple, ok := tag.([]interface{})
			if !ok || len(tuple) != 2 {
				continue
			}

			result[asString(tuple[0])] = asString(tuple[1])
		}
		return result
	}

	return map[string]string{}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
