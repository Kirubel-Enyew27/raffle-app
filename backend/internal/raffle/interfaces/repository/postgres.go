package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/raffle-app/backend/internal/raffle/domain"
	"github.com/raffle-app/backend/pkg/errors"
)

type RaffleRepo struct {
	db *sql.DB
}

func NewRaffleRepo(db *sql.DB) *RaffleRepo {
	return &RaffleRepo{db: db}
}

func (r *RaffleRepo) Create(ctx context.Context, raffle *domain.Raffle) error {
	query := `INSERT INTO raffles (id, title, description, ticket_price, total_tickets, sold_tickets, status, draw_date, creator_id, prize_pool, currency, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := r.db.ExecContext(ctx, query, raffle.ID, raffle.Title, raffle.Description, raffle.TicketPrice, raffle.TotalTickets, raffle.SoldTickets, raffle.Status, raffle.DrawDate, raffle.CreatorID, raffle.PrizePool, raffle.Currency, time.Now(), time.Now())
	return err
}

func (r *RaffleRepo) FindByID(ctx context.Context, id string) (*domain.Raffle, error) {
	query := `SELECT id, title, description, ticket_price, total_tickets, sold_tickets, status, draw_date, creator_id, prize_pool, currency, created_at, updated_at FROM raffles WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	raffle := &domain.Raffle{}
	err := row.Scan(&raffle.ID, &raffle.Title, &raffle.Description, &raffle.TicketPrice, &raffle.TotalTickets, &raffle.SoldTickets, &raffle.Status, &raffle.DrawDate, &raffle.CreatorID, &raffle.PrizePool, &raffle.Currency, &raffle.CreatedAt, &raffle.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return raffle, nil
}

func (r *RaffleRepo) FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*domain.Raffle, error) {
	query := `SELECT id, title, description, ticket_price, total_tickets, sold_tickets, status, draw_date, creator_id, prize_pool, currency, created_at, updated_at FROM raffles WHERE id = $1 FOR UPDATE`
	row := tx.QueryRowContext(ctx, query, id)
	raffle := &domain.Raffle{}
	err := row.Scan(&raffle.ID, &raffle.Title, &raffle.Description, &raffle.TicketPrice, &raffle.TotalTickets, &raffle.SoldTickets, &raffle.Status, &raffle.DrawDate, &raffle.CreatorID, &raffle.PrizePool, &raffle.Currency, &raffle.CreatedAt, &raffle.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return raffle, nil
}

func (r *RaffleRepo) Update(ctx context.Context, raffle *domain.Raffle) error {
	query := `UPDATE raffles SET title=$1, description=$2, ticket_price=$3, total_tickets=$4, sold_tickets=$5, status=$6, draw_date=$7, prize_pool=$8, currency=$9, updated_at=$10 WHERE id=$11`
	_, err := r.db.ExecContext(ctx, query, raffle.Title, raffle.Description, raffle.TicketPrice, raffle.TotalTickets, raffle.SoldTickets, raffle.Status, raffle.DrawDate, raffle.PrizePool, raffle.Currency, time.Now(), raffle.ID)
	return err
}

func (r *RaffleRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM raffles WHERE id = $1`, id)
	return err
}

func (r *RaffleRepo) List(ctx context.Context, limit, offset int) ([]*domain.Raffle, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, title, description, ticket_price, total_tickets, sold_tickets, status, draw_date, creator_id, prize_pool, currency, created_at, updated_at FROM raffles LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var raffles []*domain.Raffle
	for rows.Next() {
		raffle := &domain.Raffle{}
		if err := rows.Scan(&raffle.ID, &raffle.Title, &raffle.Description, &raffle.TicketPrice, &raffle.TotalTickets, &raffle.SoldTickets, &raffle.Status, &raffle.DrawDate, &raffle.CreatorID, &raffle.PrizePool, &raffle.Currency, &raffle.CreatedAt, &raffle.UpdatedAt); err != nil {
			return nil, err
		}
		raffles = append(raffles, raffle)
	}
	return raffles, nil
}

func (r *RaffleRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM raffles`).Scan(&count)
	return count, err
}
