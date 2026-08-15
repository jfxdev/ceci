// Command seed creates a default admin user for local development, so
// there's a way to log in on a fresh database without a signup UI.
package main

import (
	"context"
	"flag"
	"log"

	"leaflag/backend/internal/config"
	"leaflag/backend/internal/db"
	"leaflag/backend/internal/repository"
	"leaflag/backend/internal/service"
	"leaflag/backend/internal/model"
)

func main() {
	email := flag.String("email", "admin@leaflag.local", "seed user email")
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

	if user, err := userRepo.FindByEmail(context.Background(), *email); err == nil {
		if err := userRepo.SetAdmin(context.Background(), user.ID, true); err != nil {
			log.Fatalf("failed to grant admin access: %v", err)
		}
		if err := gdb.WithContext(context.Background()).Model(&model.User{}).Where("id = ?", user.ID).Update("is_bootstrap_admin", true).Error; err != nil {
			log.Fatalf("failed to preserve bootstrap admin status: %v", err)
		}
		log.Printf("user %s already exists and has admin access", *email)
		return
	}

	if _, err := authService.Register(context.Background(), *email, *password, *name); err != nil {
		log.Fatalf("failed to create seed user: %v", err)
	}
	user, err := userRepo.FindByEmail(context.Background(), *email)
	if err != nil {
		log.Fatalf("failed to look up seeded user: %v", err)
	}
	if err := userRepo.SetAdmin(context.Background(), user.ID, true); err != nil {
		log.Fatalf("failed to grant admin access: %v", err)
	}
	if err := gdb.WithContext(context.Background()).Model(&model.User{}).Where("id = ?", user.ID).Update("is_bootstrap_admin", true).Error; err != nil {
		log.Fatalf("failed to mark bootstrap admin: %v", err)
	}
	log.Printf("created user %s / %s", *email, *password)
}
