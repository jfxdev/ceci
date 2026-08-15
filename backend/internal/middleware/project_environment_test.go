package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

type fakeEnvironmentResolver struct {
	env *model.Environment
	err error
}

func (f fakeEnvironmentResolver) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error) {
	return f.env, f.err
}

func withProject(projectID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextProjectIDKey, projectID)
		c.Next()
	}
}

func TestRequireProjectEnvironment_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/environments/:envKey", withProject(uuid.New()), RequireProjectEnvironment(fakeEnvironmentResolver{err: repository.ErrNotFound}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/environments/staging", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRequireProjectEnvironment_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	envID := uuid.New()
	r.GET("/environments/:envKey", withProject(uuid.New()), RequireProjectEnvironment(fakeEnvironmentResolver{env: &model.Environment{ID: envID, Key: "staging"}}), func(c *gin.Context) {
		got := c.MustGet(ContextEnvironmentIDKey).(uuid.UUID)
		assert.Equal(t, envID, got)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/environments/staging", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
