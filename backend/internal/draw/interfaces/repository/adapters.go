package repository

import (
	"context"

	rafflerepo "github.com/raffle-app/backend/internal/raffle/interfaces/repository"
	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
	ticketrepo "github.com/raffle-app/backend/internal/ticket/interfaces/repository"
)

// DrawRaffleAdapter adapts the raffle + ticket repos to draw's RaffleRepository interface.
type DrawRaffleAdapter struct {
	raffleRepo *rafflerepo.RaffleRepo
	ticketRepo *ticketrepo.TicketRepo
}

func NewDrawRaffleAdapter(r *rafflerepo.RaffleRepo, t *ticketrepo.TicketRepo) *DrawRaffleAdapter {
	return &DrawRaffleAdapter{raffleRepo: r, ticketRepo: t}
}

func (a *DrawRaffleAdapter) FindByID(ctx context.Context, id string) (*ticketdomain.RaffleEntity, error) {
	r, err := a.raffleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ticketdomain.RaffleEntity{
		ID:           r.ID,
		Status:       r.Status,
		TotalTickets: r.TotalTickets,
		SoldTickets:  r.SoldTickets,
		TicketPrice:  r.TicketPrice,
		PrizePool:    r.PrizePool,
	}, nil
}

func (a *DrawRaffleAdapter) UpdateStatus(ctx context.Context, id, status string) error {
	r, err := a.raffleRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	r.Status = status
	return a.raffleRepo.Update(ctx, r)
}

func (a *DrawRaffleAdapter) FindTicketsByRaffleID(ctx context.Context, raffleID string) ([]ticketdomain.Ticket, error) {
	tickets, err := a.ticketRepo.FindByRaffleID(ctx, raffleID)
	if err != nil {
		return nil, err
	}
	result := make([]ticketdomain.Ticket, len(tickets))
	for i, t := range tickets {
		result[i] = *t
	}
	return result, nil
}
