package domain

import "context"

// SMSLogEntry represents a log of a received and processed SMS.
type SMSLogEntry struct {
	ID                  string
	Sender              string
	RawMessage          string
	ParsedAmount        *float64
	ParsedSenderName    *string
	ParsedSenderPhone   *string
	ParsedTransactionID *string
	ParsedTimestamp     *string
	Credited            bool
	CreditedAmount      *float64
	CreditedUserID      *string
	CreditedWalletID    *string
	ReceiptVerified     bool
	ReceiptAmount       *float64
	ReceiptPayerName    *string
	ReceiptStatus       *string
	ErrorMessage        *string
	IPAddress           *string
}

// SMSLogRepository defines the persistence contract for SMS logs.
type SMSLogRepository interface {
	Create(ctx context.Context, entry *SMSLogEntry) error
	ExistsByTransactionID(ctx context.Context, transactionID string) (bool, error)
}
