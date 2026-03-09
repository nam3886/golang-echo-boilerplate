package config

import (
	"fmt"
	"net/url"
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
	AppName string `env:"APP_NAME" envDefault:"gnha-services"`
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
	JWTRefreshTTL time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`

	// Logging
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// Observability
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"http://localhost:4317"`

	// SMTP
	SMTPHost      string `env:"SMTP_HOST" envDefault:"localhost"`
	SMTPPort      int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPFrom      string `env:"SMTP_FROM" envDefault:"noreply@app.local"`
	SMTPUser      string `env:"SMTP_USER"`
	SMTPPassword  string `env:"SMTP_PASSWORD"`
	SMTPFromAlias string `env:"SMTP_FROM_ALIAS"`

	// Elasticsearch (optional — empty URL disables search)
	ElasticsearchURL         string `env:"ELASTICSEARCH_URL"`
	ElasticsearchIndexPrefix string `env:"ELASTICSEARCH_INDEX_PREFIX" envDefault:"gnha"`

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
	return fmt.Sprintf(
		"Config{AppEnv:%s AppName:%s Port:%d DatabaseURL:%s DBMaxConns:%d DBMinConns:%d "+
			"RedisURL:%s RabbitURL:%s JWTSecret:%s JWTAccessTTL:%s JWTRefreshTTL:%s "+
			"LogLevel:%s OTLPEndpoint:%s SMTPHost:%s SMTPPort:%d SMTPFrom:%s SMTPUser:%s SMTPPassword:%s SMTPFromAlias:%s "+
			"ElasticsearchURL:%s ElasticsearchIndexPrefix:%s CORSOrigins:%v}",
		c.AppEnv,
		c.AppName,
		c.Port,
		maskURL(c.DatabaseURL),
		c.DBMaxConns,
		c.DBMinConns,
		maskURL(c.RedisURL),
		maskURL(c.RabbitURL),
		mask(c.JWTSecret),
		c.JWTAccessTTL,
		c.JWTRefreshTTL,
		c.LogLevel,
		c.OTLPEndpoint,
		c.SMTPHost,
		c.SMTPPort,
		c.SMTPFrom,
		c.SMTPUser,
		mask(c.SMTPPassword),
		c.SMTPFromAlias,
		c.ElasticsearchURL,
		c.ElasticsearchIndexPrefix,
		c.CORSOrigins,
	)
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
