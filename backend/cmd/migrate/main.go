package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
)

func main() {
	if err := run(); err != nil {
		log.Printf("Migration failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	// Require an explicitly supplied production environment. Do not allow
	// config.Load() to select production through a local .env file.
	appEnv, ok := os.LookupEnv("APP_ENV")
	if !ok || strings.ToLower(strings.TrimSpace(appEnv)) != "production" {
		return fmt.Errorf("APP_ENV must be explicitly set to production")
	}

	// Require database settings to be supplied by the deployment environment.
	required := []string{
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
	}
	for _, key := range required {
		value, exists := os.LookupEnv(key)
		if !exists || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must be explicitly configured", key)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if cfg.AppEnv != "production" {
		return fmt.Errorf("refusing to migrate outside production")
	}

	migrationsDir := "migrations"
	info, err := os.Stat(migrationsDir)
	if err != nil {
		return fmt.Errorf("access migrations directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", migrationsDir)
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("find migration files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no .up.sql migration files found in %s", migrationsDir)
	}

	log.Printf("Connecting to production database; found %d migration files", len(files))

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, migrationsDir); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}
