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
	"leaflag/backend/internal/service/contextfield"
)

type contextFieldService interface {
	List(ctx context.Context, projectID uuid.UUID) ([]model.ContextField, error)
	Create(ctx context.Context, projectID uuid.UUID, key, description string, values []contextfield.ValueInput) (*model.ContextField, error)
	Update(ctx context.Context, projectID uuid.UUID, key string, description *string, values *[]contextfield.ValueInput) (*model.ContextField, error)
	Delete(ctx context.Context, projectID uuid.UUID, key string) error
}

func RegisterContextFieldRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, fields *contextfield.Service) {
	registerContextFieldRoutes(rg, auth, roleResolver, fields)
}

func registerContextFieldRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, fields contextFieldService) {
	scoped := rg.Group("/projects/:projectID/context-fields")
	scoped.Use(middleware.RequireAuth(auth))

	scoped.GET("", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		list, err := fields.List(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list context fields"})
			return
		}
		out := make([]dto.ContextFieldDTO, 0, len(list))
		for i := range list {
			out = append(out, toContextFieldDTO(&list[i]))
		}
		c.JSON(http.StatusOK, out)
	})

	scoped.POST("", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), func(c *gin.Context) {
		var req dto.CreateContextFieldRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		field, err := fields.Create(c.Request.Context(), projectID, req.Key, req.Description, toContextFieldValueInputs(req.Values))
		if err != nil {
			switch {
			case errors.Is(err, contextfield.ErrKeyTaken):
				c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "context field key already exists"})
			case errors.Is(err, contextfield.ErrInvalid):
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create context field"})
			}
			return
		}
		c.JSON(http.StatusCreated, toContextFieldDTO(field))
	})

	scoped.PATCH("/:key", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), func(c *gin.Context) {
		var req dto.UpdateContextFieldRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		var values *[]contextfield.ValueInput
		if req.Values != nil {
			converted := toContextFieldValueInputs(*req.Values)
			values = &converted
		}
		field, err := fields.Update(c.Request.Context(), projectID, c.Param("key"), req.Description, values)
		if err != nil {
			switch {
			case errors.Is(err, contextfield.ErrNotFound):
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "context field not found"})
			case errors.Is(err, contextfield.ErrInvalid):
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update context field"})
			}
			return
		}
		c.JSON(http.StatusOK, toContextFieldDTO(field))
	})

	scoped.DELETE("/:key", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := fields.Delete(c.Request.Context(), projectID, c.Param("key")); err != nil {
			if errors.Is(err, contextfield.ErrNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "context field not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete context field"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func toContextFieldValueInputs(values []dto.ContextFieldValueInput) []contextfield.ValueInput {
	out := make([]contextfield.ValueInput, 0, len(values))
	for _, value := range values {
		out = append(out, contextfield.ValueInput{Value: value.Value, Description: value.Description})
	}
	return out
}

func toContextFieldDTO(field *model.ContextField) dto.ContextFieldDTO {
	values := make([]dto.ContextFieldValueDTO, 0, len(field.Values))
	for _, value := range field.Values {
		values = append(values, dto.ContextFieldValueDTO{Value: value.Value, Description: value.Description})
	}
	return dto.ContextFieldDTO{ID: field.ID.String(), Key: field.Key, Description: field.Description, Values: values}
}
