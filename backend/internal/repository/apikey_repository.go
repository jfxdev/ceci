package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ceci/backend/internal/model"
)

type APIKeyRepository interface {
	Create(ctx context.Context, key *model.ProjectAPIKey) error
	FindActiveByHash(ctx context.Context, keyHash string) (*model.ProjectAPIKey, error)
	List(ctx context.Context, projectID uuid.UUID) ([]model.ProjectAPIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}

type postgresAPIKeyRepository struct {
	db *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) APIKeyRepository {
	return &postgresAPIKeyRepository{db: db}
}

func (r *postgresAPIKeyRepository) Create(ctx context.Context, key *model.ProjectAPIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *postgresAPIKeyRepository) FindActiveByHash(ctx context.Context, keyHash string) (*model.ProjectAPIKey, error) {
	var k model.ProjectAPIKey
	err := r.db.WithContext(ctx).
		Where("key_hash = ? AND revoked_at IS NULL", keyHash).
		First(&k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *postgresAPIKeyRepository) List(ctx context.Context, projectID uuid.UUID) ([]model.ProjectAPIKey, error) {
	var keys []model.ProjectAPIKey
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at desc").Find(&keys).Error
	return keys, err
}

func (r *postgresAPIKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ProjectAPIKey{}).Where("id = ?", id).Update("revoked_at", now).Error
}
