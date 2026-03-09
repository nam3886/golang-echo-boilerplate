package observability

import (
	"context"
	"fmt"

	"github.com/gnha/gnha-services/internal/shared/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// NewMeterProvider creates an OTel meter provider exporting to OTLP.
// When OTLPEndpoint is empty, returns a no-op provider to avoid silent connection failures.
func NewMeterProvider(cfg *config.Config, version config.AppVersion) (*sdkmetric.MeterProvider, error) {
	if cfg.OTLPEndpoint == "" {
		mp := sdkmetric.NewMeterProvider()
		otel.SetMeterProvider(mp)
		return mp, nil
	}

	ctx := context.Background()

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpointURL(cfg.OTLPEndpoint),
	}
	if cfg.IsDevelopment() {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.AppName),
			semconv.ServiceVersion(string(version)),
			semconv.DeploymentEnvironmentName(cfg.AppEnv),
		)),
	)

	otel.SetMeterProvider(mp)
	return mp, nil
}
