package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/raffle-app/backend/internal/wallet/domain"
	"github.com/raffle-app/backend/pkg/errors"
)

type WalletRepo struct {
	db *sql.DB
}

func NewWalletRepo(db *sql.DB) *WalletRepo {
	return &WalletRepo{db: db}
}

func (r *WalletRepo) Create(ctx context.Context, wallet *domain.Wallet) error {
	query := `INSERT INTO wallets (id, user_id, balance, currency, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, wallet.ID, wallet.UserID, wallet.Balance, wallet.Currency, time.Now(), time.Now())
	return err
}

func (r *WalletRepo) CreateTx(ctx context.Context, tx *sql.Tx, wallet *domain.Wallet) error {
	query := `INSERT INTO wallets (id, user_id, balance, currency, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := tx.ExecContext(ctx, query, wallet.ID, wallet.UserID, wallet.Balance, wallet.Currency, time.Now(), time.Now())
	return err
}

func (r *WalletRepo) FindByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	query := `SELECT id, user_id, balance, currency, created_at, updated_at FROM wallets WHERE user_id = $1`
	row := r.db.QueryRowContext(ctx, query, userID)
	wallet := &domain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return wallet, nil
}

func (r *WalletRepo) FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*domain.Wallet, error) {
	query := `SELECT id, user_id, balance, currency, created_at, updated_at FROM wallets WHERE user_id = $1 FOR UPDATE`
	row := tx.QueryRowContext(ctx, query, userID)
	wallet := &domain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return wallet, nil
}

func (r *WalletRepo) FindByID(ctx context.Context, id string) (*domain.Wallet, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, user_id, balance, currency, created_at, updated_at FROM wallets WHERE id = $1`, id)
	wallet := &domain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	return wallet, nil
}

func (r *WalletRepo) GetBalance(ctx context.Context, walletID string) (float64, error) {
	var balance float64
	err := r.db.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE id = $1`, walletID).Scan(&balance)
	return balance, err
}

func (r *WalletRepo) UpdateBalance(ctx context.Context, walletID string, newBalance float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wallets SET balance = $1, updated_at = $2 WHERE id = $3`, newBalance, time.Now(), walletID)
	return err
}

func (r *WalletRepo) Debit(ctx context.Context, walletID string, amount float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1, updated_at = $2 WHERE id = $3`, amount, time.Now(), walletID)
	return err
}

func (r *WalletRepo) CreateTransaction(ctx context.Context, tx *domain.WalletTransaction) error {
	query := `INSERT INTO wallet_transactions (id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := r.db.ExecContext(ctx, query, tx.ID, tx.WalletID, tx.UserID, tx.Type, tx.Status, tx.Amount, tx.BalanceBefore, tx.BalanceAfter, tx.Reference, tx.Description, tx.Metadata, time.Now(), time.Now())
	return err
}

func (r *WalletRepo) FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int) ([]*domain.WalletTransaction, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at FROM wallet_transactions WHERE wallet_id = $1 LIMIT $2 OFFSET $3`, walletID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []*domain.WalletTransaction
	for rows.Next() {
		tx := &domain.WalletTransaction{}
		if err := rows.Scan(&tx.ID, &tx.WalletID, &tx.UserID, &tx.Type, &tx.Status, &tx.Amount, &tx.BalanceBefore, &tx.BalanceAfter, &tx.Reference, &tx.Description, &tx.Metadata, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func (r *WalletRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	_, err := tx.ExecContext(ctx, `UPDATE wallets SET balance = $1, updated_at = $2 WHERE id = $3`, newBalance, time.Now(), walletID)
	return err
}

func (r *WalletRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *domain.WalletTransaction) error {
	query := `INSERT INTO wallet_transactions (id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := tx.ExecContext(ctx, query, walletTx.ID, walletTx.WalletID, walletTx.UserID, walletTx.Type, walletTx.Status, walletTx.Amount, walletTx.BalanceBefore, walletTx.BalanceAfter, walletTx.Reference, walletTx.Description, walletTx.Metadata, time.Now(), time.Now())
	return err
}

func (r *WalletRepo) CountTransactionsByWalletID(ctx context.Context, walletID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1`, walletID).Scan(&count)
	return count, err
}
