package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/runtimeplane"
)

func TestRuntimeKVRoute_SupportsConsulTokenRawAndPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectID, environmentID := uuid.New(), uuid.New()
	rawKey := "leaflag_sk_consul"
	hash := sha256.Sum256([]byte(rawKey))
	store := runtimeplane.NewStore()
	require.NoError(t, store.Replace(&runtimeplane.Snapshot{
		APIKeys: []runtimeplane.APIKey{{KeyHash: hex.EncodeToString(hash[:]), ProjectID: projectID, EnvironmentID: environmentID}},
		Parameters: []runtimeplane.RuntimeParameter{
			{ProjectID: projectID, EnvironmentID: environmentID, Key: "app/name", Value: "leaflag", Version: 2},
			{ProjectID: projectID, EnvironmentID: environmentID, Key: "app/port", Value: "8110", Version: 3},
		},
	}, `"snapshot"`))
	r := gin.New()
	RegisterRuntimeKVRoutes(&r.RouterGroup, store, store)

	rawRequest := httptest.NewRequest(http.MethodGet, "/v1/kv/app/name?raw", nil)
	rawRequest.Header.Set("X-Consul-Token", rawKey)
	rawResponse := httptest.NewRecorder()
	r.ServeHTTP(rawResponse, rawRequest)
	assert.Equal(t, http.StatusOK, rawResponse.Code)
	assert.Equal(t, "leaflag", rawResponse.Body.String())

	prefixRequest := httptest.NewRequest(http.MethodGet, "/v1/kv/app/?keys&separator=/", nil)
	prefixRequest.Header.Set("Authorization", "Bearer "+rawKey)
	prefixResponse := httptest.NewRecorder()
	r.ServeHTTP(prefixResponse, prefixRequest)
	assert.Equal(t, http.StatusOK, prefixResponse.Code)
	assert.JSONEq(t, `["app/name","app/port"]`, prefixResponse.Body.String())
	assert.Equal(t, "3", prefixResponse.Header().Get("X-Consul-Index"))
}
