package application

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/wallet/domain"
	"github.com/raffle-app/backend/pkg/errors"
)

type mockWithdrawalRepo struct {
	wallet        *domain.Wallet
	pendingTxs    map[string]*domain.WalletTransaction
	requestErr    error
	approveErr    error
	rejectErr     error
	requestCalled bool
	approveCalled bool
	rejectCalled  bool
}

func newMockWithdrawalRepo(balance, reserved float64) *mockWithdrawalRepo {
	return &mockWithdrawalRepo{
		wallet: &domain.Wallet{
			ID:               "w-test-1",
			UserID:           "u-test-1",
			Balance:          balance,
			ReservedBalance:  reserved,
			AvailableBalance: balance - reserved,
			Currency:         "ETB",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
		pendingTxs: make(map[string]*domain.WalletTransaction),
	}
}

func (m *mockWithdrawalRepo) RequestWithdrawal(ctx context.Context, userID string, amount float64, reference, description string) (*domain.WalletTransaction, *domain.Wallet, error) {
	m.requestCalled = true
	if m.requestErr != nil {
		return nil, nil, m.requestErr
	}
	if amount <= 0 {
		return nil, nil, errors.ErrValidationFailed
	}
	if m.wallet.Balance-m.wallet.ReservedBalance < amount {
		return nil, nil, errors.ErrInsufficientFunds
	}
	m.wallet.ReservedBalance += amount
	m.wallet.AvailableBalance = m.wallet.Balance - m.wallet.ReservedBalance
	tx := &domain.WalletTransaction{
		ID:            "tx-wd-1",
		WalletID:      m.wallet.ID,
		UserID:        userID,
		Type:          "withdrawal",
		Status:        "pending",
		Amount:        amount,
		BalanceBefore: m.wallet.Balance,
		BalanceAfter:  m.wallet.Balance,
		Reference:     reference,
		Description:   description,
		Metadata:      map[string]interface{}{"reserved_amount": amount},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	m.pendingTxs[tx.ID] = tx
	return tx, m.wallet, nil
}

func (m *mockWithdrawalRepo) ApproveWithdrawal(ctx context.Context, id string) (*domain.WalletTransaction, *domain.Wallet, error) {
	m.approveCalled = true
	if m.approveErr != nil {
		return nil, nil, m.approveErr
	}
	tx, ok := m.pendingTxs[id]
	if !ok {
		return nil, nil, errors.ErrNotFound
	}
	if tx.Status != "pending" {
		return nil, nil, errors.ErrConflict
	}
	if m.wallet.Balance < tx.Amount {
		return nil, nil, errors.ErrInsufficientFunds
	}
	m.wallet.Balance -= tx.Amount
	m.wallet.ReservedBalance = max(0, m.wallet.ReservedBalance-tx.Amount)
	m.wallet.AvailableBalance = m.wallet.Balance - m.wallet.ReservedBalance
	tx.Status = "completed"
	tx.BalanceAfter = m.wallet.Balance
	return tx, m.wallet, nil
}

func (m *mockWithdrawalRepo) RejectWithdrawal(ctx context.Context, id string) (*domain.WalletTransaction, *domain.Wallet, error) {
	m.rejectCalled = true
	if m.rejectErr != nil {
		return nil, nil, m.rejectErr
	}
	tx, ok := m.pendingTxs[id]
	if !ok {
		return nil, nil, errors.ErrNotFound
	}
	if tx.Status != "pending" {
		return nil, nil, errors.ErrConflict
	}
	m.wallet.ReservedBalance = max(0, m.wallet.ReservedBalance-tx.Amount)
	m.wallet.AvailableBalance = m.wallet.Balance - m.wallet.ReservedBalance
	tx.Status = "rejected"
	return tx, m.wallet, nil
}

// Stubs for domain.WalletRepository interface
func (m *mockWithdrawalRepo) FindByID(ctx context.Context, id string) (*domain.Wallet, error) {
	return m.wallet, nil
}
func (m *mockWithdrawalRepo) FindByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	return m.wallet, nil
}
func (m *mockWithdrawalRepo) Create(ctx context.Context, wallet *domain.Wallet) error { return nil }
func (m *mockWithdrawalRepo) CreateTx(ctx context.Context, tx *sql.Tx, wallet *domain.Wallet) error {
	return nil
}
func (m *mockWithdrawalRepo) FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*domain.Wallet, error) {
	return m.wallet, nil
}
func (m *mockWithdrawalRepo) GetBalance(ctx context.Context, walletID string) (float64, error) {
	return m.wallet.Balance, nil
}
func (m *mockWithdrawalRepo) UpdateBalance(ctx context.Context, walletID string, newBalance float64) error {
	m.wallet.Balance = newBalance
	return nil
}
func (m *mockWithdrawalRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	return nil
}
func (m *mockWithdrawalRepo) Debit(ctx context.Context, walletID string, amount float64) error {
	return nil
}
func (m *mockWithdrawalRepo) CreateTransaction(ctx context.Context, tx *domain.WalletTransaction) error {
	return nil
}
func (m *mockWithdrawalRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *domain.WalletTransaction) error {
	return nil
}
func (m *mockWithdrawalRepo) FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int, txType string) ([]*domain.WalletTransaction, error) {
	return nil, nil
}
func (m *mockWithdrawalRepo) CountTransactionsByWalletID(ctx context.Context, walletID string, txType string) (int64, error) {
	return 0, nil
}
func (m *mockWithdrawalRepo) FindTransactionsByStatus(ctx context.Context, txType, status string) ([]*domain.WalletTransaction, error) {
	return nil, nil
}
func (m *mockWithdrawalRepo) FindTransactionByID(ctx context.Context, id string) (*domain.WalletTransaction, error) {
	if tx, ok := m.pendingTxs[id]; ok {
		return tx, nil
	}
	return nil, errors.ErrNotFound
}
func (m *mockWithdrawalRepo) UpdateTransactionStatus(ctx context.Context, id, status string) error {
	return nil
}
func (m *mockWithdrawalRepo) ExistsByReference(ctx context.Context, reference string) (bool, error) {
	return false, nil
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func TestWithdraw_EscrowReservation(t *testing.T) {
	repo := newMockWithdrawalRepo(500, 0)
	svc := NewWalletService(repo, nil)

	ctx := context.Background()
	tx, err := svc.Withdraw(ctx, "u-test-1", 200, "telebirr:0911223344", "Cash out")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Amount != 200 {
		t.Errorf("expected tx.Amount 200, got %.2f", tx.Amount)
	}
	if tx.Status != "pending" {
		t.Errorf("expected pending status, got %s", tx.Status)
	}
	// Verify reserved balance increased and spendable available balance decreased
	if repo.wallet.ReservedBalance != 200 {
		t.Errorf("expected reserved_balance 200, got %.2f", repo.wallet.ReservedBalance)
	}
	if repo.wallet.AvailableBalance != 300 {
		t.Errorf("expected available_balance 300, got %.2f", repo.wallet.AvailableBalance)
	}
	// Total balance remains untouched until approval
	if repo.wallet.Balance != 500 {
		t.Errorf("expected balance 500, got %.2f", repo.wallet.Balance)
	}
}

func TestWithdraw_InsufficientAvailableFunds(t *testing.T) {
	// Wallet has 500 total, but 400 is already reserved (e.g. active game stake or pending withdrawal)
	repo := newMockWithdrawalRepo(500, 400)
	svc := NewWalletService(repo, nil)

	ctx := context.Background()
	// Requesting 150 when available is 100 must fail with ErrInsufficientFunds
	_, err := svc.Withdraw(ctx, "u-test-1", 150, "ref", "desc")
	if err != errors.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	// Balances must remain unchanged
	if repo.wallet.ReservedBalance != 400 {
		t.Errorf("expected reserved_balance 400, got %.2f", repo.wallet.ReservedBalance)
	}
}

func TestWithdraw_ValidationErrors(t *testing.T) {
	repo := newMockWithdrawalRepo(500, 0)
	svc := NewWalletService(repo, nil)

	ctx := context.Background()
	for _, invalidAmount := range []float64{0, -10, -0.01} {
		_, err := svc.Withdraw(ctx, "u-test-1", invalidAmount, "ref", "desc")
		if err != errors.ErrValidationFailed {
			t.Errorf("amount %.2f: expected ErrValidationFailed, got %v", invalidAmount, err)
		}
	}
}

func TestApproveWithdrawal_Lifecycle(t *testing.T) {
	repo := newMockWithdrawalRepo(500, 0)
	svc := NewWalletService(repo, nil)

	ctx := context.Background()
	tx, err := svc.Withdraw(ctx, "u-test-1", 200, "ref-1", "desc")
	if err != nil {
		t.Fatalf("withdrawal setup failed: %v", err)
	}

	approvedTx, err := svc.ApproveWithdrawal(ctx, tx.ID, "admin-1")
	if err != nil {
		t.Fatalf("ApproveWithdrawal failed: %v", err)
	}
	if approvedTx.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", approvedTx.Status)
	}
	// Balance debited from 500 to 300, reserved balance released from 200 to 0
	if repo.wallet.Balance != 300 {
		t.Errorf("expected balance 300, got %.2f", repo.wallet.Balance)
	}
	if repo.wallet.ReservedBalance != 0 {
		t.Errorf("expected reserved_balance 0, got %.2f", repo.wallet.ReservedBalance)
	}
	if repo.wallet.AvailableBalance != 300 {
		t.Errorf("expected available_balance 300, got %.2f", repo.wallet.AvailableBalance)
	}

	// Double approval must fail with ErrConflict
	_, err = svc.ApproveWithdrawal(ctx, tx.ID, "admin-1")
	if err != errors.ErrConflict {
		t.Errorf("expected ErrConflict on double approval, got %v", err)
	}
}

