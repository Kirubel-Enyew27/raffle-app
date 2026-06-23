package repository

import (
	"context"
	"database/sql"
	"time"

	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
)

type TicketRaffleRepo struct {
	db *sql.DB
}

func NewTicketRaffleRepo(db *sql.DB) *TicketRaffleRepo {
	return &TicketRaffleRepo{db: db}
}

func (r *TicketRaffleRepo) FindByID(ctx context.Context, id string) (*ticketdomain.RaffleEntity, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, ticket_price, total_tickets, sold_tickets, status FROM raffles WHERE id = $1`, id)
	raffle := &ticketdomain.RaffleEntity{}
	err := row.Scan(&raffle.ID, &raffle.TicketPrice, &raffle.TotalTickets, &raffle.SoldTickets, &raffle.Status)
	if err != nil {
		return nil, err
	}
	return raffle, nil
}

func (r *TicketRaffleRepo) FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*ticketdomain.RaffleEntity, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, ticket_price, total_tickets, sold_tickets, status FROM raffles WHERE id = $1 FOR UPDATE`, id)
	raffle := &ticketdomain.RaffleEntity{}
	err := row.Scan(&raffle.ID, &raffle.TicketPrice, &raffle.TotalTickets, &raffle.SoldTickets, &raffle.Status)
	if err != nil {
		return nil, err
	}
	return raffle, nil
}

func (r *TicketRaffleRepo) UpdateSoldCount(ctx context.Context, raffleID string, increment int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE raffles SET sold_tickets = sold_tickets + $1, updated_at = $2 WHERE id = $3`, increment, time.Now(), raffleID)
	return err
}

func (r *TicketRaffleRepo) UpdateSoldCountTx(ctx context.Context, tx *sql.Tx, raffleID string, increment int) error {
	_, err := tx.ExecContext(ctx, `UPDATE raffles SET sold_tickets = sold_tickets + $1, updated_at = $2 WHERE id = $3`, increment, time.Now(), raffleID)
	return err
}
