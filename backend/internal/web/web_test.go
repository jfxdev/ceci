package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegisterSPA_ServesIndexOnUnknownRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterSPA(r)

	req := httptest.NewRequest(http.MethodGet, "/projects/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<html>")
}

func TestRegisterSPA_NonGetReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterSPA(r)

	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
