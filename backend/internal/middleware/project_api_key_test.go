package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeProjectKeyResolver struct {
	projectID     uuid.UUID
	environmentID uuid.UUID
	err           error
}

func (f fakeProjectKeyResolver) ResolveEnvironment(ctx context.Context, rawKey string) (uuid.UUID, uuid.UUID, error) {
	return f.projectID, f.environmentID, f.err
}

func TestRequireProjectAPIKey_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ofrep", RequireProjectAPIKey(fakeProjectKeyResolver{}), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/ofrep", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireProjectAPIKey_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ofrep", RequireProjectAPIKey(fakeProjectKeyResolver{err: assert.AnError}), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/ofrep", nil)
	req.Header.Set("Authorization", "Bearer bad-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireProjectAPIKey_Success(t *testing.T) {
	projectID := uuid.New()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ofrep", RequireProjectAPIKey(fakeProjectKeyResolver{projectID: projectID}), func(c *gin.Context) {
		got := c.MustGet(ContextProjectIDKey).(uuid.UUID)
		assert.Equal(t, projectID, got)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/ofrep", nil)
	req.Header.Set("Authorization", "Bearer leaflag_sk_good")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
