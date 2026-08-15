package routes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"leaflag/backend/internal/service"
)

type fakeMaintenanceService struct {
	status service.MaintenanceStatus
}

func (f *fakeMaintenanceService) Status(context.Context) (service.MaintenanceStatus, error) {
	return f.status, nil
}
func (f *fakeMaintenanceService) Update(_ context.Context, enabled bool, message string, changedBy uuid.UUID) (service.MaintenanceStatus, error) {
	f.status.Enabled = enabled
	f.status.Message = message
	f.status.StartedBy = changedBy
	if enabled && f.status.StartedAt == nil {
		now := time.Now()
		f.status.StartedAt = &now
	}
	return f.status, nil
}

type fakeAdminAuthorizer struct{ admin bool }

func (f fakeAdminAuthorizer) IsAdmin(context.Context, uuid.UUID) (bool, error) { return f.admin, nil }

type maintenanceAuthFake struct{ userID uuid.UUID }

func (f maintenanceAuthFake) ParseAccessToken(string) (uuid.UUID, error) { return f.userID, nil }

func TestMaintenanceRoutes_ReturnStatusAndRequireAdminForUpdates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	started := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	maintenance := &fakeMaintenanceService{status: service.MaintenanceStatus{Enabled: true, Message: "Deploying", StartedAt: &started}}
	r := gin.New()
	registerMaintenanceRoutes(&r.RouterGroup, maintenanceAuthFake{userID: userID}, fakeAdminAuthorizer{admin: true}, maintenance)

	statusRequest := httptest.NewRequest(http.MethodGet, "/maintenance-status", nil)
	statusRequest.Header.Set("Authorization", "Bearer token")
	statusResponse := httptest.NewRecorder()
	r.ServeHTTP(statusResponse, statusRequest)
	assert.Equal(t, http.StatusOK, statusResponse.Code)
	assert.Contains(t, statusResponse.Body.String(), "Deploying")

	updateRequest := httptest.NewRequest(http.MethodPatch, "/admin/maintenance", bytes.NewBufferString(`{"enabled":true,"message":"Database migration"}`))
	updateRequest.Header.Set("Authorization", "Bearer token")
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	r.ServeHTTP(updateResponse, updateRequest)
	assert.Equal(t, http.StatusOK, updateResponse.Code)
	assert.Contains(t, updateResponse.Body.String(), "Database migration")
}

var _ adminAuthorizer = fakeAdminAuthorizer{}
