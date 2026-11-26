package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
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
	query := `SELECT id, user_id, balance, reserved_balance, currency, created_at, updated_at FROM wallets WHERE user_id = $1 ORDER BY created_at ASC, id ASC LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, userID)
	wallet := &domain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.ReservedBalance, &wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	wallet.AvailableBalance = wallet.Balance - wallet.ReservedBalance
	return wallet, nil
}

func (r *WalletRepo) FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*domain.Wallet, error) {
	query := `SELECT id, user_id, balance, reserved_balance, currency, created_at, updated_at FROM wallets WHERE user_id = $1 ORDER BY created_at ASC, id ASC LIMIT 1 FOR UPDATE`
	row := tx.QueryRowContext(ctx, query, userID)
	wallet := &domain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.ReservedBalance, &wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	wallet.AvailableBalance = wallet.Balance - wallet.ReservedBalance
	return wallet, nil
}

func (r *WalletRepo) FindByID(ctx context.Context, id string) (*domain.Wallet, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, user_id, balance, reserved_balance, currency, created_at, updated_at FROM wallets WHERE id = $1`, id)
	wallet := &domain.Wallet{}
	err := row.Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.ReservedBalance, &wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	wallet.AvailableBalance = wallet.Balance - wallet.ReservedBalance
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

func (r *WalletRepo) FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int, txType string) ([]*domain.WalletTransaction, error) {
	var rows *sql.Rows
	var err error
	if txType == "deposit" {
		rows, err = r.db.QueryContext(ctx, `SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at FROM wallet_transactions WHERE wallet_id = $1 AND type IN ('deposit', 'sms_deposit') ORDER BY created_at DESC LIMIT $2 OFFSET $3`, walletID, limit, offset)
	} else if txType == "withdraw" || txType == "withdrawal" {
		rows, err = r.db.QueryContext(ctx, `SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at FROM wallet_transactions WHERE wallet_id = $1 AND type IN ('withdrawal', 'withdraw') ORDER BY created_at DESC LIMIT $2 OFFSET $3`, walletID, limit, offset)
	} else if txType != "" {
		rows, err = r.db.QueryContext(ctx, `SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at FROM wallet_transactions WHERE wallet_id = $1 AND type = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`, walletID, txType, limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, `SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at FROM wallet_transactions WHERE wallet_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, walletID, limit, offset)
	}
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

func (r *WalletRepo) CountTransactionsByWalletID(ctx context.Context, walletID string, txType string) (int64, error) {
	var count int64
	var err error
	if txType == "deposit" {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1 AND type IN ('deposit', 'sms_deposit')`, walletID).Scan(&count)
	} else if txType == "withdraw" || txType == "withdrawal" {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1 AND type IN ('withdrawal', 'withdraw')`, walletID).Scan(&count)
	} else if txType != "" {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1 AND type = $2`, walletID, txType).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1`, walletID).Scan(&count)
	}
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

// RequestWithdrawal atomically locks the wallet, validates available funds,
// increases reserved_balance (escrow), and creates a pending withdrawal transaction.
func (r *WalletRepo) RequestWithdrawal(ctx context.Context, userID string, amount float64, reference, description string) (*domain.WalletTransaction, *domain.Wallet, error) {
	if amount <= 0 {
		return nil, nil, errors.ErrValidationFailed
	}

	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer dbTx.Rollback()

	wallet := &domain.Wallet{}
	err = dbTx.QueryRowContext(ctx,
		`SELECT id, user_id, balance, reserved_balance, currency, created_at, updated_at
		 FROM wallets WHERE user_id = $1 ORDER BY created_at ASC, id ASC LIMIT 1 FOR UPDATE`, userID).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.ReservedBalance,
		&wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.ErrNotFound
		}
		return nil, nil, err
	}

	// Funds reserved for active P2P stakes or pending withdrawals cannot be spent or withdrawn.
	if wallet.Balance-wallet.ReservedBalance < amount {
		return nil, nil, errors.ErrInsufficientFunds
	}

	now := time.Now()
	amtCents := int64(math.Round(amount * 100))
	newReserved := wallet.ReservedBalance + amount

	if _, err := dbTx.ExecContext(ctx,
		`UPDATE wallets
		 SET reserved_balance = reserved_balance + $1,
		     reserved_cents = reserved_cents + $2,
		     updated_at = $3
		 WHERE id = $4`,
		amount, amtCents, now, wallet.ID); err != nil {
		return nil, nil, err
	}

	walletTx := &domain.WalletTransaction{
		WalletID:      wallet.ID,
		UserID:        userID,
		Type:          "withdrawal",
		Status:        "pending",
		Amount:        amount,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  wallet.Balance,
		Reference:     reference,
		Description:   description,
		Metadata:      map[string]interface{}{"reserved_amount": amount},
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	metaJSON, err := marshalMetadata(walletTx.Metadata)
	if err != nil {
		return nil, nil, err
	}

	err = dbTx.QueryRowContext(ctx,
		`INSERT INTO wallet_transactions (
			wallet_id, user_id, type, status, amount,
			balance_before, balance_after, reference, description, metadata,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12)
		RETURNING id`,
		walletTx.WalletID, walletTx.UserID, walletTx.Type, walletTx.Status,
		walletTx.Amount, walletTx.BalanceBefore, walletTx.BalanceAfter, walletTx.Reference,
		walletTx.Description, metaJSON, walletTx.CreatedAt, walletTx.UpdatedAt,
	).Scan(&walletTx.ID)
	if err != nil {
		return nil, nil, err
	}

	if err := dbTx.Commit(); err != nil {
		return nil, nil, err
	}

	wallet.ReservedBalance = newReserved
	wallet.AvailableBalance = wallet.Balance - newReserved
	wallet.UpdatedAt = now

	return walletTx, wallet, nil
}

// ApproveWithdrawal locks the withdrawal and wallet rows before changing either
// one. It releases the reserved escrow funds and debits the total balance.
// Two admins approving the same request therefore cannot both debit the
// wallet, and a database failure rolls the whole action back.
func (r *WalletRepo) ApproveWithdrawal(ctx context.Context, id string) (*domain.WalletTransaction, *domain.Wallet, error) {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer dbTx.Rollback()

	walletTx := &domain.WalletTransaction{}
	var metaJSON []byte
	err = dbTx.QueryRowContext(ctx,
		`SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at
		 FROM wallet_transactions WHERE id = $1 FOR UPDATE`, id).Scan(
		&walletTx.ID, &walletTx.WalletID, &walletTx.UserID, &walletTx.Type, &walletTx.Status,
		&walletTx.Amount, &walletTx.BalanceBefore, &walletTx.BalanceAfter, &walletTx.Reference,
		&walletTx.Description, &metaJSON, &walletTx.CreatedAt, &walletTx.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.ErrNotFound
		}
		return nil, nil, err
	}
	if walletTx.Type != "withdrawal" || walletTx.Status != "pending" {
		return nil, nil, errors.ErrConflict
	}
	if metaJSON != nil {
		if err := json.Unmarshal(metaJSON, &walletTx.Metadata); err != nil {
			return nil, nil, err
		}
	}

	wallet := &domain.Wallet{}
	err = dbTx.QueryRowContext(ctx,
		`SELECT id, user_id, balance, reserved_balance, currency, created_at, updated_at
		 FROM wallets WHERE id = $1 FOR UPDATE`, walletTx.WalletID).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.ReservedBalance,
		&wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.ErrNotFound
		}
		return nil, nil, err
	}
	if wallet.Balance < walletTx.Amount {
		return nil, nil, errors.ErrInsufficientFunds
	}

	now := time.Now()
	amtCents := int64(math.Round(walletTx.Amount * 100))
	newBalance := wallet.Balance - walletTx.Amount
	newReserved := math.Max(0, wallet.ReservedBalance - walletTx.Amount)

	// Consume reserved funds and debit balance
	if _, err := dbTx.ExecContext(ctx,
		`UPDATE wallets
		 SET balance = balance - $1,
		     reserved_balance = GREATEST(0, reserved_balance - $1),
		     reserved_cents = GREATEST(0, reserved_cents - $2),
		     updated_at = $3
		 WHERE id = $4`,
		walletTx.Amount, amtCents, now, wallet.ID); err != nil {
		return nil, nil, err
	}
	if _, err := dbTx.ExecContext(ctx,
		`UPDATE wallet_transactions SET status = 'completed', balance_after = $1, updated_at = $2 WHERE id = $3`,
		newBalance, now, walletTx.ID); err != nil {
		return nil, nil, err
	}
	if err := dbTx.Commit(); err != nil {
		return nil, nil, err
	}

	wallet.Balance = newBalance
	wallet.ReservedBalance = newReserved
	wallet.AvailableBalance = newBalance - newReserved
	wallet.UpdatedAt = now
	walletTx.Status = "completed"
	walletTx.BalanceBefore = wallet.Balance + walletTx.Amount
	walletTx.BalanceAfter = newBalance
	walletTx.UpdatedAt = now
	return walletTx, wallet, nil
}

// RejectWithdrawal atomically moves a pending withdrawal to rejected and releases
// the reserved funds from escrow back into the available wallet balance.
func (r *WalletRepo) RejectWithdrawal(ctx context.Context, id string) (*domain.WalletTransaction, *domain.Wallet, error) {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer dbTx.Rollback()

	walletTx := &domain.WalletTransaction{}
	var metaJSON []byte
	err = dbTx.QueryRowContext(ctx,
		`SELECT id, wallet_id, user_id, type, status, amount, balance_before, balance_after, reference, description, metadata, created_at, updated_at
		 FROM wallet_transactions WHERE id = $1 FOR UPDATE`, id).Scan(
		&walletTx.ID, &walletTx.WalletID, &walletTx.UserID, &walletTx.Type, &walletTx.Status,
		&walletTx.Amount, &walletTx.BalanceBefore, &walletTx.BalanceAfter, &walletTx.Reference,
		&walletTx.Description, &metaJSON, &walletTx.CreatedAt, &walletTx.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.ErrNotFound
		}
		return nil, nil, err
	}
	if walletTx.Type != "withdrawal" || walletTx.Status != "pending" {
		return nil, nil, errors.ErrConflict
	}
	if metaJSON != nil {
		if err := json.Unmarshal(metaJSON, &walletTx.Metadata); err != nil {
			return nil, nil, err
		}
	}

	wallet := &domain.Wallet{}
	err = dbTx.QueryRowContext(ctx,
		`SELECT id, user_id, balance, reserved_balance, currency, created_at, updated_at
		 FROM wallets WHERE id = $1 FOR UPDATE`, walletTx.WalletID).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.ReservedBalance,
		&wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.ErrNotFound
		}
		return nil, nil, err
	}

	now := time.Now()
	amtCents := int64(math.Round(walletTx.Amount * 100))
	newReserved := math.Max(0, wallet.ReservedBalance - walletTx.Amount)

	// Release reserved escrow funds back to available balance without modifying total balance
	if _, err := dbTx.ExecContext(ctx,
		`UPDATE wallets
		 SET reserved_balance = GREATEST(0, reserved_balance - $1),
		     reserved_cents = GREATEST(0, reserved_cents - $2),
		     updated_at = $3
		 WHERE id = $4`,
		walletTx.Amount, amtCents, now, wallet.ID); err != nil {
		return nil, nil, err
	}

	if _, err := dbTx.ExecContext(ctx,
		`UPDATE wallet_transactions
		 SET status = 'rejected',
		     updated_at = $1
		 WHERE id = $2`,
		now, walletTx.ID); err != nil {
		return nil, nil, err
	}

	if err := dbTx.Commit(); err != nil {
		return nil, nil, err
	}

	wallet.ReservedBalance = newReserved
	wallet.AvailableBalance = wallet.Balance - newReserved
	wallet.UpdatedAt = now

	walletTx.Status = "rejected"
	walletTx.UpdatedAt = now

	return walletTx, wallet, nil
}

func (r *WalletRepo) ExistsByReference(ctx context.Context, reference string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM wallet_transactions WHERE reference = $1 AND status NOT IN ('failed', 'rejected'))`
	err := r.db.QueryRowContext(ctx, query, reference).Scan(&exists)
	return exists, err
}

