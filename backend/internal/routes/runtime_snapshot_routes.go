package routes

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"leaflag/backend/internal/runtimeplane"
)

type runtimeSnapshotSource interface {
	Load(ctx context.Context, ifNoneMatch string) (snapshot *runtimeplane.Snapshot, etag string, notModified bool, err error)
}

func RegisterRuntimeSnapshotRoutes(rg *gin.RouterGroup, source runtimeSnapshotSource, tokens []string) {
	route := rg.Group("/internal/v1/runtime")
	route.Use(requireRuntimeSyncToken(tokens))
	route.GET("/snapshot", func(c *gin.Context) {
		snapshot, etag, notModified, err := source.Load(c.Request.Context(), c.GetHeader("If-None-Match"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build runtime snapshot"})
			return
		}
		c.Header("ETag", etag)
		c.Header("Cache-Control", "no-store")
		if notModified {
			c.Status(http.StatusNotModified)
			return
		}
		c.JSON(http.StatusOK, snapshot)
	})
}

func requireRuntimeSyncToken(tokens []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if len(header) <= len(prefix) || header[:len(prefix)] != prefix || !matchesSyncToken(header[len(prefix):], tokens) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid runtime sync token"})
			return
		}
		c.Next()
	}
}

func matchesSyncToken(token string, tokens []string) bool {
	matched := 0
	for _, candidate := range tokens {
		if len(token) == len(candidate) {
			matched |= subtle.ConstantTimeCompare([]byte(token), []byte(candidate))
		}
	}
	return matched == 1
}
