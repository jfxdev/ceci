package routes

import (
	"github.com/gin-gonic/gin"

	"ceci/backend/internal/service"
	"ceci/backend/internal/web"
)

type Services struct {
	Auth       *service.AuthService
	Projects   *service.ProjectService
	Parameters *service.ParameterService
	Flags      *service.FlagService
	APIKeys    *service.APIKeyService
}

func NewRouter(services Services) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	api := r.Group("/api/v1")
	RegisterHealthRoutes(&r.RouterGroup)
	RegisterAuthRoutes(api, services.Auth)
	RegisterProjectRoutes(api, services.Auth, services.Projects)
	RegisterParameterRoutes(api, services.Auth, services.Projects, services.Parameters)
	RegisterFlagRoutes(api, services.Auth, services.Projects, services.Flags)
	RegisterAPIKeyRoutes(api, services.Auth, services.Projects, services.APIKeys)
	RegisterOFREPRoutes(&r.RouterGroup, services.APIKeys, services.Flags)

	web.RegisterSPA(r)

	return r
}
