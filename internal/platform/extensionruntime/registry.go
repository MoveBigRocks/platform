package extensionruntime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	analyticshandlers "github.com/movebigrocks/platform/internal/analytics/handlers"
	"github.com/movebigrocks/platform/internal/infrastructure/container"
	observabilityhandlers "github.com/movebigrocks/platform/internal/observability/handlers"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	platformhandlers "github.com/movebigrocks/platform/internal/platform/handlers"
	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

type Registry struct {
	httpHandlers          map[string]gin.HandlerFunc
	eventConsumerHandlers map[string]func(context.Context, []byte) error
	scheduledJobHandlers  map[string]func(context.Context) error
	closeFns              []func() error
}

type ProbeResult struct {
	StatusCode int
	Body       []byte
}

func NewRegistry(c *container.Container) *Registry {
	registry := &Registry{
		httpHandlers:          make(map[string]gin.HandlerFunc),
		eventConsumerHandlers: make(map[string]func(context.Context, []byte) error),
		scheduledJobHandlers:  make(map[string]func(context.Context) error),
	}

	registerEnterpriseAccessTargets(registry, c)

	analyticsScriptHandler := analyticshandlers.NewAnalyticsScriptHandler()
	analyticsAdminHandler := analyticshandlers.NewAnalyticsAdminHandler()
	registry.Register("analytics.asset.script", analyticsScriptHandler.ServeScript)
	registry.Register("analytics.admin.properties", analyticsAdminHandler.ShowAnalyticsProperties)
	registry.Register("analytics.admin.property.dashboard", analyticsAdminHandler.ShowPropertyDashboard)
	registry.Register("analytics.admin.property.setup", analyticsAdminHandler.ShowPropertySetup)
	registry.Register("analytics.admin.property.settings", analyticsAdminHandler.ShowPropertySettings)

	if c != nil && c.Config != nil {
		analyticsRuntime, err := newAnalyticsRuntime(c.Config.Database.EffectiveDSN(), c.GeoIP, c.Logger)
		if err != nil {
			if c.Logger != nil {
				c.Logger.Warn("Failed to initialize analytics extension runtime", "error", err)
			}
		} else {
			registry.closeFns = append(registry.closeFns, analyticsRuntime.Close)
			analyticsAPIHandler := analyticshandlers.NewAnalyticsExtensionAPIHandler(
				analyticsRuntime.Query,
				c.Config.Server.APIBaseURL,
			)
			analyticsIngestHandler := analyticshandlers.NewAnalyticsIngestHandler(
				analyticsRuntime.Ingest,
				logger.New().WithField("handler", "analytics-ingest"),
			)
			registry.Register("analytics.ingest.event", analyticsIngestHandler.HandleEvent)
			registry.Register("analytics.api.properties", analyticsAPIHandler.ListProperties)
			registry.Register("analytics.api.properties.create", analyticsAPIHandler.CreateProperty)
			registry.Register("analytics.api.property.get", analyticsAPIHandler.GetProperty)
			registry.Register("analytics.api.property.update", analyticsAPIHandler.UpdateProperty)
			registry.Register("analytics.api.property.delete", analyticsAPIHandler.DeleteProperty)
			registry.Register("analytics.api.property.reset", analyticsAPIHandler.ResetProperty)
			registry.Register("analytics.api.property.current_visitors", analyticsAPIHandler.CurrentVisitors)
			registry.Register("analytics.api.property.verify", analyticsAPIHandler.VerifyInstallation)
			registry.Register("analytics.api.property.metrics", analyticsAPIHandler.Metrics)
			registry.Register("analytics.api.property.timeseries", analyticsAPIHandler.TimeSeries)
			registry.Register("analytics.api.property.breakdown", analyticsAPIHandler.Breakdown)
			registry.Register("analytics.api.property.goals", analyticsAPIHandler.ListGoals)
			registry.Register("analytics.api.property.goal_results", analyticsAPIHandler.GoalResults)
			registry.Register("analytics.api.property.goal.create", analyticsAPIHandler.CreateGoal)
			registry.Register("analytics.api.goal.delete", analyticsAPIHandler.DeleteGoal)
			registry.Register("analytics.api.property.hostname_rules", analyticsAPIHandler.ListHostnameRules)
			registry.Register("analytics.api.property.hostname_rule.create", analyticsAPIHandler.CreateHostnameRule)
			registry.Register("analytics.api.hostname_rule.delete", analyticsAPIHandler.DeleteHostnameRule)
			registry.Register("analytics.runtime.health", func(ctx *gin.Context) {
				ctx.JSON(http.StatusOK, gin.H{
					"status":  "healthy",
					"message": "analytics runtime ready",
				})
			})
			registry.RegisterScheduledJob("analytics.job.maintenance", func(ctx context.Context) error {
				return analyticsRuntime.RunMaintenance(ctx)
			})
		}
	}

	if c != nil && c.Observability != nil && c.Store != nil {
		sentryIngestHandler := observabilityhandlers.NewSentryIngestHandler(
			c.Observability.Project,
			c.Store.ErrorEvents(),
			c.Observability.ErrorProcessor,
			logger.New().WithField("handler", "sentry-ingest"),
		)
		registry.Register("error-tracking.ingest.envelope", sentryIngestHandler.HandleEnvelope)
		registry.Register("error-tracking.ingest.envelope.project", sentryIngestHandler.HandleEnvelopeWithProject)
		registry.Register("error-tracking.runtime.health", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"message": "error tracking runtime ready",
			})
		})
		errorEventHandler := observabilityhandlers.NewErrorEventHandler(
			c.Observability.ErrorProcessor,
			logger.New().WithField("handler", "error-tracking-consumer-errors"),
		)
		issueEventHandler := observabilityhandlers.NewIssueEventHandler(
			c.Observability.Issue,
			logger.New().WithField("handler", "error-tracking-consumer-issues"),
		)
		registry.RegisterEventConsumer("error-tracking.consumer.errors", errorEventHandler.HandleErrorEvent)
		registry.RegisterEventConsumer("error-tracking.consumer.issue-events", func(ctx context.Context, data []byte) error {
			switch eventType := strings.TrimSpace(eventbus.ParseEventType(data)); eventType {
			case "", "issue.created":
				return issueEventHandler.HandleIssueCreated(ctx, data)
			case "issue.updated":
				return issueEventHandler.HandleIssueUpdated(ctx, data)
			case "issue.resolved":
				return issueEventHandler.HandleIssueResolved(ctx, data)
			default:
				return nil
			}
		})

		if c.Service != nil && c.Service.Case != nil {
			issueCaseService := observabilityservices.NewIssueCaseService(
				c.Store.Cases(),
				c.Service.Case,
			)
			caseEventHandler := servicehandlers.NewCaseEventHandler(
				issueCaseService,
				nil,
				nil,
				c.Store,
				logger.New().WithField("handler", "error-tracking-consumer-case-links"),
			)
			registry.RegisterEventConsumer("error-tracking.consumer.case-events", caseEventHandler.HandleErrorTrackingCaseEvent)
		}

		if c.Platform != nil && c.Service != nil && c.Automation != nil {
			adminManagementHandler := platformhandlers.NewAdminManagementHandler(
				c.Platform.Workspace,
				c.Platform.User,
				c.Platform.Stats,
				c.Platform.Extension,
				c.Service.Case,
				c.Automation.Rule,
				c.Automation.Form,
				c.Observability.Issue,
				c.Observability.Project,
			)
			registry.Register("error-tracking.admin.issues", adminManagementHandler.ShowIssues)
			registry.Register("error-tracking.admin.issue.detail", adminManagementHandler.ShowIssueDetail)
			registry.Register("error-tracking.admin.applications", adminManagementHandler.ShowApplications)
			registry.Register("error-tracking.admin.application.detail", adminManagementHandler.ShowApplicationDetail)
			registry.Register("error-tracking.admin.application.get", adminManagementHandler.GetApplication)
			registry.Register("error-tracking.admin.application.create", adminManagementHandler.CreateApplication)
			registry.Register("error-tracking.admin.application.update", adminManagementHandler.UpdateApplication)
			registry.Register("error-tracking.admin.application.delete", adminManagementHandler.DeleteApplication)
		}
	}

	return registry
}

