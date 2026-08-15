package routes

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/service"
)

// environmentService is the subset of EnvironmentService behavior routes depend on.
type environmentService interface {
	List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error)
	Create(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error)
	Update(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error)
	Delete(ctx context.Context, projectID uuid.UUID, key string) error
}

func RegisterEnvironmentRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envs *service.EnvironmentService) {
	registerEnvironmentRoutes(rg, auth, roleResolver, envs)
}

func registerEnvironmentRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envs environmentService) {
	scoped := rg.Group("/projects/:projectID/environments")
	scoped.Use(middleware.RequireAuth(auth))

	scoped.GET("", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		list, err := envs.List(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list environments"})
			return
		}
		out := make([]dto.EnvironmentDTO, 0, len(list))
		for _, e := range list {
			out = append(out, toEnvironmentDTO(&e))
		}
		c.JSON(http.StatusOK, out)
	})

	scoped.POST("", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), func(c *gin.Context) {
		var req dto.CreateEnvironmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		e, err := envs.Create(c.Request.Context(), projectID, req.Key, req.Name)
		if err != nil {
			if errors.Is(err, service.ErrEnvironmentKeyTaken) {
				c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "environment key already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create environment"})
			return
		}
		c.JSON(http.StatusCreated, toEnvironmentDTO(e))
	})

	scoped.PATCH("/:envKey", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), func(c *gin.Context) {
		var req dto.UpdateEnvironmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		e, err := envs.Update(c.Request.Context(), projectID, c.Param("envKey"), req.Name)
		if err != nil {
			if errors.Is(err, service.ErrEnvironmentNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "environment not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update environment"})
			return
		}
		c.JSON(http.StatusOK, toEnvironmentDTO(e))
	})

	scoped.DELETE("/:envKey", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := envs.Delete(c.Request.Context(), projectID, c.Param("envKey")); err != nil {
			switch {
			case errors.Is(err, service.ErrEnvironmentNotFound):
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "environment not found"})
			case errors.Is(err, service.ErrLastEnvironment):
				c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "cannot delete the last environment"})
			case errors.Is(err, service.ErrDefaultEnvironment):
				c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "cannot delete the default environment"})
			default:
				c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete environment"})
			}
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func toEnvironmentDTO(e *model.Environment) dto.EnvironmentDTO {
	return dto.EnvironmentDTO{ID: e.ID.String(), Key: e.Key, Name: e.Name, IsDefault: e.Key == constants.DefaultEnvironmentKey}
}
