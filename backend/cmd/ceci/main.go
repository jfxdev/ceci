package main

import (
	"log"

	"ceci/backend/internal/config"
	"ceci/backend/internal/db"
	"ceci/backend/internal/repository"
	"ceci/backend/internal/routes"
	"ceci/backend/internal/service"
)

func main() {
	cfg := config.Load()

	gdb, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	userRepo := repository.NewUserRepository(gdb)
	projectRepo := repository.NewProjectRepository(gdb)
	parameterRepo := repository.NewParameterRepository(gdb)
	flagRepo := repository.NewFlagRepository(gdb)
	apiKeyRepo := repository.NewAPIKeyRepository(gdb)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret)
	projectService := service.NewProjectService(projectRepo, userRepo)
	parameterService := service.NewParameterService(parameterRepo)
	flagService := service.NewFlagService(flagRepo)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo)

	router := routes.NewRouter(routes.Services{
		Auth:       authService,
		Projects:   projectService,
		Parameters: parameterService,
		Flags:      flagService,
		APIKeys:    apiKeyService,
	})

	log.Printf("ceci listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
