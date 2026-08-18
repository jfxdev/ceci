package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

type ParameterRepository interface {
	List(ctx context.Context, projectID, environmentID uuid.UUID, prefix string) ([]model.Parameter, error)
	Find(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.Parameter, error)
	Upsert(ctx context.Context, param *model.Parameter, version *model.ParameterVersion) error
	Delete(ctx context.Context, projectID, environmentID uuid.UUID, key string, version *model.ParameterVersion) error
	ListVersions(ctx context.Context, parameterID uuid.UUID) ([]model.ParameterVersion, error)
}

type postgresParameterRepository struct {
	db *gorm.DB
}

func NewParameterRepository(db *gorm.DB) ParameterRepository {
	return &postgresParameterRepository{db: db}
}

func (r *postgresParameterRepository) List(ctx context.Context, projectID, environmentID uuid.UUID, prefix string) ([]model.Parameter, error) {
	q := r.db.WithContext(ctx).Where("project_id = ? AND environment_id = ?", projectID, environmentID)
	if prefix != "" {
		q = q.Where("key LIKE ?", prefix+"%")
	}
	var params []model.Parameter
	err := q.Order("key").Find(&params).Error
	return params, err
}

func (r *postgresParameterRepository) Find(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.Parameter, error) {
	var p model.Parameter
	err := r.db.WithContext(ctx).Where("project_id = ? AND environment_id = ? AND key = ?", projectID, environmentID, key).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// Upsert creates or updates a parameter and appends a version row, in a single transaction.
func (r *postgresParameterRepository) Upsert(ctx context.Context, param *model.Parameter, version *model.ParameterVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(param).Error; err != nil {
			return err
		}
		version.ParameterID = param.ID
		return tx.Create(version).Error
	})
}

func (r *postgresParameterRepository) Delete(ctx context.Context, projectID, environmentID uuid.UUID, key string, version *model.ParameterVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p model.Parameter
		if err := tx.Where("project_id = ? AND environment_id = ? AND key = ?", projectID, environmentID, key).First(&p).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		version.ParameterID = p.ID
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		return tx.Delete(&p).Error
	})
}

func (r *postgresParameterRepository) ListVersions(ctx context.Context, parameterID uuid.UUID) ([]model.ParameterVersion, error) {
	var versions []model.ParameterVersion
	err := r.db.WithContext(ctx).
		Where("parameter_id = ?", parameterID).
		Order("version DESC").
		Find(&versions).Error
	return versions, err
}
