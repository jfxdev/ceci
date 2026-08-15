package routes

import (
	"github.com/gin-gonic/gin"
	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/service"
	"net/http"
)

func RegisterOIDCAdminRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, admins adminAuthorizer, oidc *service.OIDCConfigurationService) {
	g := rg.Group("/admin/oidc")
	g.Use(middleware.RequireAuth(auth), requireAdmin(admins))
	g.GET("", func(c *gin.Context) {
		s, e := oidc.Status(c.Request.Context())
		if e != nil {
			c.JSON(500, dto.ErrorResponse{Error: "failed to load oidc configuration"})
			return
		}
		c.JSON(200, dto.OIDCConfigurationDTO{Enabled: s.Enabled, IssuerURL: s.IssuerURL, ClientID: s.ClientID, RedirectURL: s.RedirectURL, JITEnabled: s.JITEnabled, HasClientSecret: s.HasClientSecret})
	})
	g.PUT("", func(c *gin.Context) {
		var r dto.UpdateOIDCConfigurationRequest
		if e := c.ShouldBindJSON(&r); e != nil {
			c.JSON(400, dto.ErrorResponse{Error: e.Error()})
			return
		}
		s, e := oidc.Update(c.Request.Context(), service.OIDCConfigurationInput{Enabled: r.Enabled, IssuerURL: r.IssuerURL, ClientID: r.ClientID, ClientSecret: r.ClientSecret, RedirectURL: r.RedirectURL, JITEnabled: r.JITEnabled})
		if e != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: e.Error()})
			return
		}
		c.JSON(200, dto.OIDCConfigurationDTO{Enabled: s.Enabled, IssuerURL: s.IssuerURL, ClientID: s.ClientID, RedirectURL: s.RedirectURL, JITEnabled: s.JITEnabled, HasClientSecret: s.HasClientSecret})
	})
}
