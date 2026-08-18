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
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/service/flag"
)

type fakeOFREPFlagService struct {
	evalResult flag.EvaluationResult
	allResults []flag.EvaluationResult
	allErr     error
	version    string
	versionErr error
}

func (f *fakeOFREPFlagService) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx map[string]any) flag.EvaluationResult {
	return f.evalResult
}
func (f *fakeOFREPFlagService) EvaluateAll(ctx context.Context, projectID, environmentID uuid.UUID, evalCtx map[string]any) ([]flag.EvaluationResult, error) {
	return f.allResults, f.allErr
}
func (f *fakeOFREPFlagService) Version(ctx context.Context, projectID uuid.UUID) (string, error) {
	return f.version, f.versionErr
}
func (f *fakeOFREPFlagService) Subscribe(projectID uuid.UUID) (<-chan struct{}, func()) {
	ch := make(chan struct{})
	return ch, func() {}
}

type fakeProjectKeyResolverForRoutes struct {
	projectID     uuid.UUID
	environmentID uuid.UUID
	err           error
}

func (f fakeProjectKeyResolverForRoutes) ResolveEnvironment(ctx context.Context, rawKey string) (uuid.UUID, uuid.UUID, error) {
	return f.projectID, f.environmentID, f.err
}

func newOFREPTestRouter(flags ofrepFlagService, resolver middleware.ProjectKeyResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerOFREPRoutes(&r.RouterGroup, resolver, flags)
	return r
}

func ofrepRequest(path string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer leaflag_sk_test")
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
	fake := &fakeOFREPFlagService{evalResult: flag.EvaluationResult{Key: "new-checkout", Value: true, Reason: constants.ReasonTargetingMatch, Variant: "on"}}
	r := newOFREPTestRouter(fake, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ofrepRequest("/ofrep/v1/evaluate/flags/new-checkout", []byte(`{"context":{"targetingKey":"user-1"}}`)))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"variant":"on"`)
}

func TestOFREPSingleEval_FlagNotFound(t *testing.T) {
	fake := &fakeOFREPFlagService{evalResult: flag.EvaluationResult{Key: "missing", Reason: constants.ReasonError, ErrorCode: constants.ErrCodeFlagNotFound}}
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
	fake := &fakeOFREPFlagService{allResults: []flag.EvaluationResult{
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

func TestOFREPBulkEval_ETagAndNotModified(t *testing.T) {
	fake := &fakeOFREPFlagService{
		allResults: []flag.EvaluationResult{{Key: "f1", Value: true, Reason: constants.ReasonStatic, Variant: "on"}},
		version:    `"1-100"`,
	}
	r := newOFREPTestRouter(fake, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, ofrepRequest("/ofrep/v1/evaluate/flags", []byte(`{}`)))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `"1-100"`, w.Header().Get("ETag"))

	req := ofrepRequest("/ofrep/v1/evaluate/flags", []byte(`{}`))
	req.Header.Set("If-None-Match", `"1-100"`)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusNotModified, w2.Code)
	assert.Empty(t, w2.Body.String())
}

func TestOFREPConfiguration(t *testing.T) {
	r := newOFREPTestRouter(&fakeOFREPFlagService{}, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	req := httptest.NewRequest(http.MethodGet, "/ofrep/v1/configuration", nil)
	req.Header.Set("Authorization", "Bearer leaflag_sk_test")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"LeaFlag"`)
	assert.Contains(t, w.Body.String(), `"polling":{"enabled":true}`)
}

func TestOFREPStream_SendsCurrentFlagsThenClosesOnClientDisconnect(t *testing.T) {
	fake := &fakeOFREPFlagService{
		allResults: []flag.EvaluationResult{{Key: "f1", Value: true, Reason: constants.ReasonStatic, Variant: "on"}},
	}
	r := newOFREPTestRouter(fake, fakeProjectKeyResolverForRoutes{projectID: uuid.New()})

	reqCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/ofrep/v1/evaluate/flags/stream", nil).WithContext(reqCtx)
	req.Header.Set("Authorization", "Bearer leaflag_sk_test")
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		r.ServeHTTP(w, req)
		close(done)
	}()

	// The handler always emits the initial event before entering its select
	// loop. Cancel without concurrently inspecting ResponseRecorder: it is
	// intentionally not safe for a reader while the handler is still writing.
	cancel()
	<-done

	assert.Contains(t, w.Body.String(), "event:flags")
	assert.Contains(t, w.Body.String(), `"f1"`)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
}
