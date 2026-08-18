package routes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/service/flag"
)

type fakePlaygroundFlagService struct {
	allResults []flag.EvaluationResult
	allErr     error
	gotCtx     map[string]any
}

func (f *fakePlaygroundFlagService) EvaluateAll(ctx context.Context, projectID, environmentID uuid.UUID, evalCtx map[string]any) ([]flag.EvaluationResult, error) {
	f.gotCtx = evalCtx
	return f.allResults, f.allErr
}

func newPlaygroundTestRouter(flags playgroundFlagService, role constants.ProjectRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authFake := &fakeAuthService{parseUserID: uuid.New()}
	resolver := fakeRoleResolver{role: role}
	registerPlaygroundRoutes(&r.RouterGroup, authFake, resolver, fakeEnvResolver{}, flags)
	return r
}

func TestPlaygroundEvaluate_Success(t *testing.T) {
	fake := &fakePlaygroundFlagService{
		allResults: []flag.EvaluationResult{
			{Key: "flag-a", Value: true, Reason: constants.ReasonTargetingMatch, Variant: "on"},
		},
	}
	r := newPlaygroundTestRouter(fake, constants.RoleViewer)

	body := []byte(`{"context":{"empresa":"inter","environment":"prd","user_group":"xyz"}}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+uuid.NewString()+"/environments/production/playground/evaluate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "inter", fake.gotCtx["empresa"])
	assert.Contains(t, w.Body.String(), "flag-a")
}

func TestPlaygroundEvaluate_RequiresViewerRole(t *testing.T) {
	fake := &fakePlaygroundFlagService{}
	r := newPlaygroundTestRouter(fake, constants.ProjectRole(""))

	body := []byte(`{"context":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+uuid.NewString()+"/environments/production/playground/evaluate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPlaygroundEvaluate_ServiceError(t *testing.T) {
	fake := &fakePlaygroundFlagService{allErr: assert.AnError}
	r := newPlaygroundTestRouter(fake, constants.RoleViewer)

	body := []byte(`{"context":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+uuid.NewString()+"/environments/production/playground/evaluate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
