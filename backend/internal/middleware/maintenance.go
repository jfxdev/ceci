package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MaintenanceModeReader is deliberately narrow so the middleware does not
// depend on a concrete service or persistence implementation.
type MaintenanceModeReader interface {
	IsMaintenanceEnabled(ctx context.Context) (enabled bool, message string, err error)
}

// RequireControlPlaneWritable blocks management mutations while preserving
// login/refresh/logout and the administrator's maintenance toggle. It is
// registered only on Control Plane API routes; Data Plane workload APIs do
// not use it.
func RequireControlPlaneWritable(reader MaintenanceModeReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isMutation(c.Request.Method) || isMaintenanceExemptPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		enabled, message, err := reader.IsMaintenanceEnabled(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "control plane maintenance status is unavailable", "code": "MAINTENANCE_STATUS_UNAVAILABLE"})
			return
		}
		if enabled {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": message, "code": "MAINTENANCE_MODE"})
			return
		}
		c.Next()
	}
}

func isMutation(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func isMaintenanceExemptPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/auth/login") ||
		strings.HasPrefix(path, "/api/v1/auth/refresh") ||
		strings.HasPrefix(path, "/api/v1/auth/logout") ||
		path == "/api/v1/admin/maintenance"
}
