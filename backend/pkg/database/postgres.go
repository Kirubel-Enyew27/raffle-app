package database

import (
	"database/sql"
	"errors"
	"fmt"

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

func RunMigrations(db *sql.DB, migrationsDir string) error {
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
