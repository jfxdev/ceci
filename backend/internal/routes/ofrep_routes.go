package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/middleware"
	"ceci/backend/internal/service"
)

// ofrepFlagService is the subset of FlagService behavior the OFREP routes depend on.
type ofrepFlagService interface {
	Evaluate(ctx context.Context, projectID uuid.UUID, key string, evalCtx map[string]any) service.EvaluationResult
	EvaluateAll(ctx context.Context, projectID uuid.UUID, evalCtx map[string]any) ([]service.EvaluationResult, error)
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

// RegisterOFREPRoutes implements the OpenFeature Remote Evaluation Protocol
// (OFREP): single and bulk flag evaluation, authenticated with a
// project-scoped bearer API key (see middleware.RequireProjectAPIKey).
func RegisterOFREPRoutes(rg *gin.RouterGroup, keyResolver middleware.ProjectKeyResolver, flags *service.FlagService) {
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
		res := flags.Evaluate(c.Request.Context(), projectID, c.Param("key"), req.Context)
		c.JSON(statusForResult(res), toOFREPResponse(res))
	})

	scoped.POST("/evaluate/flags", func(c *gin.Context) {
		var req ofrepEvaluateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errorCode": constants.ErrCodeParseError, "errorDetails": err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		results, err := flags.EvaluateAll(c.Request.Context(), projectID, req.Context)
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

func toOFREPResponse(res service.EvaluationResult) ofrepFlagResponse {
	return ofrepFlagResponse{
		Key:       res.Key,
		Value:     res.Value,
		Reason:    res.Reason,
		Variant:   res.Variant,
		ErrorCode: res.ErrorCode,
	}
}

func statusForResult(res service.EvaluationResult) int {
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
