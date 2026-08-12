package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterHealthRoutes(rg *gin.RouterGroup) {
	rg.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
