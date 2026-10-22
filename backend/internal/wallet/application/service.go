package application

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	realtime "github.com/raffle-app/backend/internal/realtime"
	"github.com/raffle-app/backend/internal/wallet/domain"
	"github.com/raffle-app/backend/internal/wallet/infrastructure"
	"github.com/raffle-app/backend/internal/wallet/infrastructure/verifier"
	"github.com/raffle-app/backend/pkg/errors"
)

type WalletService struct {
	repo                domain.WalletRepository
	depositRepo         domain.DepositRepository
	auditService        *auditapp.AuditService
	realtimeSvc         *realtime.Service
	receiptVerifier     *infrastructure.ReceiptVerifier
	verificationManager *verifier.VerificationManager
}

func NewWalletService(repo domain.WalletRepository, auditService *auditapp.AuditService) *WalletService {
	return &WalletService{
		repo:         repo,
		auditService: auditService,
	}
}

func (s *WalletService) SetDepositRepository(repo domain.DepositRepository) {
	s.depositRepo = repo
}

func (s *WalletService) SetRealtimeService(realtimeSvc *realtime.Service) {
	s.realtimeSvc = realtimeSvc
}

func (s *WalletService) SetReceiptVerifier(verifier *infrastructure.ReceiptVerifier) {
	s.receiptVerifier = verifier
}

