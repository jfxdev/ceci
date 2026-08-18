package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"leaflag/backend/internal/runtimeplane"
)

func RegisterDataPlaneHealthRoutes(rg *gin.RouterGroup, store *runtimeplane.Store) {
	rg.GET("/healthz", func(c *gin.Context) {
		status := runtimeplane.Status{}
		if store != nil {
			status = store.Status()
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "ready": status.Ready, "lastSync": status.LastSuccess, "lastError": status.LastError})
	})
	rg.GET("/readyz", func(c *gin.Context) {
		if store != nil && store.Status().Ready {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "syncing"})
	})
}
