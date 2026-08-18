package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"leaflag/backend/internal/runtimeplane"
)

type routerSnapshotSource struct{}

func (routerSnapshotSource) Load(context.Context, string) (*runtimeplane.Snapshot, string, bool, error) {
	return &runtimeplane.Snapshot{}, `"snapshot"`, false, nil
}

func TestPlaneRouters_KeepControlAndRuntimeSurfacesSeparate(t *testing.T) {
	dataPlane := NewDataPlaneRouter(runtimeplane.NewStore())
	dataResponse := httptest.NewRecorder()
	dataPlane.ServeHTTP(dataResponse, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))
	assert.Equal(t, http.StatusNotFound, dataResponse.Code)

	controlPlane := NewControlPlaneRouter(Services{}, routerSnapshotSource{}, []string{"sync"})
	runtimeResponse := httptest.NewRecorder()
	controlPlane.ServeHTTP(runtimeResponse, httptest.NewRequest(http.MethodGet, "/ofrep/v1/configuration", nil))
	assert.Equal(t, http.StatusNotFound, runtimeResponse.Code)

	snapshotResponse := httptest.NewRecorder()
	snapshotRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/runtime/snapshot", nil)
	snapshotRequest.Header.Set("Authorization", "Bearer sync")
	controlPlane.ServeHTTP(snapshotResponse, snapshotRequest)
	assert.Equal(t, http.StatusOK, snapshotResponse.Code)
}
