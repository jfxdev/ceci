package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

type ContextFieldRepository interface {
	List(ctx context.Context, projectID uuid.UUID) ([]model.ContextField, error)
	FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.ContextField, error)
	Create(ctx context.Context, field *model.ContextField) error
	Update(ctx context.Context, field *model.ContextField) error
	ReplaceValues(ctx context.Context, fieldID uuid.UUID, values []model.ContextFieldValue) error
	Delete(ctx context.Context, fieldID uuid.UUID) error
}

type postgresContextFieldRepository struct{ db *gorm.DB }

func NewContextFieldRepository(db *gorm.DB) ContextFieldRepository {
	return &postgresContextFieldRepository{db: db}
}

func (r *postgresContextFieldRepository) List(ctx context.Context, projectID uuid.UUID) ([]model.ContextField, error) {
	var fields []model.ContextField
	err := r.db.WithContext(ctx).Preload("Values", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("sort_order, created_at")
	}).Where("project_id = ?", projectID).Order("key").Find(&fields).Error
	return fields, err
}

func (r *postgresContextFieldRepository) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.ContextField, error) {
	var field model.ContextField
	err := r.db.WithContext(ctx).Preload("Values", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("sort_order, created_at")
	}).Where("project_id = ? AND key = ?", projectID, key).First(&field).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &field, nil
}

func (r *postgresContextFieldRepository) Create(ctx context.Context, field *model.ContextField) error {
	return r.db.WithContext(ctx).Omit("Values").Create(field).Error
}

func (r *postgresContextFieldRepository) Update(ctx context.Context, field *model.ContextField) error {
	return r.db.WithContext(ctx).Model(&model.ContextField{}).Where("id = ?", field.ID).Updates(map[string]any{
		"description": field.Description,
	}).Error
}

func (r *postgresContextFieldRepository) ReplaceValues(ctx context.Context, fieldID uuid.UUID, values []model.ContextFieldValue) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("context_field_id = ?", fieldID).Delete(&model.ContextFieldValue{}).Error; err != nil {
			return err
		}
		if len(values) == 0 {
			return nil
		}
		for i := range values {
			values[i].ContextFieldID = fieldID
		}
		return tx.Create(&values).Error
	})
}

func (r *postgresContextFieldRepository) Delete(ctx context.Context, fieldID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("context_field_id = ?", fieldID).Delete(&model.ContextFieldValue{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ContextField{}, "id = ?", fieldID).Error
	})
}