func (r *Registry) Close() error {
	if r == nil {
		return nil
	}
	var firstErr error
	for _, closeFn := range r.closeFns {
		if closeFn == nil {
			continue
		}
		if err := closeFn(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Registry) Register(serviceTarget string, handler gin.HandlerFunc) {
	if r == nil || handler == nil {
		return
	}
	if r.httpHandlers == nil {
		r.httpHandlers = make(map[string]gin.HandlerFunc)
	}
	r.httpHandlers[serviceTarget] = handler
}

func (r *Registry) RegisterEventConsumer(serviceTarget string, handler func(context.Context, []byte) error) {
	if r == nil || handler == nil {
		return
	}
	if r.eventConsumerHandlers == nil {
		r.eventConsumerHandlers = make(map[string]func(context.Context, []byte) error)
	}
	r.eventConsumerHandlers[serviceTarget] = handler
}

func (r *Registry) RegisterScheduledJob(serviceTarget string, handler func(context.Context) error) {
	if r == nil || handler == nil {
		return
	}
	if r.scheduledJobHandlers == nil {
		r.scheduledJobHandlers = make(map[string]func(context.Context) error)
	}
	r.scheduledJobHandlers[serviceTarget] = handler
}

func (r *Registry) Has(serviceTarget string) bool {
	if r == nil {
		return false
	}
	if _, ok := r.httpHandlers[serviceTarget]; ok {
		return true
	}
	if _, ok := r.eventConsumerHandlers[serviceTarget]; ok {
		return true
	}
	_, ok := r.scheduledJobHandlers[serviceTarget]
	return ok
}

func (r *Registry) Dispatch(serviceTarget string, ctx *gin.Context) bool {
	if r == nil || ctx == nil {
		return false
	}
	handler, ok := r.httpHandlers[serviceTarget]
	if !ok {
		return false
	}
	handler(ctx)
	return true
}

func (r *Registry) Consume(serviceTarget string, ctx context.Context, data []byte) error {
	if r == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	handler, ok := r.eventConsumerHandlers[serviceTarget]
	if !ok {
		return fmt.Errorf("service target %s is not registered", serviceTarget)
	}
	return handler(ctx, data)
}

func (r *Registry) RunJob(serviceTarget string, ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	handler, ok := r.scheduledJobHandlers[serviceTarget]
	if !ok {
		return fmt.Errorf("service target %s is not registered", serviceTarget)
	}
	return handler(ctx)
}

func (r *Registry) Probe(serviceTarget, method, requestPath string, params map[string]string) (ProbeResult, error) {
	if r == nil {
		return ProbeResult{}, fmt.Errorf("service target registry is not configured")
	}
	handler, ok := r.httpHandlers[serviceTarget]
	if !ok {
		return ProbeResult{}, fmt.Errorf("service target %s is not registered", serviceTarget)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, requestPath, nil)
	ctx.Request = req
	ApplyRouteParams(ctx, params)
	handler(ctx)

	return ProbeResult{
		StatusCode: recorder.Code,
		Body:       append([]byte(nil), recorder.Body.Bytes()...),
	}, nil
}

func ApplyRouteParams(ctx *gin.Context, params map[string]string) {
	if ctx == nil || len(params) == 0 {
		return
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	filtered := make(gin.Params, 0, len(ctx.Params)+len(params))
	for _, existing := range ctx.Params {
		if _, overridden := params[existing.Key]; overridden {
			continue
		}
		filtered = append(filtered, existing)
	}
	for _, key := range keys {
		filtered = append(filtered, gin.Param{Key: key, Value: params[key]})
	}
	ctx.Params = filtered
}
