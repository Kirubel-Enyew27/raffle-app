package domain

import (
	"context"
	"database/sql"
)

type TicketRepository interface {
	Create(ctx context.Context, ticket *Ticket) error
	CreateBatch(ctx context.Context, tickets []*Ticket) error
	CreateBatchTx(ctx context.Context, tx *sql.Tx, tickets []*Ticket) error
	CountByRaffleID(ctx context.Context, raffleID string) (int, error)
	FindByRaffleID(ctx context.Context, raffleID string) ([]*Ticket, error)
	FindByWalletTxID(ctx context.Context, walletTxID string) ([]*Ticket, error)
}

type RaffleEntity struct {
	ID           string
	Status       string
	TotalTickets int
	SoldTickets  int
	TicketPrice  float64
}

func (r *RaffleEntity) IsActive() bool {
	return r.Status == "active"
}

func (r *RaffleEntity) HasRemainingTickets() bool {
	return r.SoldTickets < r.TotalTickets
}

type RaffleRepository interface {
	FindByID(ctx context.Context, id string) (*RaffleEntity, error)
	FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*RaffleEntity, error)
	UpdateSoldCount(ctx context.Context, raffleID string, increment int) error
	UpdateSoldCountTx(ctx context.Context, tx *sql.Tx, raffleID string, increment int) error
}

type Wallet struct {
	ID      string
	Balance float64
}

type WalletRepository interface {
	FindByUserID(ctx context.Context, userID string) (*Wallet, error)
	FindByUserIDForUpdate(ctx context.Context, tx *sql.Tx, userID string) (*Wallet, error)
	UpdateBalance(ctx context.Context, walletID string, newBalance float64) error
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error
	CreateTransaction(ctx context.Context, walletTx *WalletTransaction) error
	CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *WalletTransaction) error
	Debit(ctx context.Context, walletID string, amount float64) error
	DebitTx(ctx context.Context, tx *sql.Tx, walletID string, amount float64) error
}
