package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/dto"
)

// RequireRuntimeAPIKey accepts both the existing bearer form used by OFREP
// and X-Consul-Token, allowing Consul KV clients to use their normal header.
func RequireRuntimeAPIKey(resolver ProjectKeyResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey := strings.TrimSpace(c.GetHeader("X-Consul-Token"))
		if rawKey == "" {
			header := c.GetHeader(constants.AuthHeaderName)
			if strings.HasPrefix(header, constants.BearerPrefix) {
				rawKey = strings.TrimPrefix(header, constants.BearerPrefix)
			}
		}
		if rawKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing runtime api key"})
			return
		}
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

// RuntimeScope returns the API-key scope assigned by RequireRuntimeAPIKey.
func RuntimeScope(c *gin.Context) (uuid.UUID, uuid.UUID) {
	return c.MustGet(ContextProjectIDKey).(uuid.UUID), c.MustGet(ContextEnvironmentIDKey).(uuid.UUID)
}
