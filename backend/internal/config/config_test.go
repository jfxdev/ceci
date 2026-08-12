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

	cfg := Load()
	assert.Equal(t, "8110", cfg.Port)
	assert.Contains(t, cfg.DatabaseURL, "postgres://")
	assert.NotEmpty(t, cfg.JWTSecret)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("DATABASE_URL", "postgres://custom")
	t.Setenv("JWT_SECRET", "custom-secret")

	cfg := Load()
	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, "postgres://custom", cfg.DatabaseURL)
	assert.Equal(t, "custom-secret", cfg.JWTSecret)
}
