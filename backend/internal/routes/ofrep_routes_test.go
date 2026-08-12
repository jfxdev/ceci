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

	"ceci/backend/internal/constants"
	"ceci/backend/internal/middleware"
	"ceci/backend/internal/service"
)

type fakeOFREPFlagService struct {
	evalResult service.EvaluationResult
	allResults []service.EvaluationResult
	allErr     error
}

func (f *fakeOFREPFlagService) Evaluate(ctx context.Context, projectID uuid.UUID, key string, evalCtx map[string]any) service.EvaluationResult {
	return f.evalResult
}
func (f *fakeOFREPFlagService) EvaluateAll(ctx context.Context, projectID uuid.UUID, evalCtx map[string]any) ([]service.EvaluationResult, error) {
	return f.allResults, f.allErr
}

type fakeProjectKeyResolverForRoutes struct {
	projectID uuid.UUID
	err       error
}

func (f fakeProjectKeyResolverForRoutes) ResolveProjectID(ctx context.Context, rawKey string) (uuid.UUID, error) {
	return f.projectID, f.err
}

func newOFREPTestRouter(flags ofrepFlagService, resolver middleware.ProjectKeyResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerOFREPRoutes(&r.RouterGroup, resolver, flags)
	return r
}

func ofrepRequest(path string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer ceci_sk_test")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestOFREPSingleEval_MissingAuth(t *testing.T) {
	r := newOFREPTestRouter(&fakeOFREPFlagService{}, fakeProjectKeyResolverForRoutes{})

	req := httptest.NewRequest(http.MethodPost, "/ofrep/v1/evaluate/flags/new-checkout", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOFREPSingleEval_Success(t *testing.T) {
	fake := &fakeOFREPFlagService{evalResult: service.EvaluationResult{Key: "new-checkout", Value: true, Reason: constants.ReasonTargetingMatch, Variant: "on"}}
	r := newOFREPTestRouter(fake, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ofrepRequest("/ofrep/v1/evaluate/flags/new-checkout", []byte(`{"context":{"targetingKey":"user-1"}}`)))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"variant":"on"`)
}

func TestOFREPSingleEval_FlagNotFound(t *testing.T) {
	fake := &fakeOFREPFlagService{evalResult: service.EvaluationResult{Key: "missing", Reason: constants.ReasonError, ErrorCode: constants.ErrCodeFlagNotFound}}
	r := newOFREPTestRouter(fake, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ofrepRequest("/ofrep/v1/evaluate/flags/missing", []byte(`{}`)))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "FLAG_NOT_FOUND")
}

func TestOFREPSingleEval_BadRequest(t *testing.T) {
	r := newOFREPTestRouter(&fakeOFREPFlagService{}, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ofrepRequest("/ofrep/v1/evaluate/flags/f1", []byte(`not-json`)))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOFREPBulkEval_Success(t *testing.T) {
	fake := &fakeOFREPFlagService{allResults: []service.EvaluationResult{
		{Key: "f1", Value: true, Reason: constants.ReasonStatic, Variant: "on"},
		{Key: "f2", Value: false, Reason: constants.ReasonStatic, Variant: "off"},
	}}
	r := newOFREPTestRouter(fake, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ofrepRequest("/ofrep/v1/evaluate/flags", []byte(`{"context":{}}`)))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"f1"`)
	assert.Contains(t, w.Body.String(), `"f2"`)
}

func TestOFREPBulkEval_BadRequest(t *testing.T) {
	r := newOFREPTestRouter(&fakeOFREPFlagService{}, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ofrepRequest("/ofrep/v1/evaluate/flags", []byte(`not-json`)))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
