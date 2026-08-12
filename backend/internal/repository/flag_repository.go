package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ceci/backend/internal/model"
)

type FlagRepository interface {
	List(ctx context.Context, projectID uuid.UUID) ([]model.FeatureFlag, error)
	FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.FeatureFlag, error)
	Create(ctx context.Context, flag *model.FeatureFlag) error
	Update(ctx context.Context, flag *model.FeatureFlag) error
	Delete(ctx context.Context, projectID uuid.UUID, key string) error
	// ReplaceVariantsAndRules atomically swaps a flag's variants and rules.
	ReplaceVariantsAndRules(ctx context.Context, flagID uuid.UUID, variants []model.FlagVariant, rules []model.FlagRule) error
}

type postgresFlagRepository struct {
	db *gorm.DB
}

func NewFlagRepository(db *gorm.DB) FlagRepository {
	return &postgresFlagRepository{db: db}
}

func (r *postgresFlagRepository) List(ctx context.Context, projectID uuid.UUID) ([]model.FeatureFlag, error) {
	var flags []model.FeatureFlag
	err := r.db.WithContext(ctx).
		Preload("Variants").Preload("Rules").
		Where("project_id = ?", projectID).
		Order("key").
		Find(&flags).Error
	return flags, err
}

func (r *postgresFlagRepository) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.FeatureFlag, error) {
	var f model.FeatureFlag
	err := r.db.WithContext(ctx).
		Preload("Variants").Preload("Rules", func(tx *gorm.DB) *gorm.DB { return tx.Order("priority") }).
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
	return r.db.WithContext(ctx).Create(flag).Error
}

func (r *postgresFlagRepository) Update(ctx context.Context, flag *model.FeatureFlag) error {
	return r.db.WithContext(ctx).Model(&model.FeatureFlag{}).
		Where("id = ?", flag.ID).
		Updates(map[string]any{
			"name":            flag.Name,
			"description":     flag.Description,
			"enabled":         flag.Enabled,
			"default_variant": flag.DefaultVariant,
		}).Error
}

func (r *postgresFlagRepository) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	return r.db.WithContext(ctx).Where("project_id = ? AND key = ?", projectID, key).Delete(&model.FeatureFlag{}).Error
}

func (r *postgresFlagRepository) ReplaceVariantsAndRules(ctx context.Context, flagID uuid.UUID, variants []model.FlagVariant, rules []model.FlagRule) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("flag_id = ?", flagID).Delete(&model.FlagRule{}).Error; err != nil {
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
		return nil
	})
}
