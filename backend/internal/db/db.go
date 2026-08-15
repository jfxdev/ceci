package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&model.User{},
		&model.AuthIdentity{},
		&model.RefreshToken{},
		&model.AccessGroup{},
		&model.AccessGroupMember{},
		&model.OIDCAccessGroupMapping{},
		&model.ProjectAccessGroup{},
		&model.InstanceSettings{},
		&model.Project{},
		&model.ProjectMember{},
		&model.Environment{},
		&model.EnvironmentTemplate{},
		&model.ProjectAPIKey{},
		&model.FeatureFlag{},
		&model.FlagVariant{},
		&model.FlagEnvironmentConfig{},
		&model.FlagRule{},
		&model.Parameter{},
		&model.ParameterVersion{},
	)
}
