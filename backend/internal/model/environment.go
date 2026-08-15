package model

import (
	"time"

	"github.com/google/uuid"
)

// Environment scopes flag state, targeting rules, parameter values, and API
// keys within a project (e.g. "production", "staging"). Flag/parameter
// definitions (key, type, name) stay project-level and shared across
// environments — only their values/config vary per environment.
type Environment struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;index:idx_env_project_key,unique;not null"`
	Key       string    `gorm:"index:idx_env_project_key,unique;not null"`
	Name      string    `gorm:"not null"`
	SortOrder int       `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EnvironmentTemplate is an organization-wide preset offered when a new
// project is created. Required templates are always provisioned, regardless
// of the selections supplied by the client.
type EnvironmentTemplate struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Key        string    `gorm:"uniqueIndex;not null"`
	Name       string    `gorm:"not null"`
	IsRequired bool      `gorm:"not null;default:false"`
	SortOrder  int       `gorm:"not null;default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
