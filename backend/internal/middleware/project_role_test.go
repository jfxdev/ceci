package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/repository"
)

type fakeRoleResolver struct {
	role constants.ProjectRole
	err  error
}

func (f fakeRoleResolver) RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	return f.role, f.err
}

func withUser(userID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}

func TestRequireProjectRole_NotMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/projects/:projectID", withUser(uuid.New()), RequireProjectRole(fakeRoleResolver{err: repository.ErrNotFound}, constants.RoleViewer), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/projects/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRequireProjectRole_InvalidProjectID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/projects/:projectID", withUser(uuid.New()), RequireProjectRole(fakeRoleResolver{}, constants.RoleViewer), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/projects/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRequireProjectRole_InsufficientRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/projects/:projectID", withUser(uuid.New()), RequireProjectRole(fakeRoleResolver{role: constants.RoleViewer}, constants.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/projects/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireProjectRole_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/projects/:projectID", withUser(uuid.New()), RequireProjectRole(fakeRoleResolver{role: constants.RoleOwner}, constants.RoleAdmin), func(c *gin.Context) {
		role := c.MustGet(ContextProjectRoleKey).(constants.ProjectRole)
		assert.Equal(t, constants.RoleOwner, role)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/projects/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
