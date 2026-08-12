package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OFREPCors allows cross-origin OFREP evaluation calls (e.g. an OpenFeature
// web/browser SDK running on a different origin than this API). Admin API
// routes don't need this — the SPA is served same-origin.
func OFREPCors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
