package model

import (
	"time"

	"github.com/google/uuid"
)

// ContextField defines a project-wide attribute that can be reused by every
// flag's targeting rules. It deliberately has no environment relationship:
// environments configure flag behavior, while context vocabulary belongs to
// the project as a whole.
type ContextField struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID   uuid.UUID `gorm:"type:uuid;index:idx_context_field_project_key,unique;not null"`
	Key         string    `gorm:"index:idx_context_field_project_key,unique;not null"`
	Description string
	Values      []ContextFieldValue `gorm:"foreignKey:ContextFieldID"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ContextFieldValue is an allowed option for a context field. Values are
// intentionally strings so the catalogue remains SDK-agnostic.
type ContextFieldValue struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContextFieldID uuid.UUID `gorm:"type:uuid;index:idx_context_value_field_value,unique;not null"`
	Value          string    `gorm:"index:idx_context_value_field_value,unique;not null"`
	Description    string
	SortOrder      int `gorm:"not null;default:0"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
