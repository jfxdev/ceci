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

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
	"leaflag/backend/internal/service/access"
)

type fakeAccessGroupRouteService struct {
	items           []model.AccessGroup
	listErr         error
	createItem      *model.AccessGroup
	createErr       error
	deleteErr       error
	members         []repository.MemberWithUser
	membersErr      error
	addMemberErr    error
	removeMemberErr error
}

func (f *fakeAccessGroupRouteService) List(context.Context) ([]model.AccessGroup, error) {
	return f.items, f.listErr
}
func (f *fakeAccessGroupRouteService) Create(context.Context, string, string) (*model.AccessGroup, error) {
	return f.createItem, f.createErr
}
func (f *fakeAccessGroupRouteService) Delete(context.Context, uuid.UUID) error { return f.deleteErr }
func (f *fakeAccessGroupRouteService) ListMembers(context.Context, uuid.UUID) ([]repository.MemberWithUser, error) {
	return f.members, f.membersErr
}
func (f *fakeAccessGroupRouteService) AddMember(context.Context, uuid.UUID, string) error {
	return f.addMemberErr
}
func (f *fakeAccessGroupRouteService) RemoveMember(context.Context, uuid.UUID, uuid.UUID) error {
	return f.removeMemberErr
}
func (f *fakeAccessGroupRouteService) ListOIDCMappings(context.Context, uuid.UUID) ([]model.OIDCAccessGroupMapping, error) {
	return nil, nil
}
func (f *fakeAccessGroupRouteService) AddOIDCMapping(context.Context, uuid.UUID, string) error {
	return nil
}
func (f *fakeAccessGroupRouteService) RemoveOIDCMapping(context.Context, uuid.UUID, string) error {
	return nil
}

type fakeGroupAdminAuthorizer struct {
	isAdmin bool
	err     error
}

func (f fakeGroupAdminAuthorizer) IsAdmin(context.Context, uuid.UUID) (bool, error) {
	return f.isAdmin, f.err
}

func newAccessGroupRouteTestRouter(groups accessGroupService, isAdmin bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAccessGroupRoutes(&r.RouterGroup, &fakeAuthService{parseUserID: uuid.New()}, fakeGroupAdminAuthorizer{isAdmin: isAdmin}, groups)
	return r
}

func TestAccessGroupRoutes_ListAllowsAuthenticatedProjectAdmin(t *testing.T) {
	r := newAccessGroupRouteTestRouter(&fakeAccessGroupRouteService{items: []model.AccessGroup{{ID: uuid.New(), Name: "Payments editors"}}}, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/admin/access-groups", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Payments editors")
}

func TestAccessGroupRoutes_CreateRequiresInstanceAdmin(t *testing.T) {
	r := newAccessGroupRouteTestRouter(&fakeAccessGroupRouteService{}, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/admin/access-groups", []byte(`{"name":"Payments"}`)))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAccessGroupRoutes_CreateAndMemberConflict(t *testing.T) {
	groupID := uuid.New()
	r := newAccessGroupRouteTestRouter(&fakeAccessGroupRouteService{createItem: &model.AccessGroup{ID: groupID, Name: "Payments"}, addMemberErr: access.ErrAlreadyMember}, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/admin/access-groups", []byte(`{"name":"Payments","description":"Team"}`)))
	require.Equal(t, http.StatusCreated, w.Code)
	var response map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, groupID.String(), response["id"])

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/admin/access-groups/"+groupID.String()+"/members", []byte(`{"email":"ana@example.com"}`)))
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAccessGroupRoutes_ValidateIDsAndNotFound(t *testing.T) {
	r := newAccessGroupRouteTestRouter(&fakeAccessGroupRouteService{deleteErr: access.ErrNotFound}, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/admin/access-groups/not-a-uuid", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/admin/access-groups/"+uuid.New().String(), nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
