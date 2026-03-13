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
		tp := sdktrace.NewTracerProvider()
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			slog.Error("otel internal error", "err", err)
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
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.OTLPSampleRate)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Error("otel internal error", "err", err)
	}))

	return tp, nil
}
