package application

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/wallet/domain"
	"github.com/raffle-app/backend/pkg/errors"
)

type WalletService struct {
	repo         domain.WalletRepository
	auditService *auditapp.AuditService
}

func NewWalletService(repo domain.WalletRepository, auditService *auditapp.AuditService) *WalletService {
	return &WalletService{
		repo:         repo,
		auditService: auditService,
	}
}

func (s *WalletService) GetWallet(ctx context.Context, userID string) (*domain.Wallet, error) {
	wallet, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		if err == errors.ErrNotFound {
			wallet = &domain.Wallet{
				ID:        generateID(),
				UserID:    userID,
				Balance:   0,
				Currency:  "ETB",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := s.repo.Create(ctx, wallet); err != nil {
				return nil, err
			}
			return wallet, nil
		}
		return nil, err
	}
	return wallet, nil
}

func (s *WalletService) Deposit(ctx context.Context, userID string, amount float64, reference, description string) (*domain.WalletTransaction, error) {
	if amount <= 0 {
		return nil, errors.ErrValidationFailed
	}
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}
	newBalance := wallet.Balance + amount
	tx := &domain.WalletTransaction{
		ID:            generateID(),
		WalletID:      wallet.ID,
		UserID:        userID,
		Type:          "deposit",
		Status:        "completed",
		Amount:        amount,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  newBalance,
		Reference:     reference,
		Description:   description,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.repo.CreateTransaction(ctx, tx); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateBalance(ctx, wallet.ID, newBalance); err != nil {
		return nil, err
	}

	// Record deposit audit log
	if s.auditService != nil {
		oldVal := fmt.Sprintf("%.2f", wallet.Balance)
		newVal := fmt.Sprintf("%.2f", newBalance)
		_ = s.auditService.Record(ctx, &userID, "user", "deposit", "wallet", &wallet.ID, "", &oldVal, &newVal)
	}

	return tx, nil
}

func (s *WalletService) Withdraw(ctx context.Context, userID string, amount float64, reference, description string) (*domain.WalletTransaction, error) {
	if amount <= 0 {
		return nil, errors.ErrValidationFailed
	}
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}
	if wallet.Balance < amount {
		return nil, errors.ErrInsufficientFunds
	}

	tx := &domain.WalletTransaction{
		ID:            generateID(),
		WalletID:      wallet.ID,
		UserID:        userID,
		Type:          "withdrawal",
		Status:        "pending",
		Amount:        amount,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  wallet.Balance, // unchanged until admin approves
		Reference:     reference,
		Description:   description,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.repo.CreateTransaction(ctx, tx); err != nil {
		return nil, err
	}

	// Balance is NOT deducted — admin will approve and send manually within 24h
	if s.auditService != nil {
		oldVal := fmt.Sprintf("%.2f", wallet.Balance)
		newVal := "pending_withdrawal"
		_ = s.auditService.Record(ctx, &userID, "user", "withdrawal_request", "wallet", &wallet.ID, "", &oldVal, &newVal)
	}

	return tx, nil
}

func (s *WalletService) ListPendingWithdrawals(ctx context.Context) ([]*domain.WalletTransaction, error) {
	return s.repo.FindTransactionsByStatus(ctx, "withdrawal", "pending")
}

func (s *WalletService) ApproveWithdrawal(ctx context.Context, txID, adminUserID string) (*domain.WalletTransaction, error) {
	tx, err := s.repo.FindTransactionByID(ctx, txID)
	if err != nil {
		return nil, errors.ErrNotFound
	}
	if tx.Type != "withdrawal" || tx.Status != "pending" {
		return nil, fmt.Errorf("transaction is not a pending withdrawal")
	}

	// Deduct balance now
	wallet, err := s.repo.FindByID(ctx, tx.WalletID)
	if err != nil {
		return nil, err
	}
	if wallet.Balance < tx.Amount {
		return nil, errors.ErrInsufficientFunds
	}

	newBalance := wallet.Balance - tx.Amount
	if err := s.repo.UpdateBalance(ctx, wallet.ID, newBalance); err != nil {
		return nil, err
	}

	tx.Status = "completed"
	tx.BalanceAfter = newBalance
	tx.UpdatedAt = time.Now()
	if err := s.repo.UpdateTransactionStatus(ctx, tx.ID, "completed"); err != nil {
		return nil, err
	}

	if s.auditService != nil {
		oldVal := fmt.Sprintf("%.2f", wallet.Balance+tx.Amount)
		newVal := fmt.Sprintf("%.2f", newBalance)
		_ = s.auditService.Record(ctx, &adminUserID, "admin", "withdrawal_approved", "wallet", &wallet.ID, "", &oldVal, &newVal)
	}

	return tx, nil
}

func (s *WalletService) RejectWithdrawal(ctx context.Context, txID, adminUserID string) (*domain.WalletTransaction, error) {
	tx, err := s.repo.FindTransactionByID(ctx, txID)
	if err != nil {
		return nil, errors.ErrNotFound
	}
	if tx.Type != "withdrawal" || tx.Status != "pending" {
		return nil, fmt.Errorf("transaction is not a pending withdrawal")
	}

	tx.Status = "rejected"
	tx.UpdatedAt = time.Now()
	if err := s.repo.UpdateTransactionStatus(ctx, tx.ID, "rejected"); err != nil {
		return nil, err
	}

	if s.auditService != nil {
		_ = s.auditService.Record(ctx, &adminUserID, "admin", "withdrawal_rejected", "wallet", &tx.WalletID, "", nil, nil)
	}

	return tx, nil
}

func (s *WalletService) GetTransactionHistory(ctx context.Context, userID string, limit, offset int) ([]*domain.WalletTransaction, int64, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	txs, err := s.repo.FindTransactionsByWalletID(ctx, wallet.ID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.CountTransactionsByWalletID(ctx, wallet.ID)
	if err != nil {
		return nil, 0, err
	}
	return txs, count, nil
}

func generateID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}
