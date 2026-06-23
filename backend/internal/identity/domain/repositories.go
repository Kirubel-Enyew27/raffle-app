package domain

import "context"

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	SoftDelete(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
