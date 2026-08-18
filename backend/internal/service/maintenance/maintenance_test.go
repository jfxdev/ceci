package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/model"
)

type fakeInstanceSettingsRepository struct{ settings model.InstanceSettings }

func (f *fakeInstanceSettingsRepository) Get(context.Context) (*model.InstanceSettings, error) {
	return &f.settings, nil
}

func (f *fakeInstanceSettingsRepository) SetOIDC(_ context.Context, settings *model.InstanceSettings) error {
	f.settings = *settings
	return nil
}

func (f *fakeInstanceSettingsRepository) SetMaintenance(_ context.Context, enabled bool, message string, changedBy uuid.UUID) (*model.InstanceSettings, error) {
	f.settings.MaintenanceEnabled = enabled
	f.settings.MaintenanceMessage = message
	if enabled && f.settings.MaintenanceStartedAt == nil {
		now := time.Now()
		f.settings.MaintenanceStartedAt = &now
		f.settings.MaintenanceStartedBy = changedBy
	}
	if !enabled {
		f.settings.MaintenanceMessage = ""
		f.settings.MaintenanceStartedAt = nil
		f.settings.MaintenanceStartedBy = uuid.Nil
	}
	return &f.settings, nil
}

func TestMaintenanceService_ValidatesAndPreservesCustomMessage(t *testing.T) {
	repo := &fakeInstanceSettingsRepository{}
	svc := NewService(repo)
	adminID := uuid.New()

	_, err := svc.Update(context.Background(), true, "   ", adminID)
	assert.ErrorIs(t, err, ErrMessageRequired)

	status, err := svc.Update(context.Background(), true, "  Updating the database.  ", adminID)
	require.NoError(t, err)
	assert.True(t, status.Enabled)
	assert.Equal(t, "Updating the database.", status.Message)
	assert.Equal(t, adminID, status.StartedBy)
	require.NotNil(t, status.StartedAt)

	status, err = svc.Update(context.Background(), false, "ignored", adminID)
	require.NoError(t, err)
	assert.False(t, status.Enabled)
	assert.Empty(t, status.Message)
	assert.Nil(t, status.StartedAt)
}
