package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"ceci/backend/internal/model"
)

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&model.User{},
		&model.RefreshToken{},
		&model.Project{},
		&model.ProjectMember{},
		&model.ProjectAPIKey{},
		&model.FeatureFlag{},
		&model.FlagVariant{},
		&model.FlagRule{},
		&model.Parameter{},
		&model.ParameterVersion{},
	)
}
