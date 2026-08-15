package routes

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/service"
)

type maintenanceService interface {
	Status(ctx context.Context) (service.MaintenanceStatus, error)
	Update(ctx context.Context, enabled bool, message string, changedBy uuid.UUID) (service.MaintenanceStatus, error)
}

func RegisterMaintenanceRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, admins adminAuthorizer, maintenance *service.MaintenanceService) {
	registerMaintenanceRoutes(rg, auth, admins, maintenance)
}

func registerMaintenanceRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, admins adminAuthorizer, maintenance maintenanceService) {
	rg.GET("/maintenance-status", middleware.RequireAuth(auth), func(c *gin.Context) {
		status, err := maintenance.Status(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to load maintenance status"})
			return
		}
		c.JSON(http.StatusOK, toMaintenanceStatusDTO(status))
	})

	admin := rg.Group("/admin/maintenance")
	admin.Use(middleware.RequireAuth(auth), requireAdmin(admins))
	admin.PATCH("", func(c *gin.Context) {
		var req dto.UpdateMaintenanceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		status, err := maintenance.Update(c.Request.Context(), req.Enabled, req.Message, userID)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrMaintenanceMessageRequired), errors.Is(err, service.ErrMaintenanceMessageTooLong):
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update maintenance mode"})
			}
			return
		}
		c.JSON(http.StatusOK, toMaintenanceStatusDTO(status))
	})
}

func toMaintenanceStatusDTO(status service.MaintenanceStatus) dto.MaintenanceStatusDTO {
	out := dto.MaintenanceStatusDTO{Enabled: status.Enabled, Message: status.Message}
	if status.StartedAt != nil {
		formatted := status.StartedAt.Format(timeLayout)
		out.StartedAt = &formatted
	}
	if status.StartedBy != uuid.Nil {
		out.StartedBy = status.StartedBy.String()
	}
	return out
}
