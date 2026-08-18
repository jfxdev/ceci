package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/dto"
)

// ProjectKeyResolver validates a raw project API key and returns the
// project and environment it belongs to, kept as an interface so it can be
// satisfied by *apikey.Service or a test fake.
type ProjectKeyResolver interface {
	ResolveEnvironment(ctx context.Context, rawKey string) (projectID, environmentID uuid.UUID, err error)
}

// RequireProjectAPIKey authenticates OFREP requests via an
// environment-scoped bearer API key, separate from user JWT auth
// (SDKs/services, not humans).
func RequireProjectAPIKey(resolver ProjectKeyResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader(constants.AuthHeaderName)
		if !strings.HasPrefix(header, constants.BearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing bearer api key"})
			return
		}
		rawKey := strings.TrimPrefix(header, constants.BearerPrefix)
		projectID, environmentID, err := resolver.ResolveEnvironment(c.Request.Context(), rawKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid or revoked api key"})
			return
		}
		c.Set(ContextProjectIDKey, projectID)
		c.Set(ContextEnvironmentIDKey, environmentID)
		c.Next()
	}
}
