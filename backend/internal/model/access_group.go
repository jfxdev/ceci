package model

import (
	"time"

	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
)

// AccessGroup is a reusable collection of users. In a later SSO integration,
// an external IdP group can be mapped to this same local abstraction.
type AccessGroup struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"uniqueIndex;not null"`
	Description string    `gorm:"not null;default:''"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AccessGroupMember records the users that currently belong to a local group.
type AccessGroupMember struct {
	AccessGroupID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	// Source is local or oidc. A local membership is never removed by a
	// subsequent IdP synchronization.
	Source    string `gorm:"not null;default:'local'"`
	CreatedAt time.Time
}

// OIDCAccessGroupMapping maps a group claim value (Entra object ID or an
// Authentik group value) to a reusable LeaFlag access group.
type OIDCAccessGroupMapping struct {
	AccessGroupID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExternalGroup string    `gorm:"primaryKey"`
	CreatedAt     time.Time
}

func (OIDCAccessGroupMapping) TableName() string { return "oidc_access_group_mappings" }

// ProjectAccessGroup grants every member of an access group a project role.
// Owners are deliberately excluded by the service layer: project ownership is
// always an explicit, individual assignment.
type ProjectAccessGroup struct {
	ProjectID     uuid.UUID             `gorm:"type:uuid;primaryKey"`
	AccessGroupID uuid.UUID             `gorm:"type:uuid;primaryKey"`
	Role          constants.ProjectRole `gorm:"not null"`
	CreatedAt     time.Time
}
