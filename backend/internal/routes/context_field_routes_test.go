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
	"leaflag/backend/internal/service/contextfield"
)

type fakeContextFieldService struct {
	list      []model.ContextField
	created   *model.ContextField
	createErr error
}

func (f *fakeContextFieldService) List(context.Context, uuid.UUID) ([]model.ContextField, error) {
	return f.list, nil
}
func (f *fakeContextFieldService) Create(_ context.Context, _ uuid.UUID, _, _ string, _ []contextfield.ValueInput) (*model.ContextField, error) {
	return f.created, f.createErr
}
func (f *fakeContextFieldService) Update(context.Context, uuid.UUID, string, *string, *[]contextfield.ValueInput) (*model.ContextField, error) {
	return nil, nil
}
func (f *fakeContextFieldService) Delete(context.Context, uuid.UUID, string) error { return nil }

func newContextFieldTestRouter(fields contextFieldService, role constants.ProjectRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerContextFieldRoutes(&r.RouterGroup, &fakeAuthService{parseUserID: uuid.New()}, fakeRoleResolver{role: role}, fields)
	return r
}

func TestListContextFieldsRoute_Success(t *testing.T) {
	r := newContextFieldTestRouter(&fakeContextFieldService{list: []model.ContextField{{ID: uuid.New(), Key: "region", Values: []model.ContextFieldValue{{Value: "br"}}}}}, constants.RoleViewer)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/context-fields", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "region")
	assert.Contains(t, w.Body.String(), "br")
}

func TestCreateContextFieldRoute_RequiresAdmin(t *testing.T) {
	r := newContextFieldTestRouter(&fakeContextFieldService{}, constants.RoleEditor)
	body, _ := json.Marshal(map[string]any{"key": "region", "values": []map[string]string{{"value": "br"}}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/context-fields", body))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateContextFieldRoute_KeyTaken(t *testing.T) {
	r := newContextFieldTestRouter(&fakeContextFieldService{createErr: contextfield.ErrKeyTaken}, constants.RoleAdmin)
	body, _ := json.Marshal(map[string]any{"key": "region", "values": []map[string]string{{"value": "br"}}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/context-fields", body))
	assert.Equal(t, http.StatusConflict, w.Code)
}
