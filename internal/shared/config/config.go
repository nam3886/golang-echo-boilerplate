package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// AppVersion is a named type for the application version string.
// Using a distinct type avoids Fx ambiguous-injection errors when
// multiple bare `string` values are registered in the container.
type AppVersion string

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AppEnv  string `env:"APP_ENV" envDefault:"development"`
	AppName string `env:"APP_NAME" envDefault:"golang-echo-boilerplate"`
	Port    int    `env:"PORT" envDefault:"8080"`

	// Database
	DatabaseURL       string        `env:"DATABASE_URL,required"`
	DBMaxConns        int32         `env:"DB_MAX_CONNS" envDefault:"25"`
	DBMinConns        int32         `env:"DB_MIN_CONNS" envDefault:"5"`
	DBMaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`

	// Redis
	RedisURL string `env:"REDIS_URL,required"`

	// RabbitMQ
	RabbitURL string `env:"RABBITMQ_URL,required"`

	// JWT
	JWTSecret     string        `env:"JWT_SECRET,required"`
	JWTAccessTTL  time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	JWTRefreshTTL time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"` // 7 days; shorten to reduce exposure window after credential leak

	// Logging
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// Observability
	OTLPEndpoint   string  `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTLPSampleRate float64 `env:"OTEL_SAMPLING_RATIO" envDefault:"0.01"`

	// SMTP
	SMTPHost      string `env:"SMTP_HOST" envDefault:"localhost"`
	SMTPPort      int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPFrom      string `env:"SMTP_FROM" envDefault:"noreply@app.local"`
	SMTPUser      string `env:"SMTP_USER"`
	SMTPPassword  string `env:"SMTP_PASSWORD"`
	SMTPFromAlias string `env:"SMTP_FROM_ALIAS"`

	// Elasticsearch (optional — empty URL disables search)
	ElasticsearchURL         string `env:"ELASTICSEARCH_URL"`
	ElasticsearchIndexPrefix string `env:"ELASTICSEARCH_INDEX_PREFIX" envDefault:"app"`

	// HTTP
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`

	// Auth
	// BlacklistFailOpen controls behavior when Redis is unreachable during token blacklist check.
	// false (default, fail-closed): reject the request — security over availability.
	// true (fail-open): allow the request — use only when HA is more critical than security + local cache is configured.
	BlacklistFailOpen bool `env:"BLACKLIST_FAIL_OPEN" envDefault:"false"`

	// Rate limiting
	RateLimitRPM        int           `env:"RATE_LIMIT_RPM" envDefault:"100"`
	RateLimitWindow     time.Duration `env:"RATE_LIMIT_WINDOW" envDefault:"1m"`
	RateLimitScope      string        `env:"RATE_LIMIT_SCOPE" envDefault:"per-ip"`
	RateLimitAlgorithm  string        `env:"RATE_LIMIT_ALGORITHM" envDefault:"sliding-window"`
	RateLimitDistributed bool         `env:"RATE_LIMIT_DISTRIBUTED" envDefault:"true"`

	// CORS
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
}

// Load parses environment variables into Config.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[cfg.AppEnv] {
		return nil, fmt.Errorf("APP_ENV must be one of: development, staging, production (got %q)", cfg.AppEnv)
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	for _, v := range []struct{ name, val string }{
		{"DATABASE_URL", cfg.DatabaseURL},
		{"REDIS_URL", cfg.RedisURL},
		{"RABBITMQ_URL", cfg.RabbitURL},
	} {
		if err := validateURL(v.name, v.val); err != nil {
			return nil, err
		}
	}
	if cfg.DBMinConns > cfg.DBMaxConns {
		return nil, fmt.Errorf("DB_MIN_CONNS (%d) must not exceed DB_MAX_CONNS (%d)", cfg.DBMinConns, cfg.DBMaxConns)
	}
	if cfg.OTLPSampleRate < 0 || cfg.OTLPSampleRate > 1 {
		return nil, fmt.Errorf("OTEL_SAMPLING_RATIO must be between 0 and 1 (got %f)", cfg.OTLPSampleRate)
	}
	if cfg.RateLimitRPM <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_RPM must be greater than 0 (got %d)", cfg.RateLimitRPM)
	}
	return cfg, nil
}

