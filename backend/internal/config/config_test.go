package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("JWT_SECRET")
	t.Setenv("LEAFLAG_MODE", "")
	t.Setenv("CONTROL_PLANE_URL", "")
	t.Setenv("RUNTIME_SYNC_TOKEN", "")
	t.Setenv("RUNTIME_SYNC_TOKENS", "")
	t.Setenv("EDGE_SYNC_INTERVAL", "")
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_REDIRECT_URL", "")
	t.Setenv("OIDC_JIT_ENABLED", "")

	cfg := Load()
	assert.Equal(t, "8110", cfg.Port)
	assert.Contains(t, cfg.DatabaseURL, "postgres://")
	assert.NotEmpty(t, cfg.JWTSecret)
	assert.Equal(t, "all-in-one", cfg.Mode)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("DATABASE_URL", "postgres://custom")
	t.Setenv("JWT_SECRET", "custom-secret")
	t.Setenv("LEAFLAG_MODE", "data-plane")
	t.Setenv("CONTROL_PLANE_URL", "https://control.example.com/")
	t.Setenv("RUNTIME_SYNC_TOKEN", "sync")
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_REDIRECT_URL", "")

	cfg := Load()
	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, "postgres://custom", cfg.DatabaseURL)
	assert.Equal(t, "custom-secret", cfg.JWTSecret)
	assert.Equal(t, "data-plane", cfg.Mode)
	assert.Equal(t, "https://control.example.com", cfg.ControlPlaneURL)
}

func TestConfigValidate_Modes(t *testing.T) {
	assert.NoError(t, Config{Mode: "all-in-one", EdgeSyncInterval: 5}.Validate())
	assert.NoError(t, Config{Mode: "control-plane", RuntimeSyncTokens: []string{"current"}, EdgeSyncInterval: 5}.Validate())
	assert.NoError(t, Config{Mode: "data-plane", ControlPlaneURL: "https://control.example.com", RuntimeSyncToken: "current", EdgeSyncInterval: 5}.Validate())
	assert.Error(t, Config{Mode: "data-plane", ControlPlaneURL: "http://control.example.com", RuntimeSyncToken: "current", EdgeSyncInterval: 5}.Validate())
}
