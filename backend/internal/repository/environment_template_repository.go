package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

type EnvironmentTemplateRepository interface {
	Create(ctx context.Context, template *model.EnvironmentTemplate) error
	List(ctx context.Context) ([]model.EnvironmentTemplate, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentTemplate, error)
	FindByKey(ctx context.Context, key string) (*model.EnvironmentTemplate, error)
	Update(ctx context.Context, template *model.EnvironmentTemplate) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type postgresEnvironmentTemplateRepository struct {
	db *gorm.DB
}

func NewEnvironmentTemplateRepository(db *gorm.DB) EnvironmentTemplateRepository {
	return &postgresEnvironmentTemplateRepository{db: db}
}

func (r *postgresEnvironmentTemplateRepository) Create(ctx context.Context, template *model.EnvironmentTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *postgresEnvironmentTemplateRepository) List(ctx context.Context) ([]model.EnvironmentTemplate, error) {
	var templates []model.EnvironmentTemplate
	err := r.db.WithContext(ctx).Order("sort_order, created_at").Find(&templates).Error
	return templates, err
}

func (r *postgresEnvironmentTemplateRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentTemplate, error) {
	var template model.EnvironmentTemplate
	if err := r.db.WithContext(ctx).First(&template, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &template, nil
}

func (r *postgresEnvironmentTemplateRepository) FindByKey(ctx context.Context, key string) (*model.EnvironmentTemplate, error) {
	var template model.EnvironmentTemplate
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&template).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &template, nil
}

func (r *postgresEnvironmentTemplateRepository) Update(ctx context.Context, template *model.EnvironmentTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

func (r *postgresEnvironmentTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.EnvironmentTemplate{}, "id = ?", id).Error
}
