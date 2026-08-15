package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

type FlagRepository interface {
	List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error)
	FindByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error)
	Create(ctx context.Context, flag *model.FeatureFlag) error
	// UpdateCore updates the environment-independent fields of a flag: name,
	// description, and prerequisite.
	UpdateCore(ctx context.Context, flag *model.FeatureFlag) error
	Delete(ctx context.Context, projectID uuid.UUID, key string) error
	// ReplaceVariantsAndRules atomically replaces a flag's variant catalog
	// (shared across all environments) and, scoped to a single environment,
	// its targeting rules.
	ReplaceVariantsAndRules(ctx context.Context, flagID, environmentID uuid.UUID, variants []model.FlagVariant, rules []model.FlagRule) error
	// UpsertEnvironmentConfig sets whether a flag is enabled and its default
	// variant within one environment.
	UpsertEnvironmentConfig(ctx context.Context, flagID, environmentID uuid.UUID, enabled bool, defaultVariant string) error
	// Version returns the flag count and the most recent updated_at across a
	// project's flags, used to build an OFREP polling ETag.
	Version(ctx context.Context, projectID uuid.UUID) (int64, time.Time, error)
}

type postgresFlagRepository struct {
	db *gorm.DB
}

func NewFlagRepository(db *gorm.DB) FlagRepository {
	return &postgresFlagRepository{db: db}
}

func (r *postgresFlagRepository) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error) {
	var flags []model.FeatureFlag
	err := r.db.WithContext(ctx).
		Preload("Variants").
		Preload("Configs", func(tx *gorm.DB) *gorm.DB { return tx.Where("environment_id = ?", environmentID) }).
		Preload("Rules", func(tx *gorm.DB) *gorm.DB { return tx.Where("environment_id = ?", environmentID).Order("priority") }).
		Where("project_id = ?", projectID).
		Order("key").
		Find(&flags).Error
	return flags, err
}

func (r *postgresFlagRepository) FindByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error) {
	var f model.FeatureFlag
	err := r.db.WithContext(ctx).
		Preload("Variants").
		Preload("Configs", func(tx *gorm.DB) *gorm.DB { return tx.Where("environment_id = ?", environmentID) }).
		Preload("Rules", func(tx *gorm.DB) *gorm.DB { return tx.Where("environment_id = ?", environmentID).Order("priority") }).
		Where("project_id = ? AND key = ?", projectID, key).
		First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &f, nil
}

func (r *postgresFlagRepository) Create(ctx context.Context, flag *model.FeatureFlag) error {
	return r.db.WithContext(ctx).Omit("Variants", "Configs", "Rules").Create(flag).Error
}

func (r *postgresFlagRepository) UpdateCore(ctx context.Context, flag *model.FeatureFlag) error {
	return r.db.WithContext(ctx).Model(&model.FeatureFlag{}).
		Where("id = ?", flag.ID).
		Updates(map[string]any{
			"name":                  flag.Name,
			"description":           flag.Description,
			"prerequisite_flag_key": flag.PrerequisiteFlagKey,
			"prerequisite_variant":  flag.PrerequisiteVariant,
		}).Error
}

func (r *postgresFlagRepository) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var f model.FeatureFlag
		if err := tx.Where("project_id = ? AND key = ?", projectID, key).First(&f).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if err := tx.Where("flag_id = ?", f.ID).Delete(&model.FlagRule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flag_id = ?", f.ID).Delete(&model.FlagEnvironmentConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flag_id = ?", f.ID).Delete(&model.FlagVariant{}).Error; err != nil {
			return err
		}
		return tx.Delete(&f).Error
	})
}

func (r *postgresFlagRepository) ReplaceVariantsAndRules(ctx context.Context, flagID, environmentID uuid.UUID, variants []model.FlagVariant, rules []model.FlagRule) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("flag_id = ? AND environment_id = ?", flagID, environmentID).Delete(&model.FlagRule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flag_id = ?", flagID).Delete(&model.FlagVariant{}).Error; err != nil {
			return err
		}
		if len(variants) > 0 {
			if err := tx.Create(&variants).Error; err != nil {
				return err
			}
		}
		if len(rules) > 0 {
			if err := tx.Create(&rules).Error; err != nil {
				return err
			}
		}
		// Rule/variant-only edits don't otherwise touch the parent flag row,
		// so bump its updated_at explicitly to keep the project flags
		// version (see Version) accurate for OFREP ETag polling.
		return tx.Model(&model.FeatureFlag{}).Where("id = ?", flagID).UpdateColumn("updated_at", time.Now()).Error
	})
}

func (r *postgresFlagRepository) UpsertEnvironmentConfig(ctx context.Context, flagID, environmentID uuid.UUID, enabled bool, defaultVariant string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cfg model.FlagEnvironmentConfig
		err := tx.Where("flag_id = ? AND environment_id = ?", flagID, environmentID).First(&cfg).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&model.FlagEnvironmentConfig{
				FlagID: flagID, EnvironmentID: environmentID, Enabled: enabled, DefaultVariant: defaultVariant,
			}).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			cfg.Enabled = enabled
			cfg.DefaultVariant = defaultVariant
			if err := tx.Save(&cfg).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.FeatureFlag{}).Where("id = ?", flagID).UpdateColumn("updated_at", time.Now()).Error
	})
}

// Version returns the flag count and the most recent updated_at across a
// project's flags — used to build an OFREP polling ETag without hashing the
// full flag payload on every request.
//
// Deliberately avoids a single aggregate `max(updated_at)` query: the sqlite
// driver used in tests returns that column as a string rather than a
// scannable time.Time, and letting gorm scan a real model.FeatureFlag row
// keeps time parsing consistent with every other query in this repository.
func (r *postgresFlagRepository) Version(ctx context.Context, projectID uuid.UUID) (int64, time.Time, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.FeatureFlag{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		return 0, time.Time{}, err
	}
	if count == 0 {
		return 0, time.Time{}, nil
	}
	var latest model.FeatureFlag
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("updated_at DESC").
		Limit(1).
		Find(&latest).Error
	if err != nil {
		return 0, time.Time{}, err
	}
	return count, latest.UpdatedAt, nil
}
