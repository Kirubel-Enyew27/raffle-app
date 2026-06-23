package domain

import (
	"context"

	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
)

type DrawRepository interface {
	Create(ctx context.Context, result *DrawResult) error
	FindByRaffleID(ctx context.Context, raffleID string) (*DrawResult, error)
	ExistsForRaffle(ctx context.Context, raffleID string) (bool, error)
	CommitSeed(ctx context.Context, commitment *DrawCommitment) error
	GetCommitment(ctx context.Context, raffleID string) (*DrawCommitment, error)
}

type RaffleRepository interface {
	FindByID(ctx context.Context, id string) (*ticketdomain.RaffleEntity, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	FindTicketsByRaffleID(ctx context.Context, raffleID string) ([]ticketdomain.Ticket, error)
}

type SeedService interface {
	GenerateSeed() (string, string, error)
	CommitSeed(seed string) (string, error)
	VerifyCommit(commit, seed string) bool
}

type RandomService interface {
	GenerateRandom(serverSeed, clientSeed string, nonce, maxTickets int) int
}
