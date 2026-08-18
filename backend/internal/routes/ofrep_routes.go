package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/service/flag"
)

// ofrepFlagService is the subset of FlagService behavior the OFREP routes depend on.
type ofrepFlagService interface {
	Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx map[string]any) flag.EvaluationResult
	EvaluateAll(ctx context.Context, projectID, environmentID uuid.UUID, evalCtx map[string]any) ([]flag.EvaluationResult, error)
	Version(ctx context.Context, projectID uuid.UUID) (string, error)
	Subscribe(projectID uuid.UUID) (<-chan struct{}, func())
}

type ofrepEvaluateRequest struct {
	Context map[string]any `json:"context"`
}

type ofrepFlagResponse struct {
	Key          string `json:"key"`
	Value        any    `json:"value,omitempty"`
	Reason       string `json:"reason"`
	Variant      string `json:"variant,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorDetails string `json:"errorDetails,omitempty"`
}

type ofrepBulkResponse struct {
	Flags []ofrepFlagResponse `json:"flags"`
}

type ofrepPollingCapability struct {
	Enabled bool `json:"enabled"`
}

type ofrepCacheInvalidation struct {
	Polling ofrepPollingCapability `json:"polling"`
}

type ofrepFlagEvaluationCapability struct {
	CacheInvalidation ofrepCacheInvalidation `json:"cacheInvalidation"`
}

type ofrepCapabilities struct {
	FlagEvaluation ofrepFlagEvaluationCapability `json:"flagEvaluation"`
}

type ofrepConfigurationResponse struct {
	Name         string            `json:"name"`
	Capabilities ofrepCapabilities `json:"capabilities"`
}

// RegisterOFREPRoutes implements the OpenFeature Remote Evaluation Protocol
// (OFREP): single and bulk flag evaluation, authenticated with a
// project-scoped bearer API key (see middleware.RequireProjectAPIKey).
func RegisterOFREPRoutes(rg *gin.RouterGroup, keyResolver middleware.ProjectKeyResolver, flags ofrepFlagService) {
	registerOFREPRoutes(rg, keyResolver, flags)
}

func registerOFREPRoutes(rg *gin.RouterGroup, keyResolver middleware.ProjectKeyResolver, flags ofrepFlagService) {
	scoped := rg.Group("/ofrep/v1")
	scoped.Use(middleware.OFREPCors())
	scoped.Use(middleware.RequireProjectAPIKey(keyResolver))

	// CORS preflight requests carry no auth, so they get their own routes —
	// OFREPCors() aborts them with 204 before RequireProjectAPIKey runs.
	scoped.OPTIONS("/evaluate/flags/:key", middleware.OFREPCors())
	scoped.OPTIONS("/evaluate/flags", middleware.OFREPCors())
	scoped.OPTIONS("/configuration", middleware.OFREPCors())
	scoped.OPTIONS("/evaluate/flags/stream", middleware.OFREPCors())

	scoped.GET("/evaluate/flags/stream", func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		changed, cancel := flags.Subscribe(projectID)
		defer cancel()

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		sendFlags := func() bool {
			results, err := flags.EvaluateAll(c.Request.Context(), projectID, environmentID, nil)
			if err != nil {
				return false
			}
			out := make([]ofrepFlagResponse, 0, len(results))
			for _, r := range results {
				out = append(out, toOFREPResponse(r))
			}
			c.SSEvent("flags", ofrepBulkResponse{Flags: out})
			return true
		}

		if !sendFlags() {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Writer.Flush()

		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-changed:
				if !sendFlags() {
					return
				}
				c.Writer.Flush()
			case <-heartbeat.C:
				c.SSEvent("heartbeat", "")
				c.Writer.Flush()
			}
		}
	})

	scoped.GET("/configuration", func(c *gin.Context) {
		c.JSON(http.StatusOK, ofrepConfigurationResponse{
			Name: "LeaFlag",
			Capabilities: ofrepCapabilities{
				FlagEvaluation: ofrepFlagEvaluationCapability{
					CacheInvalidation: ofrepCacheInvalidation{
						Polling: ofrepPollingCapability{Enabled: true},
					},
				},
			},
		})
	})

	scoped.POST("/evaluate/flags/:key", func(c *gin.Context) {
		var req ofrepEvaluateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ofrepFlagResponse{
				Key: c.Param("key"), Reason: constants.ReasonError,
				ErrorCode: constants.ErrCodeParseError, ErrorDetails: err.Error(),
			})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		if notModified(c, flags, projectID) {
			return
		}
		res := flags.Evaluate(c.Request.Context(), projectID, environmentID, c.Param("key"), req.Context)
		c.JSON(statusForResult(res), toOFREPResponse(res))
	})

	scoped.POST("/evaluate/flags", func(c *gin.Context) {
		var req ofrepEvaluateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errorCode": constants.ErrCodeParseError, "errorDetails": err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		if notModified(c, flags, projectID) {
			return
		}
		results, err := flags.EvaluateAll(c.Request.Context(), projectID, environmentID, req.Context)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"errorCode": constants.ErrCodeGeneral})
			return
		}
		out := make([]ofrepFlagResponse, 0, len(results))
		for _, r := range results {
			out = append(out, toOFREPResponse(r))
		}
		c.JSON(http.StatusOK, ofrepBulkResponse{Flags: out})
	})
}

// notModified sets the ETag response header for the project's current flag
// version and, if it matches the request's If-None-Match, short-circuits
// with 304 and returns true so the caller skips evaluation entirely.
func notModified(c *gin.Context, flags ofrepFlagService, projectID uuid.UUID) bool {
	etag, err := flags.Version(c.Request.Context(), projectID)
	if err != nil {
		return false
	}
	c.Header("ETag", etag)
	if inm := c.GetHeader("If-None-Match"); inm != "" && inm == etag {
		c.Status(http.StatusNotModified)
		return true
	}
	return false
}

func toOFREPResponse(res flag.EvaluationResult) ofrepFlagResponse {
	return ofrepFlagResponse{
		Key:       res.Key,
		Value:     res.Value,
		Reason:    res.Reason,
		Variant:   res.Variant,
		ErrorCode: res.ErrorCode,
	}
}

func statusForResult(res flag.EvaluationResult) int {
	switch res.ErrorCode {
	case constants.ErrCodeFlagNotFound:
		return http.StatusNotFound
	case constants.ErrCodeParseError:
		return http.StatusBadRequest
	case "":
		return http.StatusOK
	default:
		return http.StatusInternalServerError
	}
}
