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
	if err := gdb.AutoMigrate(
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
		&model.ContextField{},
		&model.ContextFieldValue{},
		&model.ProjectAPIKey{},
		&model.FeatureFlag{},
		&model.FlagCollaborator{},
		&model.FlagEnvironmentConfig{},
		&model.FlagStrategy{},
		&model.FlagStrategyVariant{},
		&model.Parameter{},
		&model.ParameterVersion{},
	); err != nil {
		return err
	}
	return dropObsoleteFlagSchema(gdb)
}

// dropObsoleteFlagSchema removes leftovers from the pre-strategy flag model
// (flat variant catalog + rules + flag-wide default/rollout on
// FlagEnvironmentConfig). AutoMigrate only ever adds columns/tables, so a
// database that predates the strategy model keeps e.g. a NOT NULL
// default_variant column the current model no longer populates — silently
// rejecting every insert into flag_environment_configs until it's dropped.
func dropObsoleteFlagSchema(gdb *gorm.DB) error {
	m := gdb.Migrator()
	for _, col := range []string{"default_variant", "rollout_percentage", "rollout_variant"} {
		if m.HasColumn(&model.FlagEnvironmentConfig{}, col) {
			if err := m.DropColumn(&model.FlagEnvironmentConfig{}, col); err != nil {
				return err
			}
		}
	}
	for _, table := range []string{"flag_variants", "flag_rules"} {
		if m.HasTable(table) {
			if err := m.DropTable(table); err != nil {
				return err
			}
		}
	}
	return nil
}
