package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// generateID is shared by every model's BeforeCreate hook: primary keys are
// generated application-side (not via a DB-specific default like postgres'
// gen_random_uuid()) so the same models work against any SQL database.
func generateID(id *uuid.UUID) {
	if *id == uuid.Nil {
		*id = uuid.New()
	}
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	generateID(&u.ID)
	return nil
}

func (i *AuthIdentity) BeforeCreate(tx *gorm.DB) error {
	generateID(&i.ID)
	return nil
}

func (rt *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	generateID(&rt.ID)
	return nil
}

func (g *AccessGroup) BeforeCreate(tx *gorm.DB) error {
	generateID(&g.ID)
	return nil
}

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	generateID(&p.ID)
	return nil
}

func (e *Environment) BeforeCreate(tx *gorm.DB) error {
	generateID(&e.ID)
	return nil
}

func (t *EnvironmentTemplate) BeforeCreate(tx *gorm.DB) error {
	generateID(&t.ID)
	return nil
}

func (c *FlagEnvironmentConfig) BeforeCreate(tx *gorm.DB) error {
	generateID(&c.ID)
	return nil
}

func (k *ProjectAPIKey) BeforeCreate(tx *gorm.DB) error {
	generateID(&k.ID)
	return nil
}

func (f *FeatureFlag) BeforeCreate(tx *gorm.DB) error {
	generateID(&f.ID)
	return nil
}

func (c *FlagCollaborator) BeforeCreate(tx *gorm.DB) error {
	generateID(&c.ID)
	return nil
}

func (s *FlagStrategy) BeforeCreate(tx *gorm.DB) error {
	generateID(&s.ID)
	return nil
}

func (v *FlagStrategyVariant) BeforeCreate(tx *gorm.DB) error {
	generateID(&v.ID)
	return nil
}

func (p *Parameter) BeforeCreate(tx *gorm.DB) error {
	generateID(&p.ID)
	return nil
}

func (v *ParameterVersion) BeforeCreate(tx *gorm.DB) error {
	generateID(&v.ID)
	return nil
}
