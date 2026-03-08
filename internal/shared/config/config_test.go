package config

import (
	"strings"
	"testing"
)

func TestIsDevelopment(t *testing.T) {
	cfg := &Config{AppEnv: "development"}
	if !cfg.IsDevelopment() {
		t.Error("IsDevelopment should return true for development")
	}

	cfg.AppEnv = "production"
	if cfg.IsDevelopment() {
		t.Error("IsDevelopment should return false for production")
	}
}

func TestIsProduction(t *testing.T) {
	cfg := &Config{AppEnv: "production"}
	if !cfg.IsProduction() {
		t.Error("IsProduction should return true for production")
	}

	cfg.AppEnv = "development"
	if cfg.IsProduction() {
		t.Error("IsProduction should return false for development")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("RABBITMQ_URL", "amqp://localhost:5672")
	t.Setenv("JWT_SECRET", "thisisaverylongjwtsecretfor32chars")
	t.Setenv("APP_ENV", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
}

func TestLoad_RejectsShortJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("RABBITMQ_URL", "amqp://localhost:5672")
	t.Setenv("JWT_SECRET", "tooshort")
	t.Setenv("APP_ENV", "development")

	cfg, err := Load()
	if err == nil {
		t.Fatal("expected error for short JWT_SECRET, got nil")
	}
	if cfg != nil {
		t.Error("expected nil config when JWT_SECRET is too short")
	}
}

func TestLoad_RejectsInvalidAppEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("RABBITMQ_URL", "amqp://localhost:5672")
	t.Setenv("JWT_SECRET", "thisisaverylongjwtsecretfor32chars")
	t.Setenv("APP_ENV", "invalid-env")

	cfg, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid APP_ENV, got nil")
	}
	if cfg != nil {
		t.Error("expected nil config when APP_ENV is invalid")
	}
}

func TestMask_NonEmpty(t *testing.T) {
	result := mask("secret123")
	if result != "***" {
		t.Errorf("expected '***' for non-empty string, got %s", result)
	}
}

func TestMask_Empty(t *testing.T) {
	result := mask("")
	if result != "" {
		t.Errorf("expected '' for empty string, got %s", result)
	}
}

func TestMaskURL_RedactsCredentials(t *testing.T) {
	result := maskURL("postgres://user:pass@localhost/db")
	if result == "" {
		t.Fatal("expected non-empty masked URL")
	}
	if strings.Contains(result, "pass") {
		t.Errorf("password should be redacted, got %s", result)
	}
}

func TestMaskURL_SimplePathParseable(t *testing.T) {
	// url.Parse is permissive and treats most strings as paths, not errors
	result := maskURL("some-path-without-scheme")
	if result == "" {
		t.Error("expected non-empty masked URL for path-like string")
	}
}

func TestMaskURL_Empty(t *testing.T) {
	result := maskURL("")
	if result != "" {
		t.Errorf("expected '' for empty string, got %s", result)
	}
}

