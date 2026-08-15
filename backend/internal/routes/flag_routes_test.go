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
	"gorm.io/datatypes"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/service"
)

type fakeFlagService struct {
	list    []model.FeatureFlag
	listErr error

	get    *model.FeatureFlag
	getErr error

	created   *model.FeatureFlag
	createErr error

	updated   *model.FeatureFlag
	updateErr error

	deleteErr error
}

func (f *fakeFlagService) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error) {
	return f.list, f.listErr
}
func (f *fakeFlagService) Get(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error) {
	return f.get, f.getErr
}
func (f *fakeFlagService) Create(ctx context.Context, projectID, environmentID uuid.UUID, key, name, description, flagType, defaultVariant string, enabled bool, variants []service.VariantInput, rules []service.RuleInput, prerequisiteFlagKey, prerequisiteVariant string) (*model.FeatureFlag, error) {
	return f.created, f.createErr
}
func (f *fakeFlagService) Update(ctx context.Context, projectID, environmentID uuid.UUID, key string, in service.UpdateInput) (*model.FeatureFlag, error) {
	return f.updated, f.updateErr
}
func (f *fakeFlagService) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	return f.deleteErr
}

func newFlagTestRouter(flags flagService, role constants.ProjectRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authFake := &fakeAuthService{parseUserID: uuid.New()}
	resolver := fakeRoleResolver{role: role}
	registerFlagRoutes(&r.RouterGroup, authFake, resolver, fakeEnvResolver{}, flags)
	return r
}

func sampleFlag() *model.FeatureFlag {
	return &model.FeatureFlag{
		ID: uuid.New(), Key: "new-checkout", Name: "New checkout", FlagType: "boolean",
		Configs:  []model.FlagEnvironmentConfig{{Enabled: true, DefaultVariant: "off"}},
		Variants: []model.FlagVariant{{Key: "off", Value: datatypes.JSON(`false`)}},
	}
}

func flagsPath(projectID, suffix string) string {
	return "/projects/" + projectID + "/environments/production/flags" + suffix
}

func TestListFlagsRoute(t *testing.T) {
	fake := &fakeFlagService{list: []model.FeatureFlag{*sampleFlag()}}
	r := newFlagTestRouter(fake, constants.RoleViewer)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, flagsPath(uuid.New().String(), ""), nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "new-checkout")
}

func TestCreateFlagRoute_Forbidden(t *testing.T) {
	r := newFlagTestRouter(&fakeFlagService{}, constants.RoleViewer)

	body, _ := json.Marshal(map[string]any{"key": "f1", "flagType": "boolean", "defaultVariant": "off", "variants": []map[string]any{{"key": "off", "value": false}}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, flagsPath(uuid.New().String(), ""), body))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateFlagRoute_Success(t *testing.T) {
	fake := &fakeFlagService{created: sampleFlag()}
	r := newFlagTestRouter(fake, constants.RoleEditor)

	body, _ := json.Marshal(map[string]any{"key": "new-checkout", "flagType": "boolean", "defaultVariant": "off", "variants": []map[string]any{{"key": "off", "value": false}}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, flagsPath(uuid.New().String(), ""), body))

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "new-checkout")
}

func TestCreateFlagRoute_BadRequest(t *testing.T) {
	r := newFlagTestRouter(&fakeFlagService{}, constants.RoleEditor)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, flagsPath(uuid.New().String(), ""), []byte(`{}`)))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetFlagRoute_NotFound(t *testing.T) {
	r := newFlagTestRouter(&fakeFlagService{getErr: service.ErrFlagNotFound}, constants.RoleViewer)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, flagsPath(uuid.New().String(), "/missing"), nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateFlagRoute_KillSwitch(t *testing.T) {
	disabled := sampleFlag()
	disabled.Configs = []model.FlagEnvironmentConfig{{Enabled: false, DefaultVariant: "off"}}
	fake := &fakeFlagService{updated: disabled}
	r := newFlagTestRouter(fake, constants.RoleEditor)

	body, _ := json.Marshal(map[string]any{"enabled": false})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPatch, flagsPath(uuid.New().String(), "/new-checkout"), body))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
}

func TestUpdateFlagRoute_NotFound(t *testing.T) {
	fake := &fakeFlagService{updateErr: service.ErrFlagNotFound}
	r := newFlagTestRouter(fake, constants.RoleEditor)

	body, _ := json.Marshal(map[string]any{"enabled": false})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPatch, flagsPath(uuid.New().String(), "/missing"), body))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteFlagRoute(t *testing.T) {
	r := newFlagTestRouter(&fakeFlagService{}, constants.RoleAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, flagsPath(uuid.New().String(), "/new-checkout"), nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteFlagRoute_Forbidden(t *testing.T) {
	r := newFlagTestRouter(&fakeFlagService{}, constants.RoleEditor)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, flagsPath(uuid.New().String(), "/new-checkout"), nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
}
