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

func scanUser(row scannable, user *domain.User) error {
	var email sql.NullString
	var phone sql.NullString
	var banReason sql.NullString
	var fullName sql.NullString
	var avatarURL sql.NullString
	var deletedAt sql.NullTime
	err := row.Scan(&user.ID, &email, &phone, &fullName, &avatarURL, &user.PasswordHash, &user.Role, &user.IsBanned, &banReason, &deletedAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return err
	}
	user.Email = email.String
	user.Phone = phone.String
	user.FullName = fullName.String
	user.AvatarURL = avatarURL.String
	if banReason.Valid {
		user.BanReason = &banReason.String
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}
	return nil
}

// scannable is satisfied by both *sql.Row and *sql.Rows
type scannable interface {
	Scan(dest ...interface{}) error
}

const userColumns = "id, email, phone, full_name, avatar_url, password_hash, role, is_banned, ban_reason, deleted_at, created_at, updated_at"

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE email = $1 AND deleted_at IS NULL`
	user := &domain.User{}
	if err := scanUser(r.db.QueryRowContext(ctx, query, email), user); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, email, phone, full_name, avatar_url, password_hash, role, is_banned, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := r.db.ExecContext(ctx, query, user.ID, user.Email, user.Phone, user.FullName, user.AvatarURL, user.PasswordHash, user.Role, user.IsBanned, time.Now(), time.Now())
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
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1 AND deleted_at IS NULL`
	user := &domain.User{}
	if err := scanUser(r.db.QueryRowContext(ctx, query, id), user); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET email = $1, phone = $2, full_name = $3, avatar_url = $4, role = $5, is_banned = $6, ban_reason = $7, updated_at = $8 WHERE id = $9`
	var banReason sql.NullString
	if user.BanReason != nil {
		banReason.Valid = true
		banReason.String = *user.BanReason
	}
	_, err := r.db.ExecContext(ctx, query, user.Email, user.Phone, user.FullName, user.AvatarURL, user.Role, user.IsBanned, banReason, time.Now(), user.ID)
	return err
}

func (r *UserRepo) FindByName(ctx context.Context, name string) (*domain.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE LOWER(full_name) = LOWER($1) AND deleted_at IS NULL LIMIT 1`
	user := &domain.User{}
	if err := scanUser(r.db.QueryRowContext(ctx, query, name), user); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE phone = $1 AND deleted_at IS NULL`
	user := &domain.User{}
	if err := scanUser(r.db.QueryRowContext(ctx, query, phone), user); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, phone).Scan(&exists)
	return exists, err
}

func (r *UserRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}
