package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	metaJSON, err := marshalMetadata(tx.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, query, tx.ID, tx.WalletID, tx.UserID, tx.Type, tx.Status, tx.Amount, tx.BalanceBefore, tx.BalanceAfter, tx.Reference, tx.Description, metaJSON, time.Now(), time.Now())
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
		var metaJSON []byte
		if err := rows.Scan(&tx.ID, &tx.WalletID, &tx.UserID, &tx.Type, &tx.Status, &tx.Amount, &tx.BalanceBefore, &tx.BalanceAfter, &tx.Reference, &tx.Description, &metaJSON, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
			return nil, err
		}
		if metaJSON != nil {
			if err := json.Unmarshal(metaJSON, &tx.Metadata); err != nil {
				return nil, err
			}
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
	metaJSON, err := marshalMetadata(walletTx.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, query, walletTx.ID, walletTx.WalletID, walletTx.UserID, walletTx.Type, walletTx.Status, walletTx.Amount, walletTx.BalanceBefore, walletTx.BalanceAfter, walletTx.Reference, walletTx.Description, metaJSON, time.Now(), time.Now())
	return err
}

func (r *WalletRepo) CountTransactionsByWalletID(ctx context.Context, walletID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1`, walletID).Scan(&count)
	return count, err
}

func (r *WalletRepo) FindTransactionsByStatus(ctx context.Context, txType, status string) ([]*domain.WalletTransaction, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at
		 FROM wallet_transactions WHERE type = $1 AND status = $2 ORDER BY created_at DESC`, txType, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []*domain.WalletTransaction
	for rows.Next() {
		tx := &domain.WalletTransaction{}
		var metaJSON []byte
		if err := rows.Scan(&tx.ID, &tx.WalletID, &tx.UserID, &tx.Type, &tx.Status, &tx.Amount, &tx.BalanceBefore, &tx.BalanceAfter, &tx.Reference, &tx.Description, &metaJSON, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
			return nil, err
		}
		if metaJSON != nil {
			if err := json.Unmarshal(metaJSON, &tx.Metadata); err != nil {
				return nil, err
			}
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func (r *WalletRepo) FindTransactionByID(ctx context.Context, id string) (*domain.WalletTransaction, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at
		 FROM wallet_transactions WHERE id = $1`, id)
	tx := &domain.WalletTransaction{}
	var metaJSON []byte
	err := row.Scan(&tx.ID, &tx.WalletID, &tx.UserID, &tx.Type, &tx.Status, &tx.Amount, &tx.BalanceBefore, &tx.BalanceAfter, &tx.Reference, &tx.Description, &metaJSON, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	if metaJSON != nil {
		if err := json.Unmarshal(metaJSON, &tx.Metadata); err != nil {
			return nil, err
		}
	}
	return tx, nil
}

func (r *WalletRepo) UpdateTransactionStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wallet_transactions SET status = $1, updated_at = $2 WHERE id = $3`, status, time.Now(), id)
	return err
}

// marshalMetadata converts a map to a JSON byte slice for database storage.
// Returns []byte("null") for nil maps so lib/pq sends valid JSON to PostgreSQL
// instead of relying on nil→NULL driver conversion (which can produce empty strings
// that JSONB rejects with "invalid input syntax for type json").
func marshalMetadata(m map[string]interface{}) ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return b, nil
}
