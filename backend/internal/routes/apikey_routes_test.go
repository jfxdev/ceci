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

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
)

type fakeAPIKeyService struct {
	rawKey     string
	createdKey *model.ProjectAPIKey
	createErr  error

	list    []model.ProjectAPIKey
	listErr error

	revokeErr error
}

func (f *fakeAPIKeyService) Create(ctx context.Context, projectID, environmentID uuid.UUID, label string) (string, *model.ProjectAPIKey, error) {
	return f.rawKey, f.createdKey, f.createErr
}
func (f *fakeAPIKeyService) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.ProjectAPIKey, error) {
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
	registerAPIKeyRoutes(&r.RouterGroup, authFake, resolver, fakeEnvResolver{}, keys)
	return r
}

func apiKeysPath(projectID, suffix string) string {
	return "/projects/" + projectID + "/environments/production/api-keys" + suffix
}

func TestListAPIKeysRoute_Forbidden(t *testing.T) {
	r := newAPIKeyTestRouter(&fakeAPIKeyService{}, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, apiKeysPath(uuid.New().String(), ""), nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateAPIKeyRoute_Success(t *testing.T) {
	fake := &fakeAPIKeyService{rawKey: "leaflag_sk_abc123", createdKey: &model.ProjectAPIKey{ID: uuid.New(), Label: "CI", Prefix: "leaflag_sk_ab"}}
	r := newAPIKeyTestRouter(fake, constants.RoleOwner)

	body, _ := json.Marshal(map[string]string{"label": "CI"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, apiKeysPath(uuid.New().String(), ""), body))

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "leaflag_sk_abc123")
}

func TestListAPIKeysRoute_Success(t *testing.T) {
	fake := &fakeAPIKeyService{list: []model.ProjectAPIKey{{ID: uuid.New(), Label: "CI", Prefix: "leaflag_sk_ab"}}}
	r := newAPIKeyTestRouter(fake, constants.RoleOwner)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, apiKeysPath(uuid.New().String(), ""), nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CI")
}

func TestRevokeAPIKeyRoute_InvalidID(t *testing.T) {
	r := newAPIKeyTestRouter(&fakeAPIKeyService{}, constants.RoleOwner)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, apiKeysPath(uuid.New().String(), "/not-a-uuid"), nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRevokeAPIKeyRoute_Success(t *testing.T) {
	r := newAPIKeyTestRouter(&fakeAPIKeyService{}, constants.RoleOwner)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, apiKeysPath(uuid.New().String(), "/"+uuid.New().String()), nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
}
