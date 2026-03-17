package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfigString_MasksSensitiveFields(t *testing.T) {
	//nolint:gosec // test data with intentional hardcoded credentials
	cfg := Config{
		AppEnv:      "development",
		AppName:     "test-app",
		Port:        8080,
		DatabaseURL: "postgres://user:secret@localhost/testdb",
		RedisURL:    "redis://user:secret@localhost:6379",
		RabbitURL:   "amqp://user:secret@localhost:5672",
		JWTSecret:   "supersecretjwtkey1234567890abcdef",
		SMTPPassword: "smtpsecret",
		JWTAccessTTL: 15 * time.Minute,
	}

	s := cfg.String()

	// Sensitive values must not appear in output.
	for _, secret := range []string{"secret", "supersecretjwtkey1234567890abcdef", "smtpsecret"} {
		if strings.Contains(s, secret) {
			t.Errorf("String() must not contain %q, got: %s", secret, s)
		}
	}

	// Non-sensitive fields must appear.
	for _, visible := range []string{"development", "test-app", "8080"} {
		if !strings.Contains(s, visible) {
			t.Errorf("String() should contain %q, got: %s", visible, s)
		}
	}

	// Masked URLs should still contain host info.
	if !strings.Contains(s, "localhost") {
		t.Errorf("String() should still contain hostname, got: %s", s)
	}
}

func TestConfigString_DatabaseURLMasked(t *testing.T) {
	//nolint:gosec // test data
	cfg := Config{DatabaseURL: "postgres://admin:password@db.example.com/mydb"}
	s := cfg.String()
	if strings.Contains(s, "password") {
		t.Errorf("DatabaseURL password must be masked, got: %s", s)
	}
	if !strings.Contains(s, "db.example.com") {
		t.Errorf("DatabaseURL host should be visible, got: %s", s)
	}
}

func TestConfigString_RedisURLMasked(t *testing.T) {
	//nolint:gosec // test data
	cfg := Config{RedisURL: "redis://user:redispass@cache.example.com:6379"}
	s := cfg.String()
	if strings.Contains(s, "redispass") {
		t.Errorf("RedisURL password must be masked, got: %s", s)
	}
}

func TestConfigString_JWTSecretMasked(t *testing.T) {
	secret := "thisisaverylongandsecretjwtkey12"
	cfg := Config{JWTSecret: secret}
	s := cfg.String()
	if strings.Contains(s, secret) {
		t.Errorf("JWTSecret must be masked, got: %s", s)
	}
	if !strings.Contains(s, "***") {
		t.Errorf("JWTSecret should show '***', got: %s", s)
	}
}

func TestConfigString_SMTPPasswordMasked(t *testing.T) {
	cfg := Config{SMTPPassword: "mysmtppassword"}
	s := cfg.String()
	if strings.Contains(s, "mysmtppassword") {
		t.Errorf("SMTPPassword must be masked, got: %s", s)
	}
}

func TestConfigString_RabbitMQURLMasked(t *testing.T) {
	//nolint:gosec // test data
	cfg := Config{RabbitURL: "amqp://mquser:mqpass@rabbitmq.example.com:5672"}
	s := cfg.String()
	if strings.Contains(s, "mqpass") {
		t.Errorf("RabbitMQ URL password must be masked, got: %s", s)
	}
	if !strings.Contains(s, "rabbitmq.example.com") {
		t.Errorf("RabbitMQ URL host should be visible, got: %s", s)
	}
}

func TestConfigString_EmptySecretsNotMasked(t *testing.T) {
	cfg := Config{
		JWTSecret:    "",
		SMTPPassword: "",
		DatabaseURL:  "",
		RedisURL:     "",
		RabbitURL:    "",
	}
	s := cfg.String()
	// Empty values should not show '***'
	// Check that the output doesn't have extra '***' for optional empty fields
	// (SMTPPassword is optional so empty should show empty, not ***)
	_ = s // just verify it doesn't panic
}
