package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	"github.com/raffle-app/backend/pkg/errors"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, role, is_banned, ban_reason, deleted_at, created_at, updated_at FROM users WHERE email = $1 AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, email)
	user := &domain.User{}
	var banReason sql.NullString
	var deletedAt sql.NullTime
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.IsBanned, &banReason, &deletedAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	if banReason.Valid {
		user.BanReason = &banReason.String
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}
	return user, nil
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, email, password_hash, role, is_banned, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.ExecContext(ctx, query, user.ID, user.Email, user.PasswordHash, user.Role, user.IsBanned, time.Now(), time.Now())
	return err
}

func (r *UserRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, passwordHash, time.Now(), userID)
	return err
}

func (r *UserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, email).Scan(&exists)
	return exists, err
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, role, is_banned, ban_reason, deleted_at, created_at, updated_at FROM users WHERE id = $1 AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, id)
	user := &domain.User{}
	var banReason sql.NullString
	var deletedAt sql.NullTime
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.IsBanned, &banReason, &deletedAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	if banReason.Valid {
		user.BanReason = &banReason.String
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}
	return user, nil
}

func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET email = $1, role = $2, is_banned = $3, ban_reason = $4, updated_at = $5 WHERE id = $6`
	var banReason sql.NullString
	if user.BanReason != nil {
		banReason.Valid = true
		banReason.String = *user.BanReason
	}
	_, err := r.db.ExecContext(ctx, query, user.Email, user.Role, user.IsBanned, banReason, time.Now(), user.ID)
	return err
}

func (r *UserRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}
