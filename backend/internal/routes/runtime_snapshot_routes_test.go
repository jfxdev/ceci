package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"leaflag/backend/internal/runtimeplane"
)

type fakeSnapshotSource struct {
	notModified bool
}

func (f fakeSnapshotSource) Load(_ context.Context, _ string) (*runtimeplane.Snapshot, string, bool, error) {
	return &runtimeplane.Snapshot{}, `"runtime-etag"`, f.notModified, nil
}

func TestRuntimeSnapshotRoute_RequiresTokenAndSupportsETag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRuntimeSnapshotRoutes(&r.RouterGroup, fakeSnapshotSource{}, []string{"current", "previous"})

	unauthorized := httptest.NewRecorder()
	r.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/internal/v1/runtime/snapshot", nil))
	assert.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/runtime/snapshot", nil)
	req.Header.Set("Authorization", "Bearer previous")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `"runtime-etag"`, w.Header().Get("ETag"))
}
