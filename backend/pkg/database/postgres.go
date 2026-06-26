package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lib/pq"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/config"
)

func NewPostgres(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// RunMigrations reads .up.sql files from migrationsDir, sorts them by name,
// and applies any that haven't been recorded in the schema_migrations table.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	// Ensure the tracking table exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	// Collect only .up.sql files
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Edge case: schema_migrations table is empty but database already has tables.
	// This happens when migrating from a pre-existing (manually seeded) database.
	// In that case, record all migrations as applied without re-executing them.
	var totalApplied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&totalApplied); err != nil {
		return fmt.Errorf("failed to count applied migrations: %w", err)
	}
	if totalApplied == 0 {
		// Check if the database was pre-seeded by looking for the users table
		var tableCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name != 'schema_migrations'`).Scan(&tableCount); err != nil {
			return fmt.Errorf("failed to check existing tables: %w", err)
		}
		if tableCount > 0 {
			log.Printf("🔍 Detected pre-existing database (%d tables). Recording %d migrations as applied without re-executing.", tableCount, len(files))
			for _, f := range files {
				if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`, f.Name()); err != nil {
					return fmt.Errorf("failed to record migration %s: %w", f.Name(), err)
				}
			}
			return nil
		}
	}

	for _, f := range files {
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, f.Name()).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration %s: %w", f.Name(), err)
		}
		if count > 0 {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, f.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", f.Name(), err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", f.Name(), err)
		}

		if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, f.Name()); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", f.Name(), err)
		}

		log.Printf("✅ Applied migration: %s", f.Name())
	}

	return nil
}

func HealthCheck(db *sql.DB) error {
	return db.Ping()
}

func Close(db *sql.DB) error {
	return db.Close()
}

func HandlePgError(err error) error {
	if err == nil {
		return nil
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code.Name() {
		case "unique_violation":
			return apperrors.ErrConflict
		case "foreign_key_violation":
			return apperrors.ErrNotFound
		case "not_null_violation":
			return apperrors.ErrValidationFailed
		}
	}
	return apperrors.ErrInternal
}
