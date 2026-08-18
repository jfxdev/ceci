// Package maintenance holds instance-wide maintenance-mode domain logic —
// status and the message/enabled toggle admins set to block writes — split
// out from internal/service so it can be maintained and tested independently
// of the rest of the service layer.
package maintenance

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"leaflag/backend/internal/repository"
)

var ErrMessageRequired = errors.New("maintenance message is required")
var ErrMessageTooLong = errors.New("maintenance message must be 500 characters or fewer")

type Status struct {
	Enabled   bool
	Message   string
	StartedAt *time.Time
	StartedBy uuid.UUID
}

type Service struct {
	settings repository.InstanceSettingsRepository
}

func NewService(settings repository.InstanceSettingsRepository) *Service {
	return &Service{settings: settings}
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return Status{}, err
	}
	return toStatus(settings.MaintenanceEnabled, settings.MaintenanceMessage, settings.MaintenanceStartedAt, settings.MaintenanceStartedBy), nil
}

func (s *Service) Update(ctx context.Context, enabled bool, message string, changedBy uuid.UUID) (Status, error) {
	message = strings.TrimSpace(message)
	if enabled && message == "" {
		return Status{}, ErrMessageRequired
	}
	if utf8.RuneCountInString(message) > 500 {
		return Status{}, ErrMessageTooLong
	}
	settings, err := s.settings.SetMaintenance(ctx, enabled, message, changedBy)
	if err != nil {
		return Status{}, err
	}
	return toStatus(settings.MaintenanceEnabled, settings.MaintenanceMessage, settings.MaintenanceStartedAt, settings.MaintenanceStartedBy), nil
}

func (s *Service) IsMaintenanceEnabled(ctx context.Context) (bool, string, error) {
	status, err := s.Status(ctx)
	return status.Enabled, status.Message, err
}

func toStatus(enabled bool, message string, startedAt *time.Time, startedBy uuid.UUID) Status {
	return Status{Enabled: enabled, Message: message, StartedAt: startedAt, StartedBy: startedBy}
}
