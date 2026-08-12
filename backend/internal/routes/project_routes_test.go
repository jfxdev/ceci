package routes

import (
	"bytes"
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
	"ceci/backend/internal/repository"
	"ceci/backend/internal/service"
)

type fakeProjectService struct {
	createProject *model.Project
	createErr     error

	getProject *model.Project
	getErr     error

	updateProject *model.Project
	updateErr     error

	deleteErr error

	listProjects []model.Project
	listErr      error

	role    constants.ProjectRole
	roleErr error

	members    []repository.MemberWithUser
	membersErr error

	addMemberErr    error
	updateMemberErr error
	removeMemberErr error
}

func (f *fakeProjectService) Create(ctx context.Context, creatorID uuid.UUID, name, slug string) (*model.Project, error) {
	return f.createProject, f.createErr
}
func (f *fakeProjectService) Get(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return f.getProject, f.getErr
}
func (f *fakeProjectService) Update(ctx context.Context, id uuid.UUID, name string) (*model.Project, error) {
	return f.updateProject, f.updateErr
}
func (f *fakeProjectService) Delete(ctx context.Context, id uuid.UUID) error { return f.deleteErr }
func (f *fakeProjectService) ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error) {
	return f.listProjects, f.listErr
}
func (f *fakeProjectService) RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	return f.role, f.roleErr
}
func (f *fakeProjectService) ListMembers(ctx context.Context, projectID uuid.UUID) ([]repository.MemberWithUser, error) {
	return f.members, f.membersErr
}
func (f *fakeProjectService) AddMember(ctx context.Context, projectID uuid.UUID, email string, role constants.ProjectRole) error {
	return f.addMemberErr
}
func (f *fakeProjectService) UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role constants.ProjectRole) error {
	return f.updateMemberErr
}
func (f *fakeProjectService) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	return f.removeMemberErr
}

func newProjectTestRouter(projects projectService, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authFake := &fakeAuthService{parseUserID: userID}
	registerProjectRoutes(&r.RouterGroup, authFake, projects)
	return r
}

func authedRequest(method, path string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer sometoken")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestListProjectsRoute(t *testing.T) {
	userID := uuid.New()
	fake := &fakeProjectService{listProjects: []model.Project{{ID: uuid.New(), Name: "Alpha", Slug: "alpha"}}}
	r := newProjectTestRouter(fake, userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alpha")
}

func TestCreateProjectRoute(t *testing.T) {
	userID := uuid.New()
	fake := &fakeProjectService{createProject: &model.Project{ID: uuid.New(), Name: "Alpha", Slug: "alpha"}}
	r := newProjectTestRouter(fake, userID)

	body, _ := json.Marshal(map[string]string{"name": "Alpha", "slug": "alpha"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects", body))

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "owner")
}

func TestCreateProjectRoute_BadRequest(t *testing.T) {
	r := newProjectTestRouter(&fakeProjectService{}, uuid.New())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects", []byte(`{}`)))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetProjectRoute_NotMember(t *testing.T) {
	fake := &fakeProjectService{roleErr: repository.ErrNotFound}
	r := newProjectTestRouter(fake, uuid.New())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String(), nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetProjectRoute_Success(t *testing.T) {
	projectID := uuid.New()
	fake := &fakeProjectService{
		role:       constants.RoleViewer,
		getProject: &model.Project{ID: projectID, Name: "Alpha", Slug: "alpha"},
	}
	r := newProjectTestRouter(fake, uuid.New())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+projectID.String(), nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "viewer")
}

func TestUpdateProjectRoute_Forbidden(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleEditor}
	r := newProjectTestRouter(fake, uuid.New())

	body, _ := json.Marshal(map[string]string{"name": "New name"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPatch, "/projects/"+uuid.New().String(), body))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateProjectRoute_Success(t *testing.T) {
	projectID := uuid.New()
	fake := &fakeProjectService{
		role:          constants.RoleOwner,
		updateProject: &model.Project{ID: projectID, Name: "New name", Slug: "alpha"},
	}
	r := newProjectTestRouter(fake, uuid.New())

	body, _ := json.Marshal(map[string]string{"name": "New name"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPatch, "/projects/"+projectID.String(), body))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "New name")
}

func TestDeleteProjectRoute_Success(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleOwner}
	r := newProjectTestRouter(fake, uuid.New())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String(), nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestListMembersRoute(t *testing.T) {
	fake := &fakeProjectService{
		role:    constants.RoleViewer,
		members: []repository.MemberWithUser{{UserID: uuid.New(), Email: "a@b.com", Name: "Ana", Role: constants.RoleOwner}},
	}
	r := newProjectTestRouter(fake, uuid.New())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodGet, "/projects/"+uuid.New().String()+"/members", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "a@b.com")
}

func TestAddMemberRoute_UserNotFound(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleAdmin, addMemberErr: service.ErrUserNotFound}
	r := newProjectTestRouter(fake, uuid.New())

	body, _ := json.Marshal(map[string]string{"email": "x@y.com", "role": "viewer"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/members", body))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddMemberRoute_InvalidRole(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleAdmin}
	r := newProjectTestRouter(fake, uuid.New())

	body, _ := json.Marshal(map[string]string{"email": "x@y.com", "role": "superadmin"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/members", body))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddMemberRoute_Success(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleAdmin}
	r := newProjectTestRouter(fake, uuid.New())

	body, _ := json.Marshal(map[string]string{"email": "x@y.com", "role": "viewer"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPost, "/projects/"+uuid.New().String()+"/members", body))

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestUpdateMemberRoute_InvalidUserID(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleAdmin}
	r := newProjectTestRouter(fake, uuid.New())

	body, _ := json.Marshal(map[string]string{"role": "viewer"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPatch, "/projects/"+uuid.New().String()+"/members/not-a-uuid", body))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateMemberRoute_Success(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleAdmin}
	r := newProjectTestRouter(fake, uuid.New())

	body, _ := json.Marshal(map[string]string{"role": "editor"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodPatch, "/projects/"+uuid.New().String()+"/members/"+uuid.New().String(), body))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRemoveMemberRoute_Success(t *testing.T) {
	fake := &fakeProjectService{role: constants.RoleAdmin}
	r := newProjectTestRouter(fake, uuid.New())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(http.MethodDelete, "/projects/"+uuid.New().String()+"/members/"+uuid.New().String(), nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
}
