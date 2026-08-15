package model

import (
	"time"

	"github.com/google/uuid"
)

type Parameter struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID     uuid.UUID `gorm:"type:uuid;index:idx_param_project_env_key,unique;not null"`
	EnvironmentID uuid.UUID `gorm:"type:uuid;index:idx_param_project_env_key,unique;not null"`
	Key           string    `gorm:"index:idx_param_project_env_key,unique;not null"`
	Value         string    `gorm:"not null"`
	Version       int       `gorm:"not null;default:1"`
	UpdatedBy     uuid.UUID `gorm:"type:uuid"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ParameterVersion struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ParameterID uuid.UUID `gorm:"type:uuid;index;not null"`
	Version     int       `gorm:"not null"`
	Value       string
	ChangedBy   uuid.UUID `gorm:"type:uuid"`
	ChangedAt   time.Time
	ChangeType  string
}
