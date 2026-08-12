package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/dto"
)

const ContextProjectIDKey = "projectID"
const ContextProjectRoleKey = "projectRole"

// ProjectRoleResolver looks up the caller's role in a project, kept as an
// interface so it can be satisfied by *service.ProjectService or a test fake.
type ProjectRoleResolver interface {
	RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error)
}

// RequireProjectRole reads the projectID URL param, resolves the caller's
// role and aborts with 404 if they aren't a member (to avoid leaking project
// existence) or 403 if their role doesn't meet the minimum required.
func RequireProjectRole(resolver ProjectRoleResolver, min constants.ProjectRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID, err := uuid.Parse(c.Param("projectID"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: "project not found"})
			return
		}
		userID := c.MustGet(ContextUserIDKey).(uuid.UUID)
		role, err := resolver.RoleOf(c.Request.Context(), projectID, userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: "project not found"})
			return
		}
		if !role.Meets(min) {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "insufficient role"})
			return
		}
		c.Set(ContextProjectIDKey, projectID)
		c.Set(ContextProjectRoleKey, role)
		c.Next()
	}
}
