package config

import (
	"strings"
	"testing"
	"time"
)

func lookupFrom(values map[string]string) lookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestLoadDefaultsAndValues(t *testing.T) {
	get := lookupFrom(map[string]string{
		"DATABASE_URL":      "postgres://pulsewatch:secret@localhost:5432/pulsewatch",
		"REDIS_ADDR":        "localhost:6379",
		"JWT_ACCESS_SECRET": "this-is-a-local-development-secret-at-least-32-bytes",
	})

	cfg, err := load(get)
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.WorkerHTTPAddr != "127.0.0.1:8081" {
		t.Fatalf("unexpected addresses: %#v", cfg)
	}
	if cfg.HealthTimeout != 2*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected timeouts: %#v", cfg)
	}
	if cfg.SchedulerInterval != 5*time.Second || cfg.SchedulerBatchSize != 20 || cfg.DispatchInterval != 10*time.Second || cfg.DispatchBatchSize != 100 {
		t.Fatalf("unexpected worker defaults: %#v", cfg)
	}
	if cfg.SMTPAddr != "127.0.0.1:1025" || cfg.SMTPFrom != "alerts@pulsewatch.local" {
		t.Fatalf("unexpected SMTP defaults: %#v", cfg)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Fatalf("unexpected origins: %#v", cfg.CORSAllowedOrigins)
	}
}

func TestValidateAPIRequiresLongJWTSecretWithoutLeakingIt(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL": "postgres://pulsewatch:secret@localhost:5432/pulsewatch",
		"REDIS_ADDR":   "localhost:6379",
	}
	cfg, err := load(lookupFrom(base))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if err := cfg.ValidateAPI(); err == nil || !strings.Contains(err.Error(), "JWT_ACCESS_SECRET") {
		t.Fatalf("expected JWT_ACCESS_SECRET error, got %v", err)
	}
	base["JWT_ACCESS_SECRET"] = "this-is-a-local-development-secret-at-least-32-bytes"
	cfg, err = load(lookupFrom(base))
	if err != nil || cfg.ValidateAPI() != nil {
		t.Fatalf("valid API configuration rejected: load=%v validate=%v", err, cfg.ValidateAPI())
	}
}

func TestLoadRejectsMissingRequiredConfigurationWithoutLeakingValues(t *testing.T) {
	get := lookupFrom(map[string]string{"REDIS_ADDR": "localhost:6379"})

	_, err := load(get)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked a secret: %v", err)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL": "postgres://pulsewatch:p@localhost:5432/pulsewatch",
		"REDIS_ADDR":   "localhost:6379",
	}
	for key, value := range map[string]string{
		"DATABASE_URL":         "not-a-database-url",
		"REDIS_ADDR":           "not-an-address",
		"HTTP_ADDR":            "not-an-address",
		"WORKER_HTTP_ADDR":     "127.0.0.1:0",
		"CORS_ALLOWED_ORIGINS": "*",
		"HEALTH_TIMEOUT":       "not-a-duration",
		"SCHEDULER_INTERVAL":   "0s",
		"SCHEDULER_BATCH_SIZE": "0",
		"DISPATCH_INTERVAL":    "invalid",
		"DISPATCH_BATCH_SIZE":  "-1",
		"SMTP_ADDR":            "invalid",
		"SMTP_FROM":            "bad-address",
	} {
		values := make(map[string]string, len(base)+1)
		for baseKey, baseValue := range base {
			values[baseKey] = baseValue
		}
		values[key] = value
		if _, err := load(lookupFrom(values)); err == nil {
			t.Errorf("load() with %s=%q succeeded", key, value)
		}
	}
}
