package routes

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/service"
)

type adminAuthorizer interface {
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
}

type environmentTemplateService interface {
	List(ctx context.Context) ([]model.EnvironmentTemplate, error)
	Create(ctx context.Context, key, name string, isRequired bool) (*model.EnvironmentTemplate, error)
	Update(ctx context.Context, id uuid.UUID, name string, isRequired bool) (*model.EnvironmentTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

func RegisterAdminSettingsRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, admins adminAuthorizer, templates *service.EnvironmentTemplateService) {
	registerAdminSettingsRoutes(rg, auth, admins, templates)
}

func registerAdminSettingsRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, admins adminAuthorizer, templates environmentTemplateService) {
	group := rg.Group("/admin/environment-templates")
	group.Use(middleware.RequireAuth(auth))

	group.GET("", func(c *gin.Context) {
		list, err := templates.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list environment templates"})
			return
		}
		out := make([]dto.EnvironmentTemplateDTO, 0, len(list))
		for _, template := range list {
			out = append(out, toEnvironmentTemplateDTO(&template))
		}
		c.JSON(http.StatusOK, out)
	})

	group.Use(requireAdmin(admins))
	group.POST("", func(c *gin.Context) {
		var req dto.CreateEnvironmentTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		template, err := templates.Create(c.Request.Context(), req.Key, req.Name, req.IsRequired)
		if err != nil {
			if errors.Is(err, service.ErrReservedEnvironmentTemplateKey) {
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "environment template key is reserved"})
				return
			}
			if errors.Is(err, service.ErrEnvironmentTemplateKeyTaken) {
				c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "environment template key already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create environment template"})
			return
		}
		c.JSON(http.StatusCreated, toEnvironmentTemplateDTO(template))
	})

	group.PATCH("/:templateID", func(c *gin.Context) {
		templateID, err := uuid.Parse(c.Param("templateID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid environment template id"})
			return
		}
		var req dto.UpdateEnvironmentTemplateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		template, err := templates.Update(c.Request.Context(), templateID, req.Name, req.IsRequired)
		if err != nil {
			if errors.Is(err, service.ErrEnvironmentTemplateNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "environment template not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update environment template"})
			return
		}
		c.JSON(http.StatusOK, toEnvironmentTemplateDTO(template))
	})

	group.DELETE("/:templateID", func(c *gin.Context) {
		templateID, err := uuid.Parse(c.Param("templateID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid environment template id"})
			return
		}
		if err := templates.Delete(c.Request.Context(), templateID); err != nil {
			if errors.Is(err, service.ErrEnvironmentTemplateNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "environment template not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete environment template"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func requireAdmin(admins adminAuthorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		isAdmin, err := admins.IsAdmin(c.Request.Context(), userID)
		if err != nil || !isAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "admin access required"})
			return
		}
		c.Next()
	}
}

func toEnvironmentTemplateDTO(template *model.EnvironmentTemplate) dto.EnvironmentTemplateDTO {
	return dto.EnvironmentTemplateDTO{ID: template.ID.String(), Key: template.Key, Name: template.Name, IsRequired: template.IsRequired}
}
