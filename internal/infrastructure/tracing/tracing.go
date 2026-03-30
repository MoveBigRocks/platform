// Package tracing provides distributed tracing using OpenTelemetry.
// It supports multiple exporters: Jaeger, OTLP, and stdout (for debugging).
//
// Usage:
//
//	tracer, shutdown, err := tracing.Init(cfg.Tracing)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown(context.Background())
//
//	// Use the tracer
//	ctx, span := tracer.Start(ctx, "operation-name")
//	defer span.End()
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
)

// ShutdownFunc is a function that shuts down the tracing provider
type ShutdownFunc func(ctx context.Context) error

// noopShutdown is a no-op shutdown function
func noopShutdown(_ context.Context) error {
	return nil
}

// Init initializes the tracing provider based on configuration.
// Returns the tracer, a shutdown function, and any error.
// If tracing is disabled, returns a noop tracer.
func Init(cfg config.TracingConfig) (trace.Tracer, ShutdownFunc, error) {
	if !cfg.Enabled {
		// Return noop tracer
		return otel.Tracer("noop"), noopShutdown, nil
	}

	// Create resource with service information
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment(cfg.Environment),
			attribute.String("service.component", "mbr-services"),
		),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	switch cfg.Exporter {
	case "jaeger":
		exporter, err = createJaegerExporter(cfg)
	case "otlp":
		exporter, err = createOTLPExporter(cfg)
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "none", "":
		// No exporter, but tracing is enabled for local spans
		exporter = nil
	default:
		return nil, nil, fmt.Errorf("unknown tracing exporter: %s", cfg.Exporter)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Build tracer provider options
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}

	// Add exporter if configured
	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		))
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(opts...)

	// Register as global tracer provider
	otel.SetTracerProvider(tp)

	// Set up propagator for distributed tracing context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	tracer := tp.Tracer(cfg.ServiceName)

	// Shutdown function
	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}

	return tracer, shutdown, nil
}

// createJaegerExporter creates a Jaeger exporter using OTLP HTTP.
// Modern Jaeger (v1.35+) supports OTLP natively, which is the recommended approach.
func createJaegerExporter(cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	// Use OTLP HTTP exporter to Jaeger's OTLP endpoint
	// Jaeger exposes OTLP on port 4318 (HTTP) or 4317 (gRPC)
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(extractHost(cfg.JaegerURL)),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	return otlptracehttp.New(context.Background(), opts...)
}

// createOTLPExporter creates an OTLP gRPC exporter
func createOTLPExporter(cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	client := otlptracegrpc.NewClient(opts...)
	return otlptrace.New(context.Background(), client)
}

// extractHost extracts the host:port from a URL
func extractHost(url string) string {
	// Simple extraction - remove protocol prefix
	if len(url) > 7 && url[:7] == "http://" {
		url = url[7:]
	} else if len(url) > 8 && url[:8] == "https://" {
		url = url[8:]
	}
	// Remove path if present
	for i, c := range url {
		if c == '/' {
			return url[:i]
		}
	}
	return url
}