func TestRejectWithdrawal_Lifecycle(t *testing.T) {
	repo := newMockWithdrawalRepo(500, 0)
	svc := NewWalletService(repo, nil)

	ctx := context.Background()
	tx, err := svc.Withdraw(ctx, "u-test-1", 200, "ref-1", "desc")
	if err != nil {
		t.Fatalf("withdrawal setup failed: %v", err)
	}

	rejectedTx, err := svc.RejectWithdrawal(ctx, tx.ID, "admin-1")
	if err != nil {
		t.Fatalf("RejectWithdrawal failed: %v", err)
	}
	if rejectedTx.Status != "rejected" {
		t.Errorf("expected status 'rejected', got %s", rejectedTx.Status)
	}
	// Balance untouched at 500, reserved balance released from 200 to 0, available back to 500
	if repo.wallet.Balance != 500 {
		t.Errorf("expected balance 500, got %.2f", repo.wallet.Balance)
	}
	if repo.wallet.ReservedBalance != 0 {
		t.Errorf("expected reserved_balance 0, got %.2f", repo.wallet.ReservedBalance)
	}
	if repo.wallet.AvailableBalance != 500 {
		t.Errorf("expected available_balance 500, got %.2f", repo.wallet.AvailableBalance)
	}

	// Double rejection must fail with ErrConflict
	_, err = svc.RejectWithdrawal(ctx, tx.ID, "admin-1")
	if err != errors.ErrConflict {
		t.Errorf("expected ErrConflict on double rejection, got %v", err)
	}
}
