package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"leaflag/backend/internal/config"
	"leaflag/backend/internal/db"
	"leaflag/backend/internal/repository"
	"leaflag/backend/internal/routes"
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
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.Mode == "data-plane" {
		store := runtimeplane.NewStore()
		syncer := runtimeplane.Syncer{
			Store:    store,
			Source:   runtimeplane.HTTPSource{URL: cfg.ControlPlaneURL, Token: cfg.RuntimeSyncToken},
			Interval: cfg.EdgeSyncInterval,
		}
		go syncer.Run(ctx)
		serve(routes.NewDataPlaneRouter(store), cfg.Port, cfg.Mode)
		return
	}

	gdb, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	userRepo := repository.NewUserRepository(gdb)
	projectRepo := repository.NewProjectRepository(gdb)
	environmentRepo := repository.NewEnvironmentRepository(gdb)
	environmentTemplateRepo := repository.NewEnvironmentTemplateRepository(gdb)
	contextFieldRepo := repository.NewContextFieldRepository(gdb)
	parameterRepo := repository.NewParameterRepository(gdb)
	flagRepo := repository.NewFlagRepository(gdb)
	apiKeyRepo := repository.NewAPIKeyRepository(gdb)
	instanceSettingsRepo := repository.NewInstanceSettingsRepository(gdb)
	accessGroupRepo := repository.NewAccessGroupRepository(gdb)
	authService := auth.NewService(userRepo, cfg.JWTSecret)
	oidcService := oidc.NewConfigurationService(instanceSettingsRepo, cfg.EncryptionKey, cfg.JWTSecret)
	projectService := project.NewService(projectRepo, userRepo, environmentRepo, environmentTemplateRepo, accessGroupRepo)
	accessGroupService := access.NewService(accessGroupRepo, userRepo)
	environmentService := environment.NewService(environmentRepo)
	environmentTemplateService := environment.NewTemplateService(environmentTemplateRepo)
	contextFieldService := contextfield.NewService(contextFieldRepo)
	parameterService := parameter.NewService(parameterRepo)
	flagService := flag.NewService(flagRepo)
	apiKeyService := apikey.NewService(apiKeyRepo)
	maintenanceService := maintenance.NewService(instanceSettingsRepo)

	services := routes.Services{
		Auth:                 authService,
		Projects:             projectService,
		Environments:         environmentService,
		EnvironmentTemplates: environmentTemplateService,
		ContextFields:        contextFieldService,
		Parameters:           parameterService,
		Flags:                flagService,
		APIKeys:              apiKeyService,
		Maintenance:          maintenanceService,
		AccessGroups:         accessGroupService,
		OIDC:                 oidcService,
	}
	controlSource := runtimeplane.NewControlSource(gdb)

	if cfg.Mode == "control-plane" {
		serve(routes.NewControlPlaneRouter(services, controlSource, cfg.RuntimeSyncTokens), cfg.Port, cfg.Mode)
		return
	}

	// all-in-one deliberately uses the same snapshot compiler and in-memory
	// runtime store as a standalone data plane, but sources it in-process for
	// a frictionless POC deployment.
	store := runtimeplane.NewStore()
	syncer := runtimeplane.Syncer{Store: store, Source: controlSource, Interval: cfg.EdgeSyncInterval}
	go syncer.Run(ctx)
	serve(routes.NewAllInOneRouter(services, store), cfg.Port, cfg.Mode)
}

func serve(router interface{ Run(addr ...string) error }, port, mode string) {
	log.Printf("LeaFlag %s listening on :%s", mode, port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
