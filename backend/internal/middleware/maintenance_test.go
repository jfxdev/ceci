package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type fakeMaintenanceReader struct {
	enabled bool
	message string
	err     error
}

func (f fakeMaintenanceReader) IsMaintenanceEnabled(context.Context) (bool, string, error) {
	return f.enabled, f.message, f.err
}

func TestRequireControlPlaneWritable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireControlPlaneWritable(fakeMaintenanceReader{enabled: true, message: "Database upgrade"}))
	r.GET("/api/v1/projects", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/projects", func(c *gin.Context) { c.Status(http.StatusCreated) })
	r.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.PATCH("/api/v1/admin/maintenance", func(c *gin.Context) { c.Status(http.StatusOK) })

	read := httptest.NewRecorder()
	r.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))
	assert.Equal(t, http.StatusOK, read.Code)

	write := httptest.NewRecorder()
	r.ServeHTTP(write, httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil))
	assert.Equal(t, http.StatusServiceUnavailable, write.Code)
	assert.Contains(t, write.Body.String(), "Database upgrade")
	assert.Contains(t, write.Body.String(), "MAINTENANCE_MODE")

	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
	assert.Equal(t, http.StatusOK, login.Code)

	disable := httptest.NewRecorder()
	r.ServeHTTP(disable, httptest.NewRequest(http.MethodPatch, "/api/v1/admin/maintenance", nil))
	assert.Equal(t, http.StatusOK, disable.Code)
}

func TestRequireControlPlaneWritable_FailsClosedWhenStatusUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireControlPlaneWritable(fakeMaintenanceReader{err: errors.New("db unavailable")}))
	r.POST("/api/v1/projects", func(c *gin.Context) { c.Status(http.StatusCreated) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "MAINTENANCE_STATUS_UNAVAILABLE")
}
