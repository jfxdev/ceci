package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string
	Name         string
	IsAdmin      bool `gorm:"not null;default:false"`
	// IsBootstrapAdmin identifies the break-glass account created by the seed
	// command. It must never authenticate through a federated provider.
	IsBootstrapAdmin bool `gorm:"not null;default:false"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// AuthIdentity links a stable identity-provider subject to one LeaFlag user.
// Email is intentionally not part of this key because IdP email addresses can
// change; provider + subject is the durable identifier.
type AuthIdentity struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	Provider  string    `gorm:"uniqueIndex:idx_auth_identity_provider_subject;not null"`
	Subject   string    `gorm:"uniqueIndex:idx_auth_identity_provider_subject;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	CreatedAt time.Time
}
