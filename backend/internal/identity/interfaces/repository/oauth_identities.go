package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	dbpkg "github.com/raffle-app/backend/pkg/database"
	apperrors "github.com/raffle-app/backend/pkg/errors"
)

// OAuthIdentityRepo is the PostgreSQL implementation of
// domain.OAuthIdentityRepository.
type OAuthIdentityRepo struct {
	db *sql.DB
}

func NewOAuthIdentityRepo(db *sql.DB) *OAuthIdentityRepo {
	return &OAuthIdentityRepo{db: db}
}

func (r *OAuthIdentityRepo) FindByProviderUserID(ctx context.Context, provider, providerUserID string) (*domain.OAuthIdentity, error) {
	const query = `SELECT id, user_id, provider, provider_user_id, COALESCE(provider_email, ''), created_at, updated_at
		FROM oauth_identities WHERE provider = $1 AND provider_user_id = $2`

	identity := &domain.OAuthIdentity{}
	err := r.db.QueryRowContext(ctx, query, provider, providerUserID).Scan(
		&identity.ID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderUserID,
		&identity.ProviderEmail,
		&identity.CreatedAt,
		&identity.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return identity, nil
}

func (r *OAuthIdentityRepo) FindByUserID(ctx context.Context, provider, userID string) (*domain.OAuthIdentity, error) {
	const query = `SELECT id, user_id, provider, provider_user_id, COALESCE(provider_email, ''), created_at, updated_at
		FROM oauth_identities WHERE provider = $1 AND user_id = $2`

	identity := &domain.OAuthIdentity{}
	err := r.db.QueryRowContext(ctx, query, provider, userID).Scan(
		&identity.ID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderUserID,
		&identity.ProviderEmail,
		&identity.CreatedAt,
		&identity.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return identity, nil
}

// Create attaches a provider identity to an existing account. The unique
// constraints make the two dangerous outcomes inexpressible: one Google
// identity per local account, and one account per Google subject.
func (r *OAuthIdentityRepo) Create(ctx context.Context, identity *domain.OAuthIdentity) error {
	const query = `INSERT INTO oauth_identities (id, user_id, provider, provider_user_id, provider_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		identity.ID,
		identity.UserID,
		identity.Provider,
		identity.ProviderUserID,
		nullIfEmpty(identity.ProviderEmail),
		now,
		now,
	)
	if err != nil {
		return dbpkg.HandlePgError(err)
	}
	identity.CreatedAt = now
	identity.UpdatedAt = now
	return nil
}

// CreateUserWithIdentity creates the account and its provider link in a single
// transaction. A unique violation (email taken, or the same Google subject
// raced) rolls both back and is reported as a conflict.
func (r *OAuthIdentityRepo) CreateUserWithIdentity(ctx context.Context, user *domain.User, identity *domain.OAuthIdentity) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	if user.ID == "" {
		return apperrors.ErrValidationFailed
	}

	const insertUser = `INSERT INTO users (id, email, phone, full_name, avatar_url, password_hash, role, is_banned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NULL, $6, $7, $8, $9)`
	if _, err := tx.ExecContext(ctx, insertUser,
		user.ID,
		user.Email,
		nullIfEmpty(user.Phone),
		nullIfEmpty(user.FullName),
		nullIfEmpty(user.AvatarURL),
		user.Role,
		user.IsBanned,
		now,
		now,
	); err != nil {
		return dbpkg.HandlePgError(err)
	}

	const insertIdentity = `INSERT INTO oauth_identities (id, user_id, provider, provider_user_id, provider_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := tx.ExecContext(ctx, insertIdentity,
		identity.ID,
		user.ID,
		identity.Provider,
		identity.ProviderUserID,
		nullIfEmpty(identity.ProviderEmail),
		now,
		now,
	); err != nil {
		return dbpkg.HandlePgError(err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	user.CreatedAt = now
	user.UpdatedAt = now
	return nil
}
