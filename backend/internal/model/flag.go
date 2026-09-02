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
	Tags                datatypes.JSONSlice[string] `gorm:"not null;default:'[]'"`
	FlagType            string                      `gorm:"not null"`
	CreatedByID         *uuid.UUID                  `gorm:"type:uuid;index"`
	CreatedBy           *User                       `gorm:"foreignKey:CreatedByID"`
	Collaborators       []FlagCollaborator          `gorm:"foreignKey:FlagID"`
	PrerequisiteFlagKey string
	PrerequisiteVariant string
	Configs             []FlagEnvironmentConfig `gorm:"foreignKey:FlagID"`
	Strategies          []FlagStrategy          `gorm:"foreignKey:FlagID"`
	ArchivedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// FlagCollaborator records users who created or edited a feature flag.
type FlagCollaborator struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	FlagID    uuid.UUID `gorm:"type:uuid;index:idx_flag_collaborator,unique;not null"`
	UserID    uuid.UUID `gorm:"type:uuid;index:idx_flag_collaborator,unique;not null"`
	User      User      `gorm:"foreignKey:UserID"`
	CreatedAt time.Time
}

// FlagEnvironmentConfig holds the one part of a flag's behavior that's purely
// per-environment: the kill switch. At most one row exists per (FlagID,
// EnvironmentID) pair; an environment with no row for a flag is treated as
// disabled — new environments start every flag off until explicitly
// configured (see toEnvFlag in flag_evaluate.go).
type FlagEnvironmentConfig struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	FlagID        uuid.UUID `gorm:"type:uuid;index:idx_flagcfg_flag_env,unique;not null"`
	EnvironmentID uuid.UUID `gorm:"type:uuid;index:idx_flagcfg_flag_env,unique;not null"`
	Enabled       bool      `gorm:"not null;default:true"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// FlagStrategy is one targeting strategy within a flag, scoped to a single
// environment. A flag+environment holds an ordered list of strategies; at
// eval time (see evaluateFlag in flag_evaluate.go) the first strategy whose
// ConditionJSON matches the context wins, applying its own rollout and its
// own variant catalog. Strategies are evaluated by Priority; the first
// condition that matches wins. A flag can have no strategies.
type FlagStrategy struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	FlagID        uuid.UUID `gorm:"type:uuid;index:idx_strategy_flag_env_priority,unique;not null"`
	EnvironmentID uuid.UUID `gorm:"type:uuid;index:idx_strategy_flag_env_priority,unique;not null"`
	Priority      int       `gorm:"index:idx_strategy_flag_env_priority,unique;not null"`
	Name          string
	Description   string
	// IsDefault is retained for compatibility with existing persisted rows.
	// It does not affect strategy ordering or normal evaluation.
	IsDefault     bool `gorm:"not null;default:false"`
	ConditionJSON datatypes.JSON
	// DefaultVariant is served when this strategy matches and either it has
	// no rollout or the rollout doesn't produce a bucketed variant.
	DefaultVariant string `gorm:"not null"`
	// RolloutJSON, when set, splits traffic matched by this strategy across
	// variants — this is the per-strategy replacement for the old flag-wide
	// global rollout percentage.
	RolloutJSON datatypes.JSON
	Variants    []FlagStrategyVariant `gorm:"foreignKey:StrategyID"`
}

type FlagStrategyVariant struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey"`
	StrategyID uuid.UUID      `gorm:"type:uuid;index;not null"`
	Key        string         `gorm:"not null"`
	Value      datatypes.JSON `gorm:"not null"`
}
