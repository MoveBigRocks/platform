package extensionruntime

import (
	"context"
	"fmt"
	"net/http/httptest"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/container"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type Registry struct {
	httpHandlers          map[string]gin.HandlerFunc
	eventConsumerHandlers map[string]func(context.Context, []byte) error
	scheduledJobHandlers  map[string]func(context.Context) error
	closeFns              []func() error
	runtimeDir            string
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
	if c != nil && c.Config != nil {
		registry.runtimeDir = c.Config.ExtensionRuntimeDir
	}

	registerEnterpriseAccessTargets(registry, c)

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

func (r *Registry) SupportsServiceTarget(protocol, serviceTarget string) bool {
	serviceTarget = strings.TrimSpace(serviceTarget)
	switch strings.TrimSpace(protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		return serviceTarget != ""
	case "", platformdomain.ExtensionRuntimeProtocolInProcessHTTP:
		return r.Has(serviceTarget)
	default:
		return false
	}
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
