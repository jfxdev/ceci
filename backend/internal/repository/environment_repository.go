package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

type EnvironmentRepository interface {
	Create(ctx context.Context, env *model.Environment) error
	List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error)
	FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Environment, error)
	Update(ctx context.Context, env *model.Environment) error
	// Delete removes an environment and cascades its flag strategies, flag
	// configs, parameters, and API keys.
	Delete(ctx context.Context, id uuid.UUID) error
	Count(ctx context.Context, projectID uuid.UUID) (int64, error)
}

type postgresEnvironmentRepository struct {
	db *gorm.DB
}

func NewEnvironmentRepository(db *gorm.DB) EnvironmentRepository {
	return &postgresEnvironmentRepository{db: db}
}

func (r *postgresEnvironmentRepository) Create(ctx context.Context, env *model.Environment) error {
	return r.db.WithContext(ctx).Create(env).Error
}

func (r *postgresEnvironmentRepository) List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error) {
	var envs []model.Environment
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("sort_order, created_at").
		Find(&envs).Error
	return envs, err
}

func (r *postgresEnvironmentRepository) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error) {
	var e model.Environment
	err := r.db.WithContext(ctx).Where("project_id = ? AND key = ?", projectID, key).First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (r *postgresEnvironmentRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	var e model.Environment
	err := r.db.WithContext(ctx).First(&e, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (r *postgresEnvironmentRepository) Update(ctx context.Context, env *model.Environment) error {
	return r.db.WithContext(ctx).Save(env).Error
}

func (r *postgresEnvironmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := deleteStrategiesAndVariants(tx, "environment_id = ?", id); err != nil {
			return err
		}
		if err := tx.Where("environment_id = ?", id).Delete(&model.FlagEnvironmentConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("environment_id = ?", id).Delete(&model.Parameter{}).Error; err != nil {
			return err
		}
		if err := tx.Where("environment_id = ?", id).Delete(&model.ProjectAPIKey{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Environment{}, "id = ?", id).Error
	})
}

func (r *postgresEnvironmentRepository) Count(ctx context.Context, projectID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Environment{}).Where("project_id = ?", projectID).Count(&count).Error
	return count, err
}
