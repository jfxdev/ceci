package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

type InstanceSettingsRepository interface {
	Get(ctx context.Context) (*model.InstanceSettings, error)
	SetMaintenance(ctx context.Context, enabled bool, message string, changedBy uuid.UUID) (*model.InstanceSettings, error)
	SetOIDC(ctx context.Context, settings *model.InstanceSettings) error
}

func (r *postgresInstanceSettingsRepository) SetOIDC(ctx context.Context, settings *model.InstanceSettings) error {
	settings.ID = 1
	return r.db.WithContext(ctx).Save(settings).Error
}

type postgresInstanceSettingsRepository struct{ db *gorm.DB }

func NewInstanceSettingsRepository(db *gorm.DB) InstanceSettingsRepository {
	return &postgresInstanceSettingsRepository{db: db}
}

func (r *postgresInstanceSettingsRepository) Get(ctx context.Context) (*model.InstanceSettings, error) {
	settings := model.InstanceSettings{ID: 1}
	err := r.db.WithContext(ctx).First(&settings, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.db.WithContext(ctx).Create(&settings).Error; err != nil {
			return nil, err
		}
		return &settings, nil
	}
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *postgresInstanceSettingsRepository) SetMaintenance(ctx context.Context, enabled bool, message string, changedBy uuid.UUID) (*model.InstanceSettings, error) {
	var out model.InstanceSettings
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		settings := model.InstanceSettings{ID: 1}
		err := tx.First(&settings, 1).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&settings).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		settings.MaintenanceEnabled = enabled
		if enabled {
			settings.MaintenanceMessage = message
			if settings.MaintenanceStartedAt == nil {
				now := time.Now().UTC()
				settings.MaintenanceStartedAt = &now
				settings.MaintenanceStartedBy = changedBy
			}
		} else {
			settings.MaintenanceMessage = ""
			settings.MaintenanceStartedAt = nil
			settings.MaintenanceStartedBy = uuid.Nil
		}
		if err := tx.Save(&settings).Error; err != nil {
			return err
		}
		out = settings
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}
