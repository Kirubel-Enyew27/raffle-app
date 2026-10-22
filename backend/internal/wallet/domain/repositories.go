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
	FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int, txType string) ([]*WalletTransaction, error)
	CountTransactionsByWalletID(ctx context.Context, walletID string, txType string) (int64, error)
	FindTransactionsByStatus(ctx context.Context, txType, status string) ([]*WalletTransaction, error)
	FindTransactionByID(ctx context.Context, id string) (*WalletTransaction, error)
	UpdateTransactionStatus(ctx context.Context, id, status string) error
	ExistsByReference(ctx context.Context, reference string) (bool, error)
}

type DepositRepository interface {
	Create(ctx context.Context, deposit *Deposit) error
	CreateTx(ctx context.Context, tx *sql.Tx, deposit *Deposit) error
	FindByID(ctx context.Context, id string) (*Deposit, error)
	FindByIDWithLock(ctx context.Context, tx *sql.Tx, id string) (*Deposit, error)
	FindByExternalReference(ctx context.Context, ref string) (*Deposit, error)
	Update(ctx context.Context, deposit *Deposit) error
	UpdateTx(ctx context.Context, tx *sql.Tx, deposit *Deposit) error
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*Deposit, int64, error)
	BeginTx(ctx context.Context) (*sql.Tx, error)
}

type PaymentVerifier interface {
	Verify(ctx context.Context, req VerificationRequest) (*VerificationResult, error)
}