// marshalMetadata converts a map to a JSON string for database storage.
// Returns "null" for nil maps so lib/pq sends valid JSON to PostgreSQL
// instead of relying on nil→NULL driver conversion (which can produce empty strings
// that JSONB rejects with "invalid input syntax for type json").
func marshalMetadata(m map[string]interface{}) (string, error) {
	if m == nil {
		return "null", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return string(b), nil
}

type DepositRepo struct {
	db *sql.DB
}

func NewDepositRepo(db *sql.DB) *DepositRepo {
	return &DepositRepo{db: db}
}

func (r *DepositRepo) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

func (r *DepositRepo) Create(ctx context.Context, d *domain.Deposit) error {
	metaJSON, err := marshalMetadata(d.Metadata)
	if err != nil {
		return err
	}
	query := `INSERT INTO deposits (
		id, user_id, wallet_id, amount, currency, payment_method, status,
		external_reference, receipt_token, receipt_url, payer_name, payer_account,
		receiver_name, receiver_account, actual_amount_credited, fee_amount, error_message,
		metadata, verified_at, credited_at, expires_at, created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7,
		$8, $9, $10, $11, $12,
		$13, $14, $15, $16, $17,
		$18, $19, $20, $21, $22, $23
	)`
	_, err = r.db.ExecContext(ctx, query,
		d.ID, d.UserID, d.WalletID, d.Amount, d.Currency, d.PaymentMethod, d.Status,
		nullIfEmpty(d.ExternalReference), nullIfEmpty(d.ReceiptToken), nullIfEmpty(d.ReceiptURL),
		nullIfEmpty(d.PayerName), nullIfEmpty(d.PayerAccount), nullIfEmpty(d.ReceiverName),
		nullIfEmpty(d.ReceiverAccount), d.ActualAmountCredited, d.FeeAmount, nullIfEmpty(d.ErrorMessage),
		metaJSON, d.VerifiedAt, d.CreditedAt, d.ExpiresAt, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (r *DepositRepo) CreateTx(ctx context.Context, tx *sql.Tx, d *domain.Deposit) error {
	metaJSON, err := marshalMetadata(d.Metadata)
	if err != nil {
		return err
	}
	query := `INSERT INTO deposits (
		id, user_id, wallet_id, amount, currency, payment_method, status,
		external_reference, receipt_token, receipt_url, payer_name, payer_account,
		receiver_name, receiver_account, actual_amount_credited, fee_amount, error_message,
		metadata, verified_at, credited_at, expires_at, created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7,
		$8, $9, $10, $11, $12,
		$13, $14, $15, $16, $17,
		$18, $19, $20, $21, $22, $23
	)`
	_, err = tx.ExecContext(ctx, query,
		d.ID, d.UserID, d.WalletID, d.Amount, d.Currency, d.PaymentMethod, d.Status,
		nullIfEmpty(d.ExternalReference), nullIfEmpty(d.ReceiptToken), nullIfEmpty(d.ReceiptURL),
		nullIfEmpty(d.PayerName), nullIfEmpty(d.PayerAccount), nullIfEmpty(d.ReceiverName),
		nullIfEmpty(d.ReceiverAccount), d.ActualAmountCredited, d.FeeAmount, nullIfEmpty(d.ErrorMessage),
		metaJSON, d.VerifiedAt, d.CreditedAt, d.ExpiresAt, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (r *DepositRepo) FindByID(ctx context.Context, id string) (*domain.Deposit, error) {
	query := `SELECT id, user_id, wallet_id, amount, currency, payment_method, status,
		external_reference, receipt_token, receipt_url, payer_name, payer_account,
		receiver_name, receiver_account, actual_amount_credited, fee_amount, error_message,
		metadata, verified_at, credited_at, expires_at, created_at, updated_at
		FROM deposits WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanDeposit(row)
}

func (r *DepositRepo) FindByIDWithLock(ctx context.Context, tx *sql.Tx, id string) (*domain.Deposit, error) {
	query := `SELECT id, user_id, wallet_id, amount, currency, payment_method, status,
		external_reference, receipt_token, receipt_url, payer_name, payer_account,
		receiver_name, receiver_account, actual_amount_credited, fee_amount, error_message,
		metadata, verified_at, credited_at, expires_at, created_at, updated_at
		FROM deposits WHERE id = $1 FOR UPDATE`
	row := tx.QueryRowContext(ctx, query, id)
	return scanDeposit(row)
}

func (r *DepositRepo) FindByExternalReference(ctx context.Context, ref string) (*domain.Deposit, error) {
	query := `SELECT id, user_id, wallet_id, amount, currency, payment_method, status,
		external_reference, receipt_token, receipt_url, payer_name, payer_account,
		receiver_name, receiver_account, actual_amount_credited, fee_amount, error_message,
		metadata, verified_at, credited_at, expires_at, created_at, updated_at
		FROM deposits WHERE external_reference = $1 LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, ref)
	return scanDeposit(row)
}

func (r *DepositRepo) Update(ctx context.Context, d *domain.Deposit) error {
	metaJSON, err := marshalMetadata(d.Metadata)
	if err != nil {
		return err
	}
	query := `UPDATE deposits SET
		amount = $1, status = $2, external_reference = $3, receipt_token = $4, receipt_url = $5,
		payer_name = $6, payer_account = $7, receiver_name = $8, receiver_account = $9,
		actual_amount_credited = $10, fee_amount = $11, error_message = $12, metadata = $13,
		verified_at = $14, credited_at = $15, updated_at = $16
		WHERE id = $17`
	_, err = r.db.ExecContext(ctx, query,
		d.Amount, d.Status, nullIfEmpty(d.ExternalReference), nullIfEmpty(d.ReceiptToken), nullIfEmpty(d.ReceiptURL),
		nullIfEmpty(d.PayerName), nullIfEmpty(d.PayerAccount), nullIfEmpty(d.ReceiverName),
		nullIfEmpty(d.ReceiverAccount), d.ActualAmountCredited, d.FeeAmount, nullIfEmpty(d.ErrorMessage),
		metaJSON, d.VerifiedAt, d.CreditedAt, time.Now(), d.ID,
	)
	return err
}

func (r *DepositRepo) UpdateTx(ctx context.Context, tx *sql.Tx, d *domain.Deposit) error {
	metaJSON, err := marshalMetadata(d.Metadata)
	if err != nil {
		return err
	}
	query := `UPDATE deposits SET
		amount = $1, status = $2, external_reference = $3, receipt_token = $4, receipt_url = $5,
		payer_name = $6, payer_account = $7, receiver_name = $8, receiver_account = $9,
		actual_amount_credited = $10, fee_amount = $11, error_message = $12, metadata = $13,
		verified_at = $14, credited_at = $15, updated_at = $16
		WHERE id = $17`
	_, err = tx.ExecContext(ctx, query,
		d.Amount, d.Status, nullIfEmpty(d.ExternalReference), nullIfEmpty(d.ReceiptToken), nullIfEmpty(d.ReceiptURL),
		nullIfEmpty(d.PayerName), nullIfEmpty(d.PayerAccount), nullIfEmpty(d.ReceiverName),
		nullIfEmpty(d.ReceiverAccount), d.ActualAmountCredited, d.FeeAmount, nullIfEmpty(d.ErrorMessage),
		metaJSON, d.VerifiedAt, d.CreditedAt, time.Now(), d.ID,
	)
	return err
}

func (r *DepositRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Deposit, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deposits WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT id, user_id, wallet_id, amount, currency, payment_method, status,
		external_reference, receipt_token, receipt_url, payer_name, payer_account,
		receiver_name, receiver_account, actual_amount_credited, fee_amount, error_message,
		metadata, verified_at, credited_at, expires_at, created_at, updated_at
		FROM deposits WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*domain.Deposit
	for rows.Next() {
		d, err := scanDepositRow(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, d)
	}
	return list, total, nil
}

func scanDeposit(row *sql.Row) (*domain.Deposit, error) {
	d := &domain.Deposit{}
	var extRef, receiptToken, receiptURL, payerName, payerAccount sql.NullString
	var receiverName, receiverAccount, errorMsg sql.NullString
	var metaJSON []byte
	err := row.Scan(
		&d.ID, &d.UserID, &d.WalletID, &d.Amount, &d.Currency, &d.PaymentMethod, &d.Status,
		&extRef, &receiptToken, &receiptURL, &payerName, &payerAccount,
		&receiverName, &receiverAccount, &d.ActualAmountCredited, &d.FeeAmount, &errorMsg,
		&metaJSON, &d.VerifiedAt, &d.CreditedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}
	d.ExternalReference = extRef.String
	d.ReceiptToken = receiptToken.String
	d.ReceiptURL = receiptURL.String
	d.PayerName = payerName.String
	d.PayerAccount = payerAccount.String
	d.ReceiverName = receiverName.String
	d.ReceiverAccount = receiverAccount.String
	d.ErrorMessage = errorMsg.String
	if metaJSON != nil {
		_ = json.Unmarshal(metaJSON, &d.Metadata)
	}
	return d, nil
}

func scanDepositRow(rows *sql.Rows) (*domain.Deposit, error) {
	d := &domain.Deposit{}
	var extRef, receiptToken, receiptURL, payerName, payerAccount sql.NullString
	var receiverName, receiverAccount, errorMsg sql.NullString
	var metaJSON []byte
	err := rows.Scan(
		&d.ID, &d.UserID, &d.WalletID, &d.Amount, &d.Currency, &d.PaymentMethod, &d.Status,
		&extRef, &receiptToken, &receiptURL, &payerName, &payerAccount,
		&receiverName, &receiverAccount, &d.ActualAmountCredited, &d.FeeAmount, &errorMsg,
		&metaJSON, &d.VerifiedAt, &d.CreditedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.ExternalReference = extRef.String
	d.ReceiptToken = receiptToken.String
	d.ReceiptURL = receiptURL.String
	d.PayerName = payerName.String
	d.PayerAccount = payerAccount.String
	d.ReceiverName = receiverName.String
	d.ReceiverAccount = receiverAccount.String
	d.ErrorMessage = errorMsg.String
	if metaJSON != nil {
		_ = json.Unmarshal(metaJSON, &d.Metadata)
	}
	return d, nil
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
