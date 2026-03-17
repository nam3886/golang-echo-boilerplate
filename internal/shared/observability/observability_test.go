package observability

import (
	"log/slog"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestChooseSampler_Development_ReturnsAlwaysSample(t *testing.T) {
	cfg := &config.Config{AppEnv: "development"}
	sampler := chooseSampler(cfg)
	if sampler.Description() != sdktrace.AlwaysSample().Description() {
		t.Errorf("expected AlwaysSample in dev, got %s", sampler.Description())
	}
}

func TestChooseSampler_Production_ReturnsRatioBased(t *testing.T) {
	cfg := &config.Config{AppEnv: "production", OTLPSampleRate: 0.01}
	sampler := chooseSampler(cfg)
	expected := sdktrace.TraceIDRatioBased(0.01).Description()
	if sampler.Description() != expected {
		t.Errorf("expected %s in prod, got %s", expected, sampler.Description())
	}
}

func TestChooseSampler_Staging_ReturnsRatioBased(t *testing.T) {
	cfg := &config.Config{AppEnv: "staging", OTLPSampleRate: 0.1}
	sampler := chooseSampler(cfg)
	if sampler.Description() == sdktrace.AlwaysSample().Description() {
		t.Error("staging must not use AlwaysSample")
	}
}
