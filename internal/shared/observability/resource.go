package observability

import (
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// newOTelResource builds the shared OTel resource attributes used by both
// the tracer and meter providers.
func newOTelResource(cfg *config.Config, version config.AppVersion) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.AppName),
		semconv.ServiceVersion(string(version)),
		semconv.DeploymentEnvironmentName(cfg.AppEnv),
	)
}
