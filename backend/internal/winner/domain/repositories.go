package domain

import (
	"context"
	"time"
)

type WinnerRepository interface {
	Create(ctx context.Context, winner *Winner) error
	FindAll(ctx context.Context, limit, offset int, paidOnly *bool) ([]Winner, int, error)
	FindByRaffleID(ctx context.Context, raffleID string) ([]Winner, error)
	FindByDrawID(ctx context.Context, drawID string) ([]Winner, error)
	FindByID(ctx context.Context, id string) (*Winner, error)
	MarkPrizePaid(ctx context.Context, id string, paymentDate time.Time, paymentReference string) error
	ExistsByDrawIDAndTicketID(ctx context.Context, drawID, ticketID string) (bool, error)
}

type RaffleRepository interface {
	FindByID(ctx context.Context, id string) (*Raffle, error)
}

type DrawRepository interface {
	FindByID(ctx context.Context, drawID string) (*Draw, error)
	FindByRaffleID(ctx context.Context, raffleID string) (*Draw, error)
	GetProofByRaffleID(ctx context.Context, raffleID string) (*DrawProof, error)
}

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*User, error)
}

type TicketRepository interface {
	FindByID(ctx context.Context, id string) (*Ticket, error)
}
