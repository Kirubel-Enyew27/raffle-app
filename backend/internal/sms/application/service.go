package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	identitydomain "github.com/raffle-app/backend/internal/identity/domain"
	smsdomain "github.com/raffle-app/backend/internal/sms/domain"
	"github.com/raffle-app/backend/internal/sms/infrastructure"
	walletdomain "github.com/raffle-app/backend/internal/wallet/domain"
)

// UserRepository defines the user lookup methods the SMS service needs.
type UserRepository interface {
	FindByName(ctx context.Context, name string) (*identitydomain.User, error)
}

// WalletRepository defines the wallet operations the SMS service needs.
type WalletRepository interface {
	FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*walletdomain.Wallet, error)
	CreateTx(ctx context.Context, tx *sql.Tx, wallet *walletdomain.Wallet) error
	CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *walletdomain.WalletTransaction) error
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error
}

// SMSLogRepository defines persistence for SMS audit logs.
type SMSLogRepository interface {
	Create(ctx context.Context, entry *smsdomain.SMSLogEntry) error
	ExistsByTransactionID(ctx context.Context, transactionID string) (bool, error)
}

// ProcessResult is returned after processing an SMS webhook.
type ProcessResult struct {
	Credited      bool
	Amount        float64
	TransactionID string
	Verified      bool
	Error         error
}

// SMSService handles SMS-based wallet deposits via receipt verification.
// Flow: SMS trigger → extract transaction_id → fetch Telebirr receipt page →
// verify status & amount → find user by payer name → atomic wallet credit.
type SMSService struct {
	db             *sql.DB
	userRepo       UserRepository
	walletRepo     WalletRepository
	smsLogRepo     SMSLogRepository
	receiptFetcher *infrastructure.ReceiptFetcher
	apiKey         string
}

func NewSMSService(
	db *sql.DB,
	userRepo UserRepository,
	walletRepo WalletRepository,
	smsLogRepo SMSLogRepository,
	receiptFetcher *infrastructure.ReceiptFetcher,
	apiKey string,
) *SMSService {
	return &SMSService{
		db:             db,
		userRepo:       userRepo,
		walletRepo:     walletRepo,
		smsLogRepo:     smsLogRepo,
		receiptFetcher: receiptFetcher,
		apiKey:         apiKey,
	}
}

// APIKey returns the configured SMS API key for middleware validation.
func (s *SMSService) APIKey() string {
	return s.apiKey
}

