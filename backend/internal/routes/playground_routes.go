package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/service"
)

// playgroundFlagService is the subset of FlagService behavior the playground
// route depends on.
type playgroundFlagService interface {
	EvaluateAll(ctx context.Context, projectID, environmentID uuid.UUID, evalCtx map[string]any) ([]service.EvaluationResult, error)
}

type playgroundEvaluateRequest struct {
	Context map[string]any `json:"context"`
}

type playgroundResponse struct {
	Flags []ofrepFlagResponse `json:"flags"`
}

// RegisterPlaygroundRoutes exposes a session-authenticated, read-only
// equivalent of the OFREP bulk-evaluate endpoint so members can test
// targeting rules against an arbitrary context without an API key.
func RegisterPlaygroundRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envResolver middleware.EnvironmentResolver, flags *service.FlagService) {
	registerPlaygroundRoutes(rg, auth, roleResolver, envResolver, flags)
}

func registerPlaygroundRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envResolver middleware.EnvironmentResolver, flags playgroundFlagService) {
	scoped := rg.Group("/projects/:projectID/environments/:envKey/playground")
	scoped.Use(middleware.RequireAuth(auth))

	scoped.POST("/evaluate", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		var req playgroundEvaluateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		results, err := flags.EvaluateAll(c.Request.Context(), projectID, environmentID, req.Context)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to evaluate flags"})
			return
		}
		out := make([]ofrepFlagResponse, 0, len(results))
		for _, r := range results {
			out = append(out, toOFREPResponse(r))
		}
		c.JSON(http.StatusOK, playgroundResponse{Flags: out})
	})
}
