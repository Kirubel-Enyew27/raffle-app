package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/raffle-app/backend/internal/winner/domain"
)

type WinnerRepo struct {
	db *sql.DB
}

func NewWinnerRepo(db *sql.DB) *WinnerRepo {
	return &WinnerRepo{db: db}
}

func (r *WinnerRepo) Create(ctx context.Context, winner *domain.Winner) error {
	query := `INSERT INTO winners (raffle_id, draw_id, ticket_id, user_id, prize_amount, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	err := r.db.QueryRowContext(ctx, query,
		winner.RaffleID, winner.DrawID, winner.TicketID, winner.UserID,
		winner.PrizeAmount, winner.CreatedAt, winner.UpdatedAt,
	).Scan(&winner.ID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("winner already exists for this draw and ticket")
		}
		return err
	}
	return nil
}

func (r *WinnerRepo) ExistsByDrawIDAndTicketID(ctx context.Context, drawID, ticketID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM winners WHERE draw_id = $1 AND ticket_id = $2`
	err := r.db.QueryRowContext(ctx, query, drawID, ticketID).Scan(&count)
	return count > 0, err
}

func (r *WinnerRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]domain.Winner, error) {
	query := `SELECT id, raffle_id, draw_id, ticket_id, user_id, prize_amount, prize_paid, payment_date, payment_reference, created_at, updated_at
	          FROM winners WHERE raffle_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, raffleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var winners []domain.Winner
	for rows.Next() {
		var w domain.Winner
		err := rows.Scan(
			&w.ID, &w.RaffleID, &w.DrawID, &w.TicketID, &w.UserID,
			&w.PrizeAmount, &w.PrizePaid, &w.PaymentDate, &w.PaymentReference,
			&w.CreatedAt, &w.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		winners = append(winners, w)
	}
	return winners, nil
}

func (r *WinnerRepo) FindByDrawID(ctx context.Context, drawID string) ([]domain.Winner, error) {
	query := `SELECT id, raffle_id, draw_id, ticket_id, user_id, prize_amount, prize_paid, payment_date, payment_reference, created_at, updated_at
	          FROM winners WHERE draw_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, drawID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var winners []domain.Winner
	for rows.Next() {
		var w domain.Winner
		err := rows.Scan(
			&w.ID, &w.RaffleID, &w.DrawID, &w.TicketID, &w.UserID,
			&w.PrizeAmount, &w.PrizePaid, &w.PaymentDate, &w.PaymentReference,
			&w.CreatedAt, &w.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		winners = append(winners, w)
	}
	return winners, nil
}

func (r *WinnerRepo) FindByID(ctx context.Context, id string) (*domain.Winner, error) {
	w := &domain.Winner{}
	query := `SELECT id, raffle_id, draw_id, ticket_id, user_id, prize_amount, prize_paid, payment_date, payment_reference, created_at, updated_at
	          FROM winners WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&w.ID, &w.RaffleID, &w.DrawID, &w.TicketID, &w.UserID,
		&w.PrizeAmount, &w.PrizePaid, &w.PaymentDate, &w.PaymentReference,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("winner not found")
	}
	return w, err
}

func (r *WinnerRepo) MarkPrizePaid(ctx context.Context, id string, paymentDate time.Time, paymentReference string) error {
	query := `UPDATE winners SET prize_paid = TRUE, payment_date = $1, payment_reference = $2, updated_at = $3 WHERE id = $4`
	_, err := r.db.ExecContext(ctx, query, paymentDate, paymentReference, time.Now(), id)
	return err
}
