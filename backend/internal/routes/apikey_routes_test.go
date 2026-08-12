package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
)

type fakeAPIKeyService struct {
	rawKey    string
	createdKey *model.ProjectAPIKey
	createErr  error

	list    []model.ProjectAPIKey
	listErr error

	revokeErr error
}

func (f *fakeAPIKeyService) Create(ctx context.Context, projectID uuid.UUID, label string) (string, *model.ProjectAPIKey, error) {
	return f.rawKey, f.createdKey, f.createErr
}
func (f *fakeAPIKeyService) List(ctx context.Context, projectID uuid.UUID) ([]model.ProjectAPIKey, error) {
	return f.list, f.listErr
}
func (f *fakeAPIKeyService) Revoke(ctx context.Context, id uuid.UUID) error {
	return f.revokeErr
}

func newAPIKeyTestRouter(keys apiKeyService, role constants.ProjectRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authFake := &fakeAuthService{parseUserID: uuid.New()}
	resolver := fakeRoleResolver{role: role}
	registerAPIKeyRoutes(&r.RouterGroup, authFake, resolver, keys)
	return r
}

func TestListAPIKeysRoute_Forbidden(t *testing.T) {
	r := newAPIKeyTestRouter(&fakeAPIKeyService{}, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/api-keys", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateAPIKeyRoute_Success(t *testing.T) {
	fake := &fakeAPIKeyService{rawKey: "ceci_sk_abc123", createdKey: &model.ProjectAPIKey{ID: uuid.New(), Label: "CI", Prefix: "ceci_sk_ab"}}
	r := newAPIKeyTestRouter(fake, constants.RoleOwner)

	body, _ := json.Marshal(map[string]string{"label": "CI"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/api-keys", body))

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "ceci_sk_abc123")
}

func TestListAPIKeysRoute_Success(t *testing.T) {
	fake := &fakeAPIKeyService{list: []model.ProjectAPIKey{{ID: uuid.New(), Label: "CI", Prefix: "ceci_sk_ab"}}}
	r := newAPIKeyTestRouter(fake, constants.RoleOwner)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/api-keys", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CI")
}

func TestRevokeAPIKeyRoute_InvalidID(t *testing.T) {
	r := newAPIKeyTestRouter(&fakeAPIKeyService{}, constants.RoleOwner)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/api-keys/not-a-uuid", nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRevokeAPIKeyRoute_Success(t *testing.T) {
	r := newAPIKeyTestRouter(&fakeAPIKeyService{}, constants.RoleOwner)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/api-keys/"+uuid.New().String(), nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
}
