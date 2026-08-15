package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type FeatureFlag struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID           uuid.UUID `gorm:"type:uuid;index:idx_flag_project_key,unique;not null"`
	Key                 string    `gorm:"index:idx_flag_project_key,unique;not null"`
	Name                string
	Description         string
	FlagType            string `gorm:"not null"`
	PrerequisiteFlagKey string
	PrerequisiteVariant string
	Variants            []FlagVariant           `gorm:"foreignKey:FlagID"`
	Configs             []FlagEnvironmentConfig `gorm:"foreignKey:FlagID"`
	Rules               []FlagRule              `gorm:"foreignKey:FlagID"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type FlagVariant struct {
	ID     uuid.UUID      `gorm:"type:uuid;primaryKey"`
	FlagID uuid.UUID      `gorm:"type:uuid;index;not null"`
	Key    string         `gorm:"not null"`
	Value  datatypes.JSON `gorm:"not null"`
}

// FlagEnvironmentConfig holds the parts of a flag's behavior that vary per
// environment: whether it's on, and which variant it falls back to. At most
// one row exists per (FlagID, EnvironmentID) pair; an environment with no
// row for a flag is treated as disabled (see flag_evaluate.go).
type FlagEnvironmentConfig struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	FlagID         uuid.UUID `gorm:"type:uuid;index:idx_flagcfg_flag_env,unique;not null"`
	EnvironmentID  uuid.UUID `gorm:"type:uuid;index:idx_flagcfg_flag_env,unique;not null"`
	Enabled        bool      `gorm:"not null;default:true"`
	DefaultVariant string    `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type FlagRule struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	FlagID        uuid.UUID `gorm:"type:uuid;index;not null"`
	EnvironmentID uuid.UUID `gorm:"type:uuid;index;not null"`
	Priority      int       `gorm:"not null"`
	Description   string
	ConditionJSON datatypes.JSON `gorm:"not null"`
	VariantKey    string         `gorm:"not null"`
	RolloutJSON   datatypes.JSON
}
