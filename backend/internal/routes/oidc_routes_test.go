package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestOIDCRoutes_ReportDisabledWithoutConfiguredProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterOIDCRoutes(&r.RouterGroup, nil, nil, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/oidc/config", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"enabled":false}`, w.Body.String())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