func (s *WalletService) SetVerificationManager(vm *verifier.VerificationManager) {
	s.verificationManager = vm
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

	if s.realtimeSvc != nil {
		_ = s.realtimeSvc.Publish(ctx, "wallet_update", userID, "", map[string]interface{}{
			"balance":  newBalance,
			"currency": wallet.Currency,
		})
		_ = s.realtimeSvc.Publish(ctx, "transaction_update", userID, "", tx)
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

	if s.realtimeSvc != nil {
		_ = s.realtimeSvc.Publish(ctx, "withdrawal_request", "", "admin", tx)
		_ = s.realtimeSvc.Publish(ctx, "transaction_update", userID, "", tx)
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

	if s.realtimeSvc != nil {
		_ = s.realtimeSvc.Publish(ctx, "wallet_update", tx.UserID, "", map[string]interface{}{
			"balance":  newBalance,
			"currency": wallet.Currency,
		})
		_ = s.realtimeSvc.Publish(ctx, "withdrawal_status", tx.UserID, "", tx)
		_ = s.realtimeSvc.Publish(ctx, "transaction_update", tx.UserID, "", tx)
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

	if s.realtimeSvc != nil {
		_ = s.realtimeSvc.Publish(ctx, "withdrawal_status", tx.UserID, "", tx)
		_ = s.realtimeSvc.Publish(ctx, "transaction_update", tx.UserID, "", tx)
	}

	return tx, nil
}

func (s *WalletService) GetTransactionHistory(ctx context.Context, userID string, limit, offset int, txType string) ([]*domain.WalletTransaction, int64, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	txs, err := s.repo.FindTransactionsByWalletID(ctx, wallet.ID, limit, offset, txType)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.CountTransactionsByWalletID(ctx, wallet.ID, txType)
	if err != nil {
		return nil, 0, err
	}
	return txs, count, nil
}

func (s *WalletService) CreateDepositIntent(ctx context.Context, userID string, amount float64, method string) (*domain.Deposit, error) {
	if amount < 0 {
		return nil, errors.ErrValidationFailed
	}
	method = strings.ToLower(strings.TrimSpace(method))
	if method != "cbe" && method != "telebirr" {
		return nil, fmt.Errorf("unsupported payment method %q (allowed: cbe, telebirr)", method)
	}

	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	deposit := &domain.Deposit{
		ID:            generateID(),
		UserID:        userID,
		WalletID:      wallet.ID,
		Amount:        amount,
		Currency:      wallet.Currency,
		PaymentMethod: method,
		Status:        domain.DepositStatusPending,
		ExpiresAt:     time.Now().Add(30 * time.Minute),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if s.depositRepo != nil {
		if err := s.depositRepo.Create(ctx, deposit); err != nil {
			return nil, fmt.Errorf("failed to create deposit intent: %w", err)
		}
	}

	if s.auditService != nil {
		amountStr := fmt.Sprintf("%.2f", amount)
		_ = s.auditService.Record(ctx, &userID, "user", "create_deposit_intent", "deposit", &deposit.ID, "", nil, &amountStr)
	}

	return deposit, nil
}

func (s *WalletService) GetDeposit(ctx context.Context, depositID, userID string) (*domain.Deposit, error) {
	if s.depositRepo == nil {
		return nil, fmt.Errorf("deposit repository is not configured")
	}
	deposit, err := s.depositRepo.FindByID(ctx, depositID)
	if err != nil {
		return nil, err
	}
	if deposit.UserID != userID {
		return nil, errors.ErrForbidden
	}
	return deposit, nil
}

func (s *WalletService) ListUserDeposits(ctx context.Context, userID string, limit, offset int) ([]*domain.Deposit, int64, error) {
	if s.depositRepo == nil {
		return nil, 0, fmt.Errorf("deposit repository is not configured")
	}
	return s.depositRepo.ListByUserID(ctx, userID, limit, offset)
}

func (s *WalletService) VerifyDeposit(ctx context.Context, depositID, userID, tokenOrRef, rawURL string, file multipart.File, contentType string) (*domain.Deposit, *domain.WalletTransaction, error) {
	if s.depositRepo == nil {
		return nil, nil, fmt.Errorf("deposit repository is not configured")
	}

	// 1. Retrieve deposit
	deposit, err := s.depositRepo.FindByID(ctx, depositID)
	if err != nil {
		return nil, nil, fmt.Errorf("deposit not found: %w", err)
	}
	if deposit.UserID != userID {
		return nil, nil, errors.ErrForbidden
	}

	if deposit.Status == domain.DepositStatusCredited {
		return deposit, nil, fmt.Errorf("deposit has already been credited")
	}
	if time.Now().After(deposit.ExpiresAt) {
		deposit.Status = domain.DepositStatusExpired
		_ = s.depositRepo.Update(ctx, deposit)
		return deposit, nil, fmt.Errorf("deposit intent has expired")
	}

	// 2. Extract reference/token from image if uploaded
	if file != nil {
		imageBytes, err := io.ReadAll(file)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read receipt image: %w", err)
		}
		if s.verificationManager != nil && s.verificationManager.ImageExtractor() != nil {
			extracted, err := s.verificationManager.ImageExtractor().Extract(ctx, deposit.PaymentMethod, imageBytes, contentType)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to extract receipt reference from image: %w", err)
			}
			tokenOrRef = extracted
		}
	}

	tokenOrRef = strings.TrimSpace(tokenOrRef)
	if tokenOrRef == "" && rawURL == "" {
		return nil, nil, fmt.Errorf("transaction reference, receipt token, URL, or screenshot is required")
	}

	// 3. Update status to verifying
	deposit.Status = domain.DepositStatusVerifying
	deposit.ReceiptURL = rawURL
	if strings.HasPrefix(tokenOrRef, "v2-") {
		deposit.ReceiptToken = tokenOrRef
	} else if tokenOrRef != "" {
		deposit.ExternalReference = tokenOrRef
	}
	_ = s.depositRepo.Update(ctx, deposit)

	// 4. Verify with provider
	if s.verificationManager == nil {
		return nil, nil, fmt.Errorf("verification manager is not configured")
	}

	verifyReq := domain.VerificationRequest{
		Method:         deposit.PaymentMethod,
		TransactionRef: tokenOrRef,
		ReceiptToken:   deposit.ReceiptToken,
		RawURL:         rawURL,
		ExpectedAmount: deposit.Amount,
	}

	result, err := s.verificationManager.Verify(ctx, verifyReq)
	if err != nil {
		deposit.Status = domain.DepositStatusFailed
		deposit.ErrorMessage = err.Error()
		_ = s.depositRepo.Update(ctx, deposit)
		return deposit, nil, fmt.Errorf("verification failed: %w", err)
	}

	// 5. Atomic credit with database transaction & row locks
	tx, err := s.depositRepo.BeginTx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start database transaction: %w", err)
	}
	if tx != nil {
		defer tx.Rollback()
	}

	// Check if this external reference was already credited
	exists, err := s.repo.ExistsByReference(ctx, result.TransactionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check reference uniqueness: %w", err)
	}
	if exists {
		deposit.Status = domain.DepositStatusRejected
		deposit.ErrorMessage = fmt.Sprintf("transaction %s has already been credited", result.TransactionID)
		_ = s.depositRepo.Update(ctx, deposit)
		return deposit, nil, fmt.Errorf("transaction %s has already been credited", result.TransactionID)
	}

	// Lock wallet row
	wallet, err := s.repo.FindByUserIDWithLock(ctx, tx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lock wallet row: %w", err)
	}

	newBalance := wallet.Balance + result.AmountCredited
	walletTx := &domain.WalletTransaction{
		ID:            generateID(),
		WalletID:      wallet.ID,
		UserID:        userID,
		Type:          "deposit",
		Status:        "completed",
		Amount:        result.AmountCredited,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  newBalance,
		Reference:     result.TransactionID,
		Description:   fmt.Sprintf("Deposit via %s (%s)", strings.ToUpper(result.Provider), result.TransactionID),
		Metadata: map[string]interface{}{
			"deposit_id":     deposit.ID,
			"payment_method": result.Provider,
			"receipt_url":    result.ReceiptURL,
			"receiver_name":  result.ReceiverName,
			"payer_name":     result.PayerName,
			"transfer_date":  result.TransactionTime,
			"debited_amount": result.AmountDebited,
			"fee_amount":     result.FeeAmount,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateTransactionTx(ctx, tx, walletTx); err != nil {
		return nil, nil, fmt.Errorf("failed to insert transaction record: %w", err)
	}
	if err := s.repo.UpdateBalanceTx(ctx, tx, wallet.ID, newBalance); err != nil {
		return nil, nil, fmt.Errorf("failed to update wallet balance: %w", err)
	}

	now := time.Now()
	deposit.Status = domain.DepositStatusCredited
	deposit.ExternalReference = result.TransactionID
	deposit.ActualAmountCredited = result.AmountCredited
	deposit.FeeAmount = result.FeeAmount
	deposit.PayerName = result.PayerName
	deposit.PayerAccount = result.PayerAccount
	deposit.ReceiverName = result.ReceiverName
	deposit.ReceiverAccount = result.ReceiverAccount
	deposit.ReceiptURL = result.ReceiptURL
	deposit.VerifiedAt = &now
	deposit.CreditedAt = &now
	deposit.ErrorMessage = ""
	deposit.Metadata = result.RawPayload

	if err := s.depositRepo.UpdateTx(ctx, tx, deposit); err != nil {
		return nil, nil, fmt.Errorf("failed to update deposit status: %w", err)
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	// 6. Audit & Realtime notification
	if s.auditService != nil {
		oldVal := fmt.Sprintf("%.2f", wallet.Balance)
		newVal := fmt.Sprintf("%.2f", newBalance)
		_ = s.auditService.Record(ctx, &userID, "user", "deposit_credited", "wallet", &wallet.ID, "", &oldVal, &newVal)
	}

	if s.realtimeSvc != nil {
		_ = s.realtimeSvc.Publish(ctx, "wallet_update", userID, "", map[string]interface{}{
			"balance":  newBalance,
			"currency": wallet.Currency,
		})
		_ = s.realtimeSvc.Publish(ctx, "transaction_update", userID, "", walletTx)
		_ = s.realtimeSvc.Publish(ctx, "deposit_update", userID, "", deposit)
	}

	return deposit, walletTx, nil
}

func (s *WalletService) ExtractReceiptTransactionID(ctx context.Context, method string, file multipart.File, contentType string) (string, error) {
	extraction, err := s.ExtractReceiptDetails(ctx, method, file, contentType)
	if err != nil {
		return "", err
	}
	return extraction.TransactionID, nil
}

// ExtractReceiptDetails reads the transaction number, and where available the
// amount and receiver, from an uploaded receipt image.
func (s *WalletService) ExtractReceiptDetails(ctx context.Context, method string, file multipart.File, contentType string) (*verifier.ReceiptExtraction, error) {
	if s.verificationManager != nil && s.verificationManager.ImageExtractor() != nil {
		imageBytes, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		return s.verificationManager.ImageExtractor().ExtractDetails(ctx, method, imageBytes, contentType)
	}
	if s.receiptVerifier != nil {
		transactionID, err := s.receiptVerifier.ExtractTransactionID(ctx, method, file, contentType)
		if err != nil {
			return nil, err
		}
		return &verifier.ReceiptExtraction{TransactionID: transactionID, Source: "gemini"}, nil
	}
	return nil, fmt.Errorf("receipt verifier is not configured")
}

func (s *WalletService) VerifyAndDepositReceipt(ctx context.Context, userID, method, transactionID string) (*domain.WalletTransaction, *infrastructure.ReceiptInfo, error) {
	// If verification manager is configured, use the robust new workflow
	if s.verificationManager != nil && s.depositRepo != nil {
		dep, tx, err := s.VerifyDirect(ctx, userID, method, transactionID)
		if err == nil && tx != nil {
			info := &infrastructure.ReceiptInfo{
				PaymentMethod:  dep.PaymentMethod,
				Amount:         tx.Amount,
				TransactionRef: tx.Reference,
				ReceiverName:   dep.ReceiverName,
				Status:         "Completed",
				ReceiptURL:     dep.ReceiptURL,
			}
			return tx, info, nil
		}
		if err != nil {
			return nil, nil, err
		}
	}

	if s.receiptVerifier == nil {
		return nil, nil, fmt.Errorf("receipt verifier is not configured")
	}

	cleanID := strings.TrimSpace(transactionID)
	if cleanID == "" {
		return nil, nil, fmt.Errorf("transaction ID is required")
	}

	// Idempotency check: verify this transaction reference has not been credited already
	exists, err := s.repo.ExistsByReference(ctx, cleanID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check existing transactions: %w", err)
	}
	if exists {
		return nil, nil, fmt.Errorf("transaction %s has already been processed", cleanID)
	}

	// Verify payment with external provider (Telebirr or CBE)
	receiptInfo, err := s.receiptVerifier.Verify(ctx, method, cleanID)
	if err != nil {
		return nil, nil, fmt.Errorf("verification failed: %w", err)
	}

	if receiptInfo.Amount <= 0 {
		return nil, nil, fmt.Errorf("invalid receipt amount: %.2f", receiptInfo.Amount)
	}

	// Double check the normalized reference from provider if it differs from raw input
	if receiptInfo.TransactionRef != "" && receiptInfo.TransactionRef != cleanID {
		exists, err = s.repo.ExistsByReference(ctx, receiptInfo.TransactionRef)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to check existing transactions: %w", err)
		}
		if exists {
			return nil, nil, fmt.Errorf("transaction %s has already been processed", receiptInfo.TransactionRef)
		}
	}

	// Process deposit
	ref := receiptInfo.TransactionRef
	if ref == "" {
		ref = cleanID
	}
	methodDisplay := "Telebirr"
	if strings.ToLower(receiptInfo.PaymentMethod) == "cbe" {
		methodDisplay = "CBE"
	}
	desc := fmt.Sprintf("Deposit via %s (%s)", methodDisplay, ref)

	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	newBalance := wallet.Balance + receiptInfo.Amount
	tx := &domain.WalletTransaction{
		ID:            generateID(),
		WalletID:      wallet.ID,
		UserID:        userID,
		Type:          "deposit",
		Status:        "completed",
		Amount:        receiptInfo.Amount,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  newBalance,
		Reference:     ref,
		Description:   desc,
		Metadata: map[string]interface{}{
			"payment_method": receiptInfo.PaymentMethod,
			"receipt_url":    receiptInfo.ReceiptURL,
			"receiver_name":  receiptInfo.ReceiverName,
			"transfer_date":  receiptInfo.TransferDate,
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.CreateTransaction(ctx, tx); err != nil {
		return nil, nil, fmt.Errorf("failed to record transaction: %w", err)
	}
	if err := s.repo.UpdateBalance(ctx, wallet.ID, newBalance); err != nil {
		return nil, nil, fmt.Errorf("failed to update wallet balance: %w", err)
	}

	if s.auditService != nil {
		oldVal := fmt.Sprintf("%.2f", wallet.Balance)
		newVal := fmt.Sprintf("%.2f", newBalance)
		_ = s.auditService.Record(ctx, &userID, "user", "receipt_deposit", "wallet", &wallet.ID, "", &oldVal, &newVal)
	}

	if s.realtimeSvc != nil {
		_ = s.realtimeSvc.Publish(ctx, "wallet_update", userID, "", map[string]interface{}{
			"balance":  newBalance,
			"currency": wallet.Currency,
		})
		_ = s.realtimeSvc.Publish(ctx, "transaction_update", userID, "", tx)
	}

	return tx, receiptInfo, nil
}

func (s *WalletService) VerifyDirect(ctx context.Context, userID, method, tokenOrRef string) (*domain.Deposit, *domain.WalletTransaction, error) {
	// Helper creating and executing an immediate flex deposit intent
	clean := strings.TrimSpace(tokenOrRef)
	dep, err := s.CreateDepositIntent(ctx, userID, 0.0, method)
	if err != nil {
		return nil, nil, err
	}

	return s.VerifyDeposit(ctx, dep.ID, userID, clean, "", nil, "")
}

func generateID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

