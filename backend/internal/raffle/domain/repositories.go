package domain

import (
	"context"
	"database/sql"
)

type RaffleRepository interface {
	Create(ctx context.Context, raffle *Raffle) error
	FindByID(ctx context.Context, id string) (*Raffle, error)
	FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*Raffle, error)
	Update(ctx context.Context, raffle *Raffle) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*Raffle, error)
	Count(ctx context.Context) (int64, error)
}
