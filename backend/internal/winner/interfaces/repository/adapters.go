package repository

import (
	"context"

	drawrepo "github.com/raffle-app/backend/internal/draw/interfaces/repository"
	identityrepo "github.com/raffle-app/backend/internal/identity/interfaces/repository"
	rafflerepo "github.com/raffle-app/backend/internal/raffle/interfaces/repository"
	ticketrepo "github.com/raffle-app/backend/internal/ticket/interfaces/repository"
	"github.com/raffle-app/backend/internal/winner/domain"
)

// RaffleAdapter adapts the raffle repo to winner's RaffleRepository.
type RaffleAdapter struct{ repo *rafflerepo.RaffleRepo }

func NewRaffleAdapter(r *rafflerepo.RaffleRepo) *RaffleAdapter { return &RaffleAdapter{r} }

func (a *RaffleAdapter) FindByID(ctx context.Context, id string) (*domain.Raffle, error) {
	r, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &domain.Raffle{ID: r.ID, Title: r.Title, Status: r.Status}, nil
}

// DrawAdapter adapts the draw repo to winner's DrawRepository.
type DrawAdapter struct{ repo *drawrepo.DrawRepo }

func NewDrawAdapter(r *drawrepo.DrawRepo) *DrawAdapter { return &DrawAdapter{r} }

func (a *DrawAdapter) FindByRaffleID(ctx context.Context, raffleID string) (*domain.Draw, error) {
	r, err := a.repo.FindByRaffleID(ctx, raffleID)
	if err != nil {
		return nil, err
	}
	return &domain.Draw{
		ID:            r.ID,
		RaffleID:      r.RaffleID,
		DrawTimestamp: r.DrawTimestamp,
		Status:        r.Status,
		WinningTicket: r.WinningTicketID,
	}, nil
}

func (a *DrawAdapter) GetProofByRaffleID(ctx context.Context, raffleID string) (*domain.DrawProof, error) {
	r, err := a.repo.FindByRaffleID(ctx, raffleID)
	if err != nil {
		return nil, err
	}
	return &domain.DrawProof{
		CommitHash:      r.Proof.CommitHash,
		ServerSeedHash:  r.Proof.CommitHash,
		RevealedSeed:    r.Proof.RevealedSeed,
		CombinedHash:    r.Proof.CombinedHash,
		WinningNumber:   r.Proof.WinningNumber,
		VerificationURL: r.Proof.VerificationURL,
	}, nil
}

// UserAdapter adapts the identity user repo to winner's UserRepository.
type UserAdapter struct{ repo *identityrepo.UserRepo }

func NewUserAdapter(r *identityrepo.UserRepo) *UserAdapter { return &UserAdapter{r} }

func (a *UserAdapter) FindByID(ctx context.Context, id string) (*domain.User, error) {
	u, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &domain.User{ID: u.ID, Email: u.Email}, nil
}

// TicketAdapter adapts the ticket repo to winner's TicketRepository.
type TicketAdapter struct{ repo *ticketrepo.TicketRepo }

func NewTicketAdapter(r *ticketrepo.TicketRepo) *TicketAdapter { return &TicketAdapter{r} }

func (a *TicketAdapter) FindByID(ctx context.Context, id string) (*domain.Ticket, error) {
	t, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &domain.Ticket{ID: t.ID, RaffleID: t.RaffleID, UserID: t.UserID, TicketNumber: t.TicketNumber}, nil
}