// validateURL checks that a config URL has a valid scheme and host.
func validateURL(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s: invalid URL format %q", name, raw)
	}
	return nil
}

// IsDevelopment returns true when running in dev mode.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

// IsProduction returns true when running in production.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// String returns a human-readable config summary with sensitive fields masked.
// Safe to log or print without leaking credentials.
func (c Config) String() string {
	var b strings.Builder
	b.WriteString("Config{")
	fmt.Fprintf(&b, "AppEnv:%s ", c.AppEnv)
	fmt.Fprintf(&b, "AppName:%s ", c.AppName)
	fmt.Fprintf(&b, "Port:%d ", c.Port)
	fmt.Fprintf(&b, "DatabaseURL:%s ", maskURL(c.DatabaseURL))
	fmt.Fprintf(&b, "DBMaxConns:%d ", c.DBMaxConns)
	fmt.Fprintf(&b, "DBMinConns:%d ", c.DBMinConns)
	fmt.Fprintf(&b, "DBMaxConnLifetime:%s ", c.DBMaxConnLifetime)
	fmt.Fprintf(&b, "RequestTimeout:%s ", c.RequestTimeout)
	fmt.Fprintf(&b, "RedisURL:%s ", maskURL(c.RedisURL))
	fmt.Fprintf(&b, "RabbitURL:%s ", maskURL(c.RabbitURL))
	fmt.Fprintf(&b, "JWTSecret:%s ", mask(c.JWTSecret))
	fmt.Fprintf(&b, "JWTAccessTTL:%s ", c.JWTAccessTTL)
	fmt.Fprintf(&b, "JWTRefreshTTL:%s ", c.JWTRefreshTTL)
	fmt.Fprintf(&b, "LogLevel:%s ", c.LogLevel)
	fmt.Fprintf(&b, "OTLPEndpoint:%s ", c.OTLPEndpoint)
	fmt.Fprintf(&b, "SMTPHost:%s ", c.SMTPHost)
	fmt.Fprintf(&b, "SMTPPort:%d ", c.SMTPPort)
	fmt.Fprintf(&b, "SMTPFrom:%s ", c.SMTPFrom)
	fmt.Fprintf(&b, "SMTPUser:%s ", mask(c.SMTPUser))
	fmt.Fprintf(&b, "SMTPPassword:%s ", mask(c.SMTPPassword))
	fmt.Fprintf(&b, "SMTPFromAlias:%s ", c.SMTPFromAlias)
	fmt.Fprintf(&b, "ElasticsearchURL:%s ", c.ElasticsearchURL)
	fmt.Fprintf(&b, "ElasticsearchIndexPrefix:%s ", c.ElasticsearchIndexPrefix)
	fmt.Fprintf(&b, "BlacklistFailOpen:%v ", c.BlacklistFailOpen)
	fmt.Fprintf(&b, "RateLimitRPM:%d ", c.RateLimitRPM)
	fmt.Fprintf(&b, "RateLimitWindow:%s ", c.RateLimitWindow)
	fmt.Fprintf(&b, "RateLimitScope:%s ", c.RateLimitScope)
	fmt.Fprintf(&b, "RateLimitAlgorithm:%s ", c.RateLimitAlgorithm)
	fmt.Fprintf(&b, "RateLimitDistributed:%v ", c.RateLimitDistributed)
	fmt.Fprintf(&b, "CORSOrigins:%v", c.CORSOrigins)
	b.WriteByte('}')
	return b.String()
}

// mask replaces a non-empty secret string with "***".
func mask(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

// maskURL redacts the userinfo (credentials) from a URL string.
// If parsing fails the entire value is masked.
func maskURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "***"
	}
	if u.User != nil {
		u.User = url.UserPassword("***", "***")
	}
	return u.String()
}
