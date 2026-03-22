package tracing

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Middleware returns a Gin middleware that instruments HTTP requests with tracing.
// It creates a span for each request and propagates trace context.
func Middleware(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
		// Extract trace context from incoming request headers
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Create span name from route pattern
		spanName := c.FullPath()
		if spanName == "" {
			spanName = c.Request.URL.Path
		}
		spanName = fmt.Sprintf("%s %s", c.Request.Method, spanName)

		// Start span with semantic convention attributes
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
				attribute.String("url.full", c.Request.URL.String()),
				attribute.String("url.scheme", c.Request.URL.Scheme),
				attribute.String("user_agent.original", c.Request.UserAgent()),
				attribute.Int64("http.request.header.content-length", c.Request.ContentLength),
				attribute.String("server.address", c.Request.Host),
				attribute.String("client.address", c.ClientIP()),
			),
		)
		defer span.End()

		// Add request ID if present
		if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
			span.SetAttributes(attribute.String("request.id", requestID))
		}

		// Add workspace ID if present in context
		if workspaceID, exists := c.Get("workspace_id"); exists {
			span.SetAttributes(attribute.String("workspace.id", workspaceID.(string)))
		}

		// Store span in context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Record response attributes
		status := c.Writer.Status()
		span.SetAttributes(
			attribute.Int("http.response.status_code", status),
			attribute.Int("http.response.body.size", c.Writer.Size()),
		)

		// Record errors
		if len(c.Errors) > 0 {
			span.SetAttributes(attribute.String("error.message", c.Errors.String()))
			for _, e := range c.Errors {
				span.RecordError(e.Err)
			}
		}

		// Set span status based on HTTP status code
		if status >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
		}
	}
}
