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
	"leaflag/backend/internal/service"
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
	parameterRepo := repository.NewParameterRepository(gdb)
	flagRepo := repository.NewFlagRepository(gdb)
	apiKeyRepo := repository.NewAPIKeyRepository(gdb)
	instanceSettingsRepo := repository.NewInstanceSettingsRepository(gdb)
	accessGroupRepo := repository.NewAccessGroupRepository(gdb)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret)
	oidcService := service.NewOIDCConfigurationService(instanceSettingsRepo, cfg.EncryptionKey, cfg.JWTSecret)
	projectService := service.NewProjectService(projectRepo, userRepo, environmentRepo, environmentTemplateRepo, accessGroupRepo)
	accessGroupService := service.NewAccessGroupService(accessGroupRepo, userRepo)
	environmentService := service.NewEnvironmentService(environmentRepo)
	environmentTemplateService := service.NewEnvironmentTemplateService(environmentTemplateRepo)
	parameterService := service.NewParameterService(parameterRepo)
	flagService := service.NewFlagService(flagRepo)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo)
	maintenanceService := service.NewMaintenanceService(instanceSettingsRepo)

	services := routes.Services{
		Auth:                 authService,
		Projects:             projectService,
		Environments:         environmentService,
		EnvironmentTemplates: environmentTemplateService,
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
