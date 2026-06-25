package domain

import (
	"context"
	"database/sql"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet *Wallet) error
	CreateTx(ctx context.Context, tx *sql.Tx, wallet *Wallet) error
	FindByUserID(ctx context.Context, userID string) (*Wallet, error)
	FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*Wallet, error)
	FindByID(ctx context.Context, id string) (*Wallet, error)
	GetBalance(ctx context.Context, walletID string) (float64, error)
	UpdateBalance(ctx context.Context, walletID string, newBalance float64) error
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error
	Debit(ctx context.Context, walletID string, amount float64) error
	CreateTransaction(ctx context.Context, tx *WalletTransaction) error
	CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *WalletTransaction) error
	FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int) ([]*WalletTransaction, error)
	CountTransactionsByWalletID(ctx context.Context, walletID string) (int64, error)
}
