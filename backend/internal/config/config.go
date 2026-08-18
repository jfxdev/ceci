package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port              string
	DatabaseURL       string
	JWTSecret         string
	Mode              string
	ControlPlaneURL   string
	RuntimeSyncToken  string
	RuntimeSyncTokens []string
	EdgeSyncInterval  time.Duration
	EncryptionKey     string
}

func Load() Config {
	return Config{
		Port:              getEnv("PORT", "8110"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://leaflag:leaflag@localhost:5433/leaflag?sslmode=disable"),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-me"),
		Mode:              getEnv("LEAFLAG_MODE", "all-in-one"),
		ControlPlaneURL:   strings.TrimRight(getEnv("CONTROL_PLANE_URL", ""), "/"),
		RuntimeSyncToken:  getEnv("RUNTIME_SYNC_TOKEN", ""),
		RuntimeSyncTokens: splitCSV(getEnv("RUNTIME_SYNC_TOKENS", "")),
		EdgeSyncInterval:  durationEnv("EDGE_SYNC_INTERVAL", 5*time.Second),
		EncryptionKey:     getEnv("ENCRYPTION_KEY", getEnv("OIDC_CONFIG_ENCRYPTION_KEY", getEnv("JWT_SECRET", "dev-secret-change-me"))),
	}
}

// Validate ensures the selected process mode has the configuration required
// to keep the runtime plane isolated from the control-plane database.
func (c Config) Validate() error {
	switch c.Mode {
	case "control-plane":
		if len(c.RuntimeSyncTokens) == 0 {
			return fmt.Errorf("RUNTIME_SYNC_TOKENS is required in control-plane mode")
		}
	case "data-plane":
		if c.ControlPlaneURL == "" {
			return fmt.Errorf("CONTROL_PLANE_URL is required in data-plane mode")
		}
		parsed, err := url.Parse(c.ControlPlaneURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("CONTROL_PLANE_URL must be an https URL in data-plane mode")
		}
		if c.RuntimeSyncToken == "" {
			return fmt.Errorf("RUNTIME_SYNC_TOKEN is required in data-plane mode")
		}
	case "all-in-one":
	default:
		return fmt.Errorf("invalid LEAFLAG_MODE %q (expected control-plane, data-plane, or all-in-one)", c.Mode)
	}
	if c.EdgeSyncInterval <= 0 {
		return fmt.Errorf("EDGE_SYNC_INTERVAL must be positive")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return d
}

func boolEnv(key string, fallback bool) bool {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
