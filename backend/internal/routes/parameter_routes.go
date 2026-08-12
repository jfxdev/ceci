package routes

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/dto"
	"ceci/backend/internal/middleware"
	"ceci/backend/internal/model"
	"ceci/backend/internal/service"
)

// parameterService is the subset of ParameterService behavior routes depend on.
type parameterService interface {
	List(ctx context.Context, projectID uuid.UUID, prefix string) ([]model.Parameter, error)
	Get(ctx context.Context, projectID uuid.UUID, key string) (*model.Parameter, error)
	Upsert(ctx context.Context, projectID uuid.UUID, key, value string, changedBy uuid.UUID) (*model.Parameter, error)
	Delete(ctx context.Context, projectID uuid.UUID, key string, deletedBy uuid.UUID) error
	ListVersions(ctx context.Context, projectID uuid.UUID, key string) ([]model.ParameterVersion, error)
}

// Parameter keys may contain "/" (e.g. "service/db/host", Consul-style), so
// single-value and version-history routes live under their own static
// prefix ("value/", "versions/") followed by a wildcard, keeping them
// unambiguous for gin's router instead of colliding on a bare ":key" param.
func RegisterParameterRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, params *service.ParameterService) {
	registerParameterRoutes(rg, auth, roleResolver, params)
}

func registerParameterRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, params parameterService) {
	scoped := rg.Group("/projects/:projectID/parameters")
	scoped.Use(middleware.RequireAuth(auth))

	scoped.GET("", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		prefix := c.Query("prefix")
		list, err := params.List(c.Request.Context(), projectID, prefix)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list parameters"})
			return
		}
		out := make([]dto.ParameterDTO, 0, len(list))
		for _, p := range list {
			out = append(out, dto.ParameterDTO{Key: p.Key, Value: p.Value, Version: p.Version, UpdatedAt: p.UpdatedAt.Format(timeLayout)})
		}
		c.JSON(http.StatusOK, out)
	})

	scoped.GET("/value/*key", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		p, err := params.Get(c.Request.Context(), projectID, wildcardKey(c))
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "parameter not found"})
			return
		}
		c.JSON(http.StatusOK, dto.ParameterDTO{Key: p.Key, Value: p.Value, Version: p.Version, UpdatedAt: p.UpdatedAt.Format(timeLayout)})
	})

	scoped.PUT("/value/*key", middleware.RequireProjectRole(roleResolver, constants.RoleEditor), func(c *gin.Context) {
		var req dto.UpsertParameterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		p, err := params.Upsert(c.Request.Context(), projectID, wildcardKey(c), req.Value, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to upsert parameter"})
			return
		}
		c.JSON(http.StatusOK, dto.ParameterDTO{Key: p.Key, Value: p.Value, Version: p.Version, UpdatedAt: p.UpdatedAt.Format(timeLayout)})
	})

	scoped.DELETE("/value/*key", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		err := params.Delete(c.Request.Context(), projectID, wildcardKey(c), userID)
		switch {
		case err == nil:
			c.Status(http.StatusNoContent)
		case errors.Is(err, service.ErrParameterNotFound):
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "parameter not found"})
		default:
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete parameter"})
		}
	})

	scoped.GET("/versions/*key", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		versions, err := params.ListVersions(c.Request.Context(), projectID, wildcardKey(c))
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "parameter not found"})
			return
		}
		out := make([]dto.ParameterVersionDTO, 0, len(versions))
		for _, v := range versions {
			out = append(out, dto.ParameterVersionDTO{
				Version:    v.Version,
				Value:      v.Value,
				ChangeType: v.ChangeType,
				ChangedAt:  v.ChangedAt.Format(timeLayout),
			})
		}
		c.JSON(http.StatusOK, out)
	})
}

// wildcardKey strips the leading slash gin includes in a "*key" wildcard match.
func wildcardKey(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("key"), "/")
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
