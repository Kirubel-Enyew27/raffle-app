package repository

import (
	"context"
	"database/sql"

	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
)

type TicketRepo struct {
	db *sql.DB
}

func NewTicketRepo(db *sql.DB) *TicketRepo {
	return &TicketRepo{db: db}
}

func (r *TicketRepo) Create(ctx context.Context, ticket *ticketdomain.Ticket) error {
	query := `INSERT INTO tickets (raffle_id, user_id, wallet_tx_id, ticket_number, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, query,
		ticket.RaffleID, ticket.UserID, ticket.WalletTxID,
		ticket.TicketNumber,
	).Scan(&ticket.ID, &ticket.CreatedAt, &ticket.UpdatedAt)
	return err
}

func (r *TicketRepo) CreateBatchTx(ctx context.Context, tx *sql.Tx, tickets []*ticketdomain.Ticket) error {
	if len(tickets) == 0 {
		return nil
	}

	for i := range tickets {
		var nextNumber int
		err := tx.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(ticket_number), 0) + 1 FROM tickets WHERE raffle_id = $1",
			tickets[i].RaffleID,
		).Scan(&nextNumber)
		if err != nil {
			return err
		}
		tickets[i].TicketNumber = nextNumber

		query := `INSERT INTO tickets (raffle_id, user_id, wallet_tx_id, ticket_number, created_at, updated_at)
		          VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id, created_at, updated_at`
		err = tx.QueryRowContext(ctx, query,
			tickets[i].RaffleID, tickets[i].UserID, tickets[i].WalletTxID,
			tickets[i].TicketNumber,
		).Scan(&tickets[i].ID, &tickets[i].CreatedAt, &tickets[i].UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *TicketRepo) CountByRaffleID(ctx context.Context, raffleID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM tickets WHERE raffle_id = $1`
	err := r.db.QueryRowContext(ctx, query, raffleID).Scan(&count)
	return count, err
}

func (r *TicketRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]*ticketdomain.Ticket, error) {
	query := `SELECT id, raffle_id, user_id, wallet_tx_id, ticket_number, created_at, updated_at
	          FROM tickets WHERE raffle_id = $1 ORDER BY ticket_number`
	rows, err := r.db.QueryContext(ctx, query, raffleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]*ticketdomain.Ticket, 0)
	for rows.Next() {
		ticket := &ticketdomain.Ticket{}
		err := rows.Scan(
			&ticket.ID, &ticket.RaffleID, &ticket.UserID, &ticket.WalletTxID,
			&ticket.TicketNumber, &ticket.CreatedAt, &ticket.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, nil
}

func (r *TicketRepo) FindByWalletTxID(ctx context.Context, walletTxID string) ([]*ticketdomain.Ticket, error) {
	query := `SELECT id, raffle_id, user_id, wallet_tx_id, ticket_number, created_at, updated_at
	          FROM tickets WHERE wallet_tx_id = $1`
	rows, err := r.db.QueryContext(ctx, query, walletTxID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]*ticketdomain.Ticket, 0)
	for rows.Next() {
		ticket := &ticketdomain.Ticket{}
		err := rows.Scan(
			&ticket.ID, &ticket.RaffleID, &ticket.UserID, &ticket.WalletTxID,
			&ticket.TicketNumber, &ticket.CreatedAt, &ticket.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, nil
}


func (r *TicketRepo) FindByID(ctx context.Context, id string) (*ticketdomain.Ticket, error) {
	ticket := &ticketdomain.Ticket{}
	query := `SELECT id, raffle_id, user_id, wallet_tx_id, ticket_number, created_at, updated_at FROM tickets WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&ticket.ID, &ticket.RaffleID, &ticket.UserID, &ticket.WalletTxID,
		&ticket.TicketNumber, &ticket.CreatedAt, &ticket.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	return ticket, err
}

func (r *TicketRepo) CreateBatch(ctx context.Context, tickets []*ticketdomain.Ticket) error {
	for _, t := range tickets {
		if err := r.Create(ctx, t); err != nil {
			return err
		}
	}
	return nil
}
