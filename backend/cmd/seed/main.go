// Command seed creates a default admin user for local development, so
// there's a way to log in on a fresh database without a signup UI.
package main

import (
	"context"
	"flag"
	"log"

	"ceci/backend/internal/config"
	"ceci/backend/internal/db"
	"ceci/backend/internal/repository"
	"ceci/backend/internal/service"
)

func main() {
	email := flag.String("email", "admin@ceci.local", "seed user email")
	password := flag.String("password", "admin123", "seed user password")
	name := flag.String("name", "Admin", "seed user name")
	flag.Parse()

	cfg := config.Load()
	gdb, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	userRepo := repository.NewUserRepository(gdb)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret)

	if _, err := userRepo.FindByEmail(context.Background(), *email); err == nil {
		log.Printf("user %s already exists, nothing to do", *email)
		return
	}

	if _, err := authService.Register(context.Background(), *email, *password, *name); err != nil {
		log.Fatalf("failed to create seed user: %v", err)
	}
	log.Printf("created user %s / %s", *email, *password)
}
