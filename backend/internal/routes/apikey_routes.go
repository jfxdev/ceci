package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/service/apikey"
)

// apiKeyService is the subset of APIKeyService behavior routes depend on.
type apiKeyService interface {
	Create(ctx context.Context, projectID, environmentID uuid.UUID, label string) (rawKey string, key *model.ProjectAPIKey, err error)
	List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.ProjectAPIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}

func RegisterAPIKeyRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envResolver middleware.EnvironmentResolver, keys *apikey.Service) {
	registerAPIKeyRoutes(rg, auth, roleResolver, envResolver, keys)
}

func registerAPIKeyRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envResolver middleware.EnvironmentResolver, keys apiKeyService) {
	scoped := rg.Group("/projects/:projectID/environments/:envKey/api-keys")
	scoped.Use(middleware.RequireAuth(auth))

	scoped.GET("", middleware.RequireProjectRole(roleResolver, constants.RoleOwner), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		list, err := keys.List(c.Request.Context(), projectID, environmentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list api keys"})
			return
		}
		out := make([]dto.APIKeyDTO, 0, len(list))
		for _, k := range list {
			out = append(out, dto.APIKeyDTO{ID: k.ID.String(), Label: k.Label, Prefix: k.Prefix, EnvironmentID: k.EnvironmentID.String(), Revoked: k.RevokedAt != nil})
		}
		c.JSON(http.StatusOK, out)
	})

	scoped.POST("", middleware.RequireProjectRole(roleResolver, constants.RoleOwner), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		var req dto.CreateAPIKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		rawKey, key, err := keys.Create(c.Request.Context(), projectID, environmentID, req.Label)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create api key"})
			return
		}
		c.JSON(http.StatusCreated, dto.APIKeyCreatedDTO{ID: key.ID.String(), Label: key.Label, Prefix: key.Prefix, RawKey: rawKey})
	})

	scoped.DELETE("/:keyID", middleware.RequireProjectRole(roleResolver, constants.RoleOwner), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		keyID, err := uuid.Parse(c.Param("keyID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid key id"})
			return
		}
		if err := keys.Revoke(c.Request.Context(), keyID); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to revoke api key"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}
