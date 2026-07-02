package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
)

type TicketWalletRepo struct {
	db *sql.DB
}

func NewTicketWalletRepo(db *sql.DB) *TicketWalletRepo {
	return &TicketWalletRepo{db: db}
}

func (r *TicketWalletRepo) FindByUserID(ctx context.Context, userID string) (*ticketdomain.Wallet, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, balance FROM wallets WHERE user_id = $1`, userID)
	wallet := &ticketdomain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.Balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return wallet, nil
}

func (r *TicketWalletRepo) FindByUserIDForUpdate(ctx context.Context, tx *sql.Tx, userID string) (*ticketdomain.Wallet, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID)
	wallet := &ticketdomain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.Balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return wallet, nil
}

func (r *TicketWalletRepo) UpdateBalance(ctx context.Context, walletID string, newBalance float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wallets SET balance = $1, updated_at = $2 WHERE id = $3`, newBalance, time.Now(), walletID)
	return err
}

func (r *TicketWalletRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	_, err := tx.ExecContext(ctx, `UPDATE wallets SET balance = $1, updated_at = $2 WHERE id = $3`, newBalance, time.Now(), walletID)
	return err
}

func (r *TicketWalletRepo) DebitTx(ctx context.Context, tx *sql.Tx, walletID string, amount float64) error {
	_, err := tx.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1, updated_at = $2 WHERE id = $3`, amount, time.Now(), walletID)
	return err
}

func (r *TicketWalletRepo) Debit(ctx context.Context, walletID string, amount float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1, updated_at = $2 WHERE id = $3`, amount, time.Now(), walletID)
	return err
}

func (r *TicketWalletRepo) CreateTransaction(ctx context.Context, walletTx *ticketdomain.WalletTransaction) error {
	query := `INSERT INTO wallet_transactions (id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	metaJSON, err := marshalWalletMetadata(walletTx.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, query, walletTx.ID, walletTx.WalletID, walletTx.UserID, walletTx.Type, walletTx.Status, walletTx.Amount, walletTx.BalanceBefore, walletTx.BalanceAfter, walletTx.Reference, walletTx.Description, metaJSON, time.Now(), time.Now())
	return err
}

func (r *TicketWalletRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *ticketdomain.WalletTransaction) error {
	query := `INSERT INTO wallet_transactions (id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	metaJSON, err := marshalWalletMetadata(walletTx.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, query, walletTx.ID, walletTx.WalletID, walletTx.UserID, walletTx.Type, walletTx.Status, walletTx.Amount, walletTx.BalanceBefore, walletTx.BalanceAfter, walletTx.Reference, walletTx.Description, metaJSON, time.Now(), time.Now())
	return err
}

// marshalWalletMetadata converts a map to JSON bytes for database storage.
func marshalWalletMetadata(m map[string]interface{}) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return b, nil
}
