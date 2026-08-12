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

	"ceci/backend/internal/model"
)

type fakeAuthService struct {
	loginAccessToken  string
	loginRefreshToken string
	loginUser         *model.User
	loginErr          error

	refreshAccessToken string
	refreshNewToken     string
	refreshErr          error

	logoutErr error

	meUser *model.User
	meErr  error

	parseUserID uuid.UUID
	parseErr    error
}

func (f *fakeAuthService) Login(ctx context.Context, email, password string) (string, string, *model.User, error) {
	return f.loginAccessToken, f.loginRefreshToken, f.loginUser, f.loginErr
}

func (f *fakeAuthService) Refresh(ctx context.Context, rawRefresh string) (string, string, error) {
	return f.refreshAccessToken, f.refreshNewToken, f.refreshErr
}

func (f *fakeAuthService) Logout(ctx context.Context, rawRefresh string) error {
	return f.logoutErr
}

func (f *fakeAuthService) Me(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return f.meUser, f.meErr
}

func (f *fakeAuthService) ParseAccessToken(tokenStr string) (uuid.UUID, error) {
	return f.parseUserID, f.parseErr
}

func newTestRouter(auth authService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerAuthRoutes(&r.RouterGroup, auth)
	return r
}

func TestLoginRoute_Success(t *testing.T) {
	userID := uuid.New()
	fake := &fakeAuthService{
		loginAccessToken:  "access-token",
		loginRefreshToken: "refresh-token",
		loginUser:         &model.User{ID: userID, Email: "a@b.com", Name: "Ana"},
	}
	r := newTestRouter(fake)

	body, _ := json.Marshal(map[string]string{"email": "a@b.com", "password": "secret"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "access-token")
	assert.Contains(t, w.Header().Get("Set-Cookie"), "ceci_refresh=refresh-token")
}

func TestLoginRoute_InvalidCredentials(t *testing.T) {
	fake := &fakeAuthService{loginErr: assert.AnError}
	r := newTestRouter(fake)

	body, _ := json.Marshal(map[string]string{"email": "a@b.com", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLoginRoute_BadRequest(t *testing.T) {
	fake := &fakeAuthService{}
	r := newTestRouter(fake)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMeRoute_Unauthorized(t *testing.T) {
	fake := &fakeAuthService{parseErr: assert.AnError}
	r := newTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMeRoute_Success(t *testing.T) {
	userID := uuid.New()
	fake := &fakeAuthService{
		parseUserID: userID,
		meUser:      &model.User{ID: userID, Email: "a@b.com", Name: "Ana"},
	}
	r := newTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "a@b.com")
}

func TestRefreshRoute_MissingCookie(t *testing.T) {
	fake := &fakeAuthService{}
	r := newTestRouter(fake)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogoutRoute(t *testing.T) {
	fake := &fakeAuthService{}
	r := newTestRouter(fake)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
