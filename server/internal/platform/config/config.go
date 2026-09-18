package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains configuration shared by the API and Worker processes.
// Values are loaded once at the process boundary and passed down explicitly.
type Config struct {
	Environment        string
	HTTPAddr           string
	WorkerHTTPAddr     string
	DatabaseURL        string
	RedisAddr          string
	RedisPassword      string
	CORSAllowedOrigins []string
	HealthTimeout      time.Duration
	ShutdownTimeout    time.Duration
	JWTAccessSecret    string
	JWTIssuer          string
}

// Load reads environment variables and validates the values that Day02 uses.
// Future settings such as JWT and SMTP are intentionally not required yet.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

type lookup func(string) (string, bool)

func load(get lookup) (Config, error) {
	cfg := Config{
		Environment:     envOr(get, "APP_ENV", "development"),
		HTTPAddr:        envOr(get, "HTTP_ADDR", ":8080"),
		WorkerHTTPAddr:  envOr(get, "WORKER_HTTP_ADDR", "127.0.0.1:8081"),
		HealthTimeout:   durationOr(get, "HEALTH_TIMEOUT", 2*time.Second),
		ShutdownTimeout: durationOr(get, "SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	var ok bool
	if cfg.DatabaseURL, ok = get("DATABASE_URL"); !ok || strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, missing("DATABASE_URL")
	}
	if cfg.RedisAddr, ok = get("REDIS_ADDR"); !ok || strings.TrimSpace(cfg.RedisAddr) == "" {
		return Config{}, missing("REDIS_ADDR")
	}
	if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
		return Config{}, err
	}
	if err := validateAddress(cfg.RedisAddr, "REDIS_ADDR"); err != nil {
		return Config{}, err
	}
	cfg.RedisPassword, _ = get("REDIS_PASSWORD")
	cfg.JWTAccessSecret, _ = get("JWT_ACCESS_SECRET")
	cfg.JWTIssuer = envOr(get, "JWT_ISSUER", "pulsewatch")

	origins := envOr(get, "CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	cfg.CORSAllowedOrigins = splitOrigins(origins)

	if err := validateAddress(cfg.HTTPAddr, "HTTP_ADDR"); err != nil {
		return Config{}, err
	}
	if err := validateAddress(cfg.WorkerHTTPAddr, "WORKER_HTTP_ADDR"); err != nil {
		return Config{}, err
	}
	if err := validateOrigins(cfg.CORSAllowedOrigins); err != nil {
		return Config{}, err
	}
	if cfg.HealthTimeout <= 0 {
		return Config{}, invalid("HEALTH_TIMEOUT")
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, invalid("SHUTDOWN_TIMEOUT")
	}

	return cfg, nil
}

// ValidateAPI validates settings that only the public API process needs.
func (c Config) ValidateAPI() error {
	if len(c.JWTAccessSecret) < 32 {
		return invalid("JWT_ACCESS_SECRET")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" {
		return invalid("JWT_ISSUER")
	}
	return nil
}

func (c Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func envOr(get lookup, key, fallback string) string {
	if value, ok := get(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func durationOr(get lookup, key string, fallback time.Duration) time.Duration {
	value, ok := get(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func splitOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func validateOrigins(origins []string) error {
	if len(origins) == 0 {
		return invalid("CORS_ALLOWED_ORIGINS")
	}
	for _, origin := range origins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return invalid("CORS_ALLOWED_ORIGINS")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return invalid("CORS_ALLOWED_ORIGINS")
		}
	}
	return nil
}

func validateAddress(address, key string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return invalid(key)
	}
	if host == "" {
		// :8080 is the intended development default and is valid.
		host = "0.0.0.0"
	}
	if strings.TrimSpace(host) == "" {
		return invalid(key)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return invalid(key)
	}
	return nil
}

func validateDatabaseURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return invalid("DATABASE_URL")
	}
	return nil
}

func missing(key string) error { return fmt.Errorf("missing required configuration: %s", key) }

func invalid(key string) error { return fmt.Errorf("invalid configuration: %s", key) }
