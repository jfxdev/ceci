package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/dto"
)

const ContextUserIDKey = "userID"

// TokenParser is the minimal capability RequireAuth needs, kept as an
// interface so it can be satisfied by *auth.Service or a test fake.
type TokenParser interface {
	ParseAccessToken(tokenStr string) (uuid.UUID, error)
}

func RequireAuth(auth TokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader(constants.AuthHeaderName)
		if !strings.HasPrefix(header, constants.BearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing bearer token"})
			return
		}
		tokenStr := strings.TrimPrefix(header, constants.BearerPrefix)
		userID, err := auth.ParseAccessToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid or expired token"})
			return
		}
		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}