// ProcessWebhook handles an incoming SMS webhook from the SMS Forwarder app.
//
// The SMS is only a trigger — the source of truth is the Telebirr receipt page.
// 1. Extract transaction_id from SMS
// 2. Fetch receipt page from Ethiotelecom
// 3. Verify transaction is completed, extract confirmed amount and payer name
// 4. Find user by payer name
// 5. Atomically credit wallet with verified amount
func (s *SMSService) ProcessWebhook(ctx context.Context, sender, message, ipAddress string) *ProcessResult {
	// Validate it's a Telebirr SMS
	if !smsdomain.IsTelebirrSMS(sender, message) {
		return &ProcessResult{
			Credited: false,
			Error:    fmt.Errorf("not a valid Telebirr SMS: sender=%q", sender),
		}
	}

	// Extract only the transaction ID from the SMS
	transactionID, err := smsdomain.ExtractTransactionID(message)
	if err != nil {
		return &ProcessResult{
			Credited: false,
			Error:    fmt.Errorf("failed to extract transaction ID from SMS: %w", err),
		}
	}

	// Check if this transaction ID has already been credited (idempotency check)
	exists, err := s.smsLogRepo.ExistsByTransactionID(ctx, transactionID)
	if err != nil {
		return &ProcessResult{
			TransactionID: transactionID,
			Error:         fmt.Errorf("failed to check duplicate transaction: %w", err),
		}
	}
	if exists {
		return &ProcessResult{
			Credited:      false,
			TransactionID: transactionID,
			Error:         fmt.Errorf("duplicate transaction: %s already processed", transactionID),
		}
	}

	// Fetch the official Telebirr receipt page to verify the transaction
	receipt, err := s.receiptFetcher.Fetch(transactionID)
	if err != nil {
		logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
		logEntry.Credited = false
		errMsg := fmt.Sprintf("receipt fetch failed: %v", err)
		logEntry.ErrorMessage = &errMsg
		_ = s.smsLogRepo.Create(ctx, logEntry)
		return &ProcessResult{
			TransactionID: transactionID,
			Error:         fmt.Errorf("receipt verification failed for %s: %w", transactionID, err),
		}
	}

	// Validate the receipt data (status must be completed, amount > 0, payer name present)
	if err := infrastructure.ValidateReceipt(receipt); err != nil {
		logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
		logEntry.Credited = false
		logEntry.ReceiptVerified = false
		logEntry.ReceiptAmount = &receipt.TotalPaidAmount
		logEntry.ReceiptPayerName = &receipt.PayerName
		logEntry.ReceiptStatus = &receipt.Status
		errMsg := fmt.Sprintf("receipt validation failed: %v", err)
		logEntry.ErrorMessage = &errMsg
		_ = s.smsLogRepo.Create(ctx, logEntry)
		return &ProcessResult{
			TransactionID: transactionID,
			Amount:        receipt.TotalPaidAmount,
			Verified:      true,
			Error:         fmt.Errorf("receipt validation failed: %w", err),
		}
	}

	// Find the user by payer name from the receipt page
	user, err := s.userRepo.FindByName(ctx, receipt.PayerName)
	if err != nil {
		logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
		logEntry.Credited = false
		logEntry.ReceiptVerified = true
		logEntry.ReceiptAmount = &receipt.TotalPaidAmount
		logEntry.ReceiptPayerName = &receipt.PayerName
		logEntry.ReceiptStatus = &receipt.Status
		errMsg := fmt.Sprintf("user not found for payer name %q: %v", receipt.PayerName, err)
		logEntry.ErrorMessage = &errMsg
		_ = s.smsLogRepo.Create(ctx, logEntry)
		return &ProcessResult{
			TransactionID: transactionID,
			Amount:        receipt.TotalPaidAmount,
			Verified:      true,
			Error:         fmt.Errorf("no user matches payer name %q", receipt.PayerName),
		}
	}

	// Atomic wallet credit within a DB transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return &ProcessResult{
			TransactionID: transactionID,
			Amount:        receipt.TotalPaidAmount,
			Verified:      true,
			Error:         fmt.Errorf("failed to begin transaction: %w", err),
		}
	}

	commitFailed := true
	defer func() {
		if commitFailed {
			tx.Rollback()
		}
	}()

	// Lock the wallet row for update (concurrency safety)
	wallet, err := s.walletRepo.FindByUserIDWithLock(ctx, tx, user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Auto-create wallet inside the transaction
			wallet = &walletdomain.Wallet{
				ID:        generateID(),
				UserID:    user.ID,
				Balance:   0,
				Currency:  "ETB",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := s.walletRepo.CreateTx(ctx, tx, wallet); err != nil {
				logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
				logEntry.Credited = false
				logEntry.ReceiptVerified = true
				logEntry.ReceiptAmount = &receipt.TotalPaidAmount
				logEntry.ReceiptPayerName = &receipt.PayerName
				logEntry.ReceiptStatus = &receipt.Status
				errMsg := fmt.Sprintf("failed to create wallet: %v", err)
				logEntry.ErrorMessage = &errMsg
				_ = s.smsLogRepo.Create(ctx, logEntry)
				return &ProcessResult{
					TransactionID: transactionID,
					Amount:        receipt.TotalPaidAmount,
					Verified:      true,
					Error:         fmt.Errorf("failed to create wallet: %w", err),
				}
			}
		} else {
			logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
			logEntry.Credited = false
			logEntry.ReceiptVerified = true
			logEntry.ReceiptAmount = &receipt.TotalPaidAmount
			logEntry.ReceiptPayerName = &receipt.PayerName
			logEntry.ReceiptStatus = &receipt.Status
			errMsg := fmt.Sprintf("failed to find wallet: %v", err)
			logEntry.ErrorMessage = &errMsg
			_ = s.smsLogRepo.Create(ctx, logEntry)
			return &ProcessResult{
				TransactionID: transactionID,
				Amount:        receipt.TotalPaidAmount,
				Verified:      true,
				Error:         fmt.Errorf("failed to find wallet: %w", err),
			}
		}
	}

	// Calculate new balance using the VERIFIED amount from the receipt
	newBalance := wallet.Balance + receipt.TotalPaidAmount

	// Create wallet transaction (reference is the transaction_id for dedup)
	walletTx := &walletdomain.WalletTransaction{
		ID:            generateID(),
		WalletID:      wallet.ID,
		UserID:        user.ID,
		Type:          "sms_deposit",
		Status:        "completed",
		Amount:        receipt.TotalPaidAmount,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  newBalance,
		Reference:     transactionID,
		Description:   fmt.Sprintf("Telebirr deposit from %s (verified via receipt)", receipt.PayerName),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.walletRepo.CreateTransactionTx(ctx, tx, walletTx); err != nil {
		logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
		logEntry.Credited = false
		logEntry.ReceiptVerified = true
		logEntry.ReceiptAmount = &receipt.TotalPaidAmount
		logEntry.ReceiptPayerName = &receipt.PayerName
		logEntry.ReceiptStatus = &receipt.Status
		errMsg := fmt.Sprintf("failed to create transaction: %v", err)
		logEntry.ErrorMessage = &errMsg
		_ = s.smsLogRepo.Create(ctx, logEntry)
		return &ProcessResult{
			TransactionID: transactionID,
			Amount:        receipt.TotalPaidAmount,
			Verified:      true,
			Error:         fmt.Errorf("failed to create wallet transaction: %w", err),
		}
	}

	// Update wallet balance
	if err := s.walletRepo.UpdateBalanceTx(ctx, tx, wallet.ID, newBalance); err != nil {
		logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
		logEntry.Credited = false
		logEntry.ReceiptVerified = true
		logEntry.ReceiptAmount = &receipt.TotalPaidAmount
		logEntry.ReceiptPayerName = &receipt.PayerName
		logEntry.ReceiptStatus = &receipt.Status
		errMsg := fmt.Sprintf("failed to update balance: %v", err)
		logEntry.ErrorMessage = &errMsg
		_ = s.smsLogRepo.Create(ctx, logEntry)
		return &ProcessResult{
			TransactionID: transactionID,
			Amount:        receipt.TotalPaidAmount,
			Verified:      true,
			Error:         fmt.Errorf("failed to update wallet balance: %w", err),
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return &ProcessResult{
			TransactionID: transactionID,
			Amount:        receipt.TotalPaidAmount,
			Verified:      true,
			Error:         fmt.Errorf("failed to commit transaction: %w", err),
		}
	}
	commitFailed = false

	// Log the successful SMS processing (outside the transaction)
	logEntry := buildLogEntry(sender, message, transactionID, ipAddress)
	logEntry.Credited = true
	logEntry.CreditedAmount = &receipt.TotalPaidAmount
	logEntry.CreditedUserID = &user.ID
	logEntry.CreditedWalletID = &wallet.ID
	logEntry.ReceiptVerified = true
	logEntry.ReceiptAmount = &receipt.TotalPaidAmount
	logEntry.ReceiptPayerName = &receipt.PayerName
	logEntry.ReceiptStatus = &receipt.Status
	if err := s.smsLogRepo.Create(ctx, logEntry); err != nil {
		fmt.Printf("WARNING: failed to create SMS log entry: %v\n", err)
	}

	return &ProcessResult{
		Credited:      true,
		Amount:        receipt.TotalPaidAmount,
		TransactionID: transactionID,
		Verified:      true,
	}
}

func buildLogEntry(sender, message, transactionID, ipAddress string) *smsdomain.SMSLogEntry {
	return &smsdomain.SMSLogEntry{
		ID:                  generateID(),
		Sender:              sender,
		RawMessage:          message,
		ParsedTransactionID: &transactionID,
		IPAddress:           &ipAddress,
	}
}

func generateID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}
