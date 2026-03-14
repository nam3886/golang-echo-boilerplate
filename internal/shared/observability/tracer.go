package observability

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewTracerProvider creates an OTel tracer provider exporting to OTLP.
// When OTLPEndpoint is empty, returns a no-op provider to avoid silent connection failures.
func NewTracerProvider(cfg *config.Config, version config.AppVersion) (*sdktrace.TracerProvider, error) {
	if cfg.OTLPEndpoint == "" {
		// No OTLP endpoint configured: traces are created but immediately discarded.
		// Set OTEL_EXPORTER_OTLP_ENDPOINT to export traces in production.
		slog.Warn("tracing disabled: OTEL_EXPORTER_OTLP_ENDPOINT not set, spans will be dropped")
		tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			slog.Error("otel internal error",
				"module", "observability", "operation", "OtelInternal",
				"error_code", "otel_error", "retryable", false, "err", err)
		}))
		return tp, nil
	}

	ctx := context.Background()

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpointURL(cfg.OTLPEndpoint),
	}
	if cfg.IsDevelopment() {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newOTelResource(cfg, version)),
		sdktrace.WithSampler(chooseSampler(cfg)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Error("otel internal error",
			"module", "observability", "operation", "OtelInternal",
			"error_code", "otel_error", "retryable", false, "err", err)
	}))

	return tp, nil
}

// chooseSampler returns AlwaysSample in development for full trace visibility
// and TraceIDRatioBased in production/staging for cost control.
func chooseSampler(cfg *config.Config) sdktrace.Sampler {
	if cfg.IsDevelopment() {
		return sdktrace.AlwaysSample()
	}
	return sdktrace.TraceIDRatioBased(cfg.OTLPSampleRate)
}
