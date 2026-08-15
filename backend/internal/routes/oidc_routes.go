package routes

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"leaflag/backend/internal/service"
)

const oidcTransactionCookie = "leaflag_oidc_tx"

func RegisterOIDCRoutes(rg *gin.RouterGroup, auth *service.AuthService, groups *service.AccessGroupService, oidc *service.OIDCConfigurationService) {
	rg.GET("/auth/oidc/config", func(c *gin.Context) {
		if oidc == nil {
			c.JSON(http.StatusOK, gin.H{"enabled": false})
			return
		}
		status, err := oidc.Status(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load oidc configuration"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"enabled": status.Enabled})
	})
	rg.GET("/auth/oidc/login", func(c *gin.Context) {
		if oidc == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "oidc is not configured"})
			return
		}
		provider, err := oidc.Current(c.Request.Context())
		if err != nil || provider == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "oidc is not configured"})
			return
		}
		authorizationURL, transaction, err := provider.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start oidc login"})
			return
		}
		setOIDCTransactionCookie(c, transaction, 600)
		c.Redirect(http.StatusFound, authorizationURL)
	})
	rg.GET("/auth/oidc/callback", func(c *gin.Context) {
		if oidc == nil {
			c.Status(http.StatusNotFound)
			return
		}
		provider, providerErr := oidc.Current(c.Request.Context())
		if providerErr != nil || provider == nil {
			c.Status(http.StatusNotFound)
			return
		}
		transaction, err := c.Cookie(oidcTransactionCookie)
		if err != nil {
			redirectOIDCResult(c, provider, false)
			return
		}
		identity, err := provider.Complete(c.Request.Context(), c.Query("state"), transaction, c.Query("code"))
		setOIDCTransactionCookie(c, "", -1)
		if err != nil {
			redirectOIDCResult(c, provider, false)
			return
		}
		user, err := auth.LoginOIDC(c.Request.Context(), "oidc", identity.Subject, identity.Email, identity.Name, provider.JITEnabled())
		if err != nil {
			redirectOIDCResult(c, provider, false)
			return
		}
		if groups != nil {
			if err := groups.SyncOIDCGroups(c.Request.Context(), user.ID, identity.Groups); err != nil {
				redirectOIDCResult(c, provider, false)
				return
			}
		}
		_, refreshToken, err := auth.CreateSession(c.Request.Context(), user)
		if err != nil {
			redirectOIDCResult(c, provider, false)
			return
		}
		setRefreshCookie(c, refreshToken)
		redirectOIDCResult(c, provider, true)
	})
}

func setOIDCTransactionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: oidcTransactionCookie, Value: value, MaxAge: maxAge, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: requestIsSecure(c)})
}

func requestIsSecure(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func redirectOIDCResult(c *gin.Context, oidc *service.OIDCService, success bool) {
	target := oidc.FrontendURL()
	if !success {
		target = target + "?sso_error=1"
	}
	c.Redirect(http.StatusFound, target)
}
