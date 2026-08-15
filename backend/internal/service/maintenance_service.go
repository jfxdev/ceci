package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"leaflag/backend/internal/repository"
)

var ErrMaintenanceMessageRequired = errors.New("maintenance message is required")
var ErrMaintenanceMessageTooLong = errors.New("maintenance message must be 500 characters or fewer")

type MaintenanceStatus struct {
	Enabled   bool
	Message   string
	StartedAt *time.Time
	StartedBy uuid.UUID
}

type MaintenanceService struct {
	settings repository.InstanceSettingsRepository
}

func NewMaintenanceService(settings repository.InstanceSettingsRepository) *MaintenanceService {
	return &MaintenanceService{settings: settings}
}

func (s *MaintenanceService) Status(ctx context.Context) (MaintenanceStatus, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return MaintenanceStatus{}, err
	}
	return toMaintenanceStatus(settings.MaintenanceEnabled, settings.MaintenanceMessage, settings.MaintenanceStartedAt, settings.MaintenanceStartedBy), nil
}

func (s *MaintenanceService) Update(ctx context.Context, enabled bool, message string, changedBy uuid.UUID) (MaintenanceStatus, error) {
	message = strings.TrimSpace(message)
	if enabled && message == "" {
		return MaintenanceStatus{}, ErrMaintenanceMessageRequired
	}
	if utf8.RuneCountInString(message) > 500 {
		return MaintenanceStatus{}, ErrMaintenanceMessageTooLong
	}
	settings, err := s.settings.SetMaintenance(ctx, enabled, message, changedBy)
	if err != nil {
		return MaintenanceStatus{}, err
	}
	return toMaintenanceStatus(settings.MaintenanceEnabled, settings.MaintenanceMessage, settings.MaintenanceStartedAt, settings.MaintenanceStartedBy), nil
}

func (s *MaintenanceService) IsMaintenanceEnabled(ctx context.Context) (bool, string, error) {
	status, err := s.Status(ctx)
	return status.Enabled, status.Message, err
}

func toMaintenanceStatus(enabled bool, message string, startedAt *time.Time, startedBy uuid.UUID) MaintenanceStatus {
	return MaintenanceStatus{Enabled: enabled, Message: message, StartedAt: startedAt, StartedBy: startedBy}
}
