package routes

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/runtimeplane"
	"leaflag/backend/internal/service/access"
	"leaflag/backend/internal/service/apikey"
	"leaflag/backend/internal/service/auth"
	"leaflag/backend/internal/service/contextfield"
	"leaflag/backend/internal/service/environment"
	"leaflag/backend/internal/service/flag"
	"leaflag/backend/internal/service/maintenance"
	"leaflag/backend/internal/service/oidc"
	"leaflag/backend/internal/service/parameter"
	"leaflag/backend/internal/service/project"
	"leaflag/backend/internal/web"
)

type Services struct {
	Auth                 *auth.Service
	Projects             *project.Service
	Environments         *environment.Service
	EnvironmentTemplates *environment.TemplateService
	ContextFields        *contextfield.Service
	Parameters           *parameter.Service
	Flags                *flag.Service
	APIKeys              *apikey.Service
	Maintenance          *maintenance.Service
	AccessGroups         *access.Service
	OIDC                 *oidc.ConfigurationService
}

// NewRouter retains the previous all-in-one behavior for callers that have
// not yet selected an explicit LEAFLAG_MODE.
func NewRouter(services Services) *gin.Engine {
	return NewAllInOneRouter(services, nil)
}

func NewControlPlaneRouter(services Services, source runtimeSnapshotSource, syncTokens []string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	RegisterHealthRoutes(&r.RouterGroup)
	registerControlRoutes(r, services)
	RegisterRuntimeSnapshotRoutes(&r.RouterGroup, source, syncTokens)
	web.RegisterSPA(r)
	return r
}

func NewDataPlaneRouter(store *runtimeplane.Store) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	RegisterDataPlaneHealthRoutes(&r.RouterGroup, store)
	RegisterOFREPRoutes(&r.RouterGroup, store, store)
	RegisterRuntimeKVRoutes(&r.RouterGroup, store, store)
	return r
}

func NewAllInOneRouter(services Services, store *runtimeplane.Store) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	RegisterDataPlaneHealthRoutes(&r.RouterGroup, store)
	registerControlRoutes(r, services)
	if store == nil {
		// Compatibility path for direct users of NewRouter. The application
		// entrypoint always supplies a store in all-in-one mode.
		RegisterOFREPRoutes(&r.RouterGroup, services.APIKeys, services.Flags)
		RegisterRuntimeKVRoutes(&r.RouterGroup, services.APIKeys, parameterServiceAdapter{services.Parameters})
	} else {
		RegisterOFREPRoutes(&r.RouterGroup, store, store)
		RegisterRuntimeKVRoutes(&r.RouterGroup, store, store)
	}
	web.RegisterSPA(r)
	return r
}

func registerControlRoutes(r *gin.Engine, services Services) {
	api := r.Group("/api/v1")
	api.Use(middleware.RequireControlPlaneWritable(services.Maintenance))
	RegisterAuthRoutes(api, services.Auth)
	RegisterOIDCRoutes(api, services.Auth, services.AccessGroups, services.OIDC)
	RegisterOIDCAdminRoutes(api, services.Auth, services.Auth, services.OIDC)
	RegisterMaintenanceRoutes(api, services.Auth, services.Auth, services.Maintenance)
	RegisterProjectRoutes(api, services.Auth, services.Projects, services.AccessGroups)
	RegisterEnvironmentRoutes(api, services.Auth, services.Projects, services.Environments)
	RegisterContextFieldRoutes(api, services.Auth, services.Projects, services.ContextFields)
	RegisterAdminSettingsRoutes(api, services.Auth, services.Auth, services.EnvironmentTemplates)
	RegisterAccessGroupRoutes(api, services.Auth, services.Auth, services.AccessGroups)
	RegisterParameterRoutes(api, services.Auth, services.Projects, services.Environments, services.Parameters)
	RegisterFlagRoutes(api, services.Auth, services.Projects, services.Environments, services.Flags)
	RegisterPlaygroundRoutes(api, services.Auth, services.Projects, services.Environments, services.Flags)
	RegisterAPIKeyRoutes(api, services.Auth, services.Projects, services.Environments, services.APIKeys)
}

type parameterServiceAdapter struct{ service *parameter.Service }

func (a parameterServiceAdapter) GetParameter(projectID, environmentID uuid.UUID, key string) (runtimeplane.RuntimeParameter, bool) {
	p, err := a.service.Get(context.Background(), projectID, environmentID, key)
	if err != nil {
		return runtimeplane.RuntimeParameter{}, false
	}
	return runtimeplane.RuntimeParameter{ProjectID: p.ProjectID, EnvironmentID: p.EnvironmentID, Key: p.Key, Value: p.Value, Version: p.Version}, true
}

func (a parameterServiceAdapter) ListParameters(projectID, environmentID uuid.UUID, prefix string) []runtimeplane.RuntimeParameter {
	params, err := a.service.List(context.Background(), projectID, environmentID, prefix)
	if err != nil {
		return nil
	}
	out := make([]runtimeplane.RuntimeParameter, 0, len(params))
	for _, p := range params {
		out = append(out, runtimeplane.RuntimeParameter{ProjectID: p.ProjectID, EnvironmentID: p.EnvironmentID, Key: p.Key, Value: p.Value, Version: p.Version})
	}
	return out
}
