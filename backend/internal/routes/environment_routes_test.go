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
	"leaflag/backend/internal/service"
)

type fakeEnvironmentService struct {
	list    []model.Environment
	listErr error

	created   *model.Environment
	createErr error

	updated   *model.Environment
	updateErr error

	deleteErr error
}

func (f *fakeEnvironmentService) List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error) {
	return f.list, f.listErr
}
func (f *fakeEnvironmentService) Create(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error) {
	return f.created, f.createErr
}
func (f *fakeEnvironmentService) Update(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error) {
	return f.updated, f.updateErr
}
func (f *fakeEnvironmentService) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	return f.deleteErr
}

func newEnvironmentTestRouter(envs environmentService, role constants.ProjectRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authFake := &fakeAuthService{parseUserID: uuid.New()}
	resolver := fakeRoleResolver{role: role}
	registerEnvironmentRoutes(&r.RouterGroup, authFake, resolver, envs)
	return r
}

func TestListEnvironmentsRoute_Success(t *testing.T) {
	fake := &fakeEnvironmentService{list: []model.Environment{{ID: uuid.New(), Key: "production", Name: "Production"}}}
	r := newEnvironmentTestRouter(fake, constants.RoleViewer)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/environments", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "production")
}

func TestCreateEnvironmentRoute_Forbidden(t *testing.T) {
	r := newEnvironmentTestRouter(&fakeEnvironmentService{}, constants.RoleEditor)

	body, _ := json.Marshal(map[string]string{"key": "staging", "name": "Staging"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/environments", body))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateEnvironmentRoute_Success(t *testing.T) {
	fake := &fakeEnvironmentService{created: &model.Environment{ID: uuid.New(), Key: "staging", Name: "Staging"}}
	r := newEnvironmentTestRouter(fake, constants.RoleAdmin)

	body, _ := json.Marshal(map[string]string{"key": "staging", "name": "Staging"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/environments", body))

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "staging")
}

func TestCreateEnvironmentRoute_KeyTaken(t *testing.T) {
	fake := &fakeEnvironmentService{createErr: service.ErrEnvironmentKeyTaken}
	r := newEnvironmentTestRouter(fake, constants.RoleAdmin)

	body, _ := json.Marshal(map[string]string{"key": "staging", "name": "Staging"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/environments", body))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdateEnvironmentRoute_NotFound(t *testing.T) {
	fake := &fakeEnvironmentService{updateErr: service.ErrEnvironmentNotFound}
	r := newEnvironmentTestRouter(fake, constants.RoleAdmin)

	body, _ := json.Marshal(map[string]string{"name": "Staging 2"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPatch, "/projects/"+uuid.New().String()+"/environments/staging", body))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteEnvironmentRoute_LastEnvironment(t *testing.T) {
	fake := &fakeEnvironmentService{deleteErr: service.ErrLastEnvironment}
	r := newEnvironmentTestRouter(fake, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/environments/staging", nil))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDeleteEnvironmentRoute_DefaultEnvironment(t *testing.T) {
	fake := &fakeEnvironmentService{deleteErr: service.ErrDefaultEnvironment}
	r := newEnvironmentTestRouter(fake, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/environments/all", nil))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDeleteEnvironmentRoute_Success(t *testing.T) {
	r := newEnvironmentTestRouter(&fakeEnvironmentService{}, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/environments/staging", nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
}
