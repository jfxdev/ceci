package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/model"
)

const ContextEnvironmentIDKey = "environmentID"

// EnvironmentResolver looks up an environment by its project-scoped key,
// kept as an interface so it can be satisfied by *service.EnvironmentService
// or a test fake.
type EnvironmentResolver interface {
	FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error)
}

// RequireProjectEnvironment reads the envKey URL param and resolves it to an
// environment within the already-resolved project (see RequireProjectRole,
// which must run first). Aborts with 404 if the environment doesn't exist.
func RequireProjectEnvironment(resolver EnvironmentResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.MustGet(ContextProjectIDKey).(uuid.UUID)
		env, err := resolver.FindByKey(c.Request.Context(), projectID, c.Param("envKey"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: "environment not found"})
			return
		}
		c.Set(ContextEnvironmentIDKey, env.ID)
		c.Next()
	}
}
