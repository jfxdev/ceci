package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
	"ceci/backend/internal/service"
)

type fakeParameterService struct {
	list    []model.Parameter
	listErr error

	get    *model.Parameter
	getErr error

	upsert    *model.Parameter
	upsertErr error

	deleteErr error

	versions    []model.ParameterVersion
	versionsErr error
}

func (f *fakeParameterService) List(ctx context.Context, projectID uuid.UUID, prefix string) ([]model.Parameter, error) {
	return f.list, f.listErr
}
func (f *fakeParameterService) Get(ctx context.Context, projectID uuid.UUID, key string) (*model.Parameter, error) {
	return f.get, f.getErr
}
func (f *fakeParameterService) Upsert(ctx context.Context, projectID uuid.UUID, key, value string, changedBy uuid.UUID) (*model.Parameter, error) {
	return f.upsert, f.upsertErr
}
func (f *fakeParameterService) Delete(ctx context.Context, projectID uuid.UUID, key string, deletedBy uuid.UUID) error {
	return f.deleteErr
}
func (f *fakeParameterService) ListVersions(ctx context.Context, projectID uuid.UUID, key string) ([]model.ParameterVersion, error) {
	return f.versions, f.versionsErr
}

type fakeRoleResolver struct {
	role constants.ProjectRole
	err  error
}

func (f fakeRoleResolver) RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	return f.role, f.err
}

func newParameterTestRouter(params parameterService, role constants.ProjectRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authFake := &fakeAuthService{parseUserID: uuid.New()}
	resolver := fakeRoleResolver{role: role}
	registerParameterRoutes(&r.RouterGroup, authFake, resolver, params)
	return r
}

func TestListParametersRoute(t *testing.T) {
	fake := &fakeParameterService{list: []model.Parameter{{Key: "service/db/host", Value: "localhost", Version: 1}}}
	r := newParameterTestRouter(fake, constants.RoleViewer)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/parameters", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "service/db/host")
}

func TestGetParameterRoute_HierarchicalKey(t *testing.T) {
	fake := &fakeParameterService{get: &model.Parameter{Key: "service/db/host", Value: "localhost", Version: 1, UpdatedAt: time.Now()}}
	r := newParameterTestRouter(fake, constants.RoleViewer)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/parameters/value/service/db/host", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "localhost")
}

func TestGetParameterRoute_NotFound(t *testing.T) {
	fake := &fakeParameterService{getErr: service.ErrParameterNotFound}
	r := newParameterTestRouter(fake, constants.RoleViewer)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/parameters/value/missing", nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPutParameterRoute_Forbidden(t *testing.T) {
	fake := &fakeParameterService{}
	r := newParameterTestRouter(fake, constants.RoleViewer)

	body, _ := json.Marshal(map[string]string{"value": "localhost"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPut, "/projects/"+uuid.New().String()+"/parameters/value/db/host", body))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPutParameterRoute_Success(t *testing.T) {
	fake := &fakeParameterService{upsert: &model.Parameter{Key: "db/host", Value: "localhost", Version: 1, UpdatedAt: time.Now()}}
	r := newParameterTestRouter(fake, constants.RoleEditor)

	body, _ := json.Marshal(map[string]string{"value": "localhost"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPut, "/projects/"+uuid.New().String()+"/parameters/value/db/host", body))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "localhost")
}

func TestDeleteParameterRoute_Forbidden(t *testing.T) {
	fake := &fakeParameterService{}
	r := newParameterTestRouter(fake, constants.RoleEditor)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/parameters/value/db/host", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteParameterRoute_Success(t *testing.T) {
	fake := &fakeParameterService{}
	r := newParameterTestRouter(fake, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/parameters/value/db/host", nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteParameterRoute_NotFound(t *testing.T) {
	fake := &fakeParameterService{deleteErr: service.ErrParameterNotFound}
	r := newParameterTestRouter(fake, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/parameters/value/db/host", nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListVersionsRoute(t *testing.T) {
	fake := &fakeParameterService{versions: []model.ParameterVersion{{Version: 2, Value: "new", ChangeType: "update", ChangedAt: time.Now()}}}
	r := newParameterTestRouter(fake, constants.RoleViewer)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/parameters/versions/db/host", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "update")
}
