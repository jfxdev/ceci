package model

import (
	"time"

	"github.com/google/uuid"
)

// InstanceSettings is the singleton configuration shared by every Control
// Plane replica. ID is always 1.
type InstanceSettings struct {
	ID                   uint   `gorm:"primaryKey"`
	MaintenanceEnabled   bool   `gorm:"not null;default:false"`
	MaintenanceMessage   string `gorm:"not null;default:''"`
	MaintenanceStartedAt *time.Time
	MaintenanceStartedBy uuid.UUID `gorm:"type:uuid"`
	OIDCEnabled          bool      `gorm:"not null;default:false"`
	OIDCIssuerURL        string    `gorm:"not null;default:''"`
	OIDCClientID         string    `gorm:"not null;default:''"`
	OIDCSecretCiphertext string    `gorm:"not null;default:''"`
	OIDCRedirectURL      string    `gorm:"not null;default:''"`
	OIDCJITEnabled       bool      `gorm:"not null;default:false"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
