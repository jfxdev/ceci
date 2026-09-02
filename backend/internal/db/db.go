package db

import (
	"strconv"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func AutoMigrate(gdb *gorm.DB) error {
	// The unique strategy priority index is introduced by the model migration.
	// Existing installations may have duplicate legacy rows, so resolve those
	// before AutoMigrate attempts to create the index.
	if err := dropObsoleteFlagSchema(gdb); err != nil {
		return err
	}
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
	if m.HasTable(&model.FlagStrategy{}) {
		if err := deduplicateFlagStrategies(gdb); err != nil {
			return err
		}
	}
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

// deduplicateFlagStrategies keeps one deterministic strategy for each legacy
// (flag_id, environment_id, priority) collision. Variants must be removed
// first because they reference the strategy being discarded.
func deduplicateFlagStrategies(gdb *gorm.DB) error {
	return gdb.Transaction(func(tx *gorm.DB) error {
		var strategies []model.FlagStrategy
		if err := tx.Order("id").Find(&strategies).Error; err != nil {
			return err
		}

		seen := make(map[string]struct{}, len(strategies))
		duplicateIDs := make([]uuid.UUID, 0)
		for _, strategy := range strategies {
			key := strategy.FlagID.String() + "|" + strategy.EnvironmentID.String() + "|" + strconv.Itoa(strategy.Priority)
			if _, exists := seen[key]; exists {
				duplicateIDs = append(duplicateIDs, strategy.ID)
				continue
			}
			seen[key] = struct{}{}
		}
		if len(duplicateIDs) == 0 {
			return nil
		}
		if tx.Migrator().HasTable(&model.FlagStrategyVariant{}) {
			if err := tx.Where("strategy_id IN ?", duplicateIDs).Delete(&model.FlagStrategyVariant{}).Error; err != nil {
				return err
			}
		}
		return tx.Where("id IN ?", duplicateIDs).Delete(&model.FlagStrategy{}).Error
	})
}
