package model

import (
	"time"

	"leaflag/backend/internal/constants"
	"github.com/google/uuid"
)

type Project struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"not null"`
	Slug      string    `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectMember struct {
	ProjectID uuid.UUID             `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID             `gorm:"type:uuid;primaryKey"`
	Role      constants.ProjectRole `gorm:"not null"`
	CreatedAt time.Time
}

type ProjectAPIKey struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID     uuid.UUID `gorm:"type:uuid;index;not null"`
	EnvironmentID uuid.UUID `gorm:"type:uuid;index;not null"`
	Label         string
	KeyHash       string `gorm:"uniqueIndex;not null"`
	Prefix        string
	CreatedAt     time.Time
	RevokedAt     *time.Time
}
