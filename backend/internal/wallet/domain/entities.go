package domain

import "time"

type Wallet struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WalletTransaction struct {
	ID            string                 `json:"id"`
	WalletID      string                 `json:"wallet_id"`
	UserID        string                 `json:"user_id"`
	Type          string                 `json:"type"`
	Status        string                 `json:"status"`
	Amount        float64                `json:"amount"`
	BalanceBefore float64                `json:"balance_before"`
	BalanceAfter  float64                `json:"balance_after"`
	Reference     string                 `json:"reference"`
	Description   string                 `json:"description"`
	Metadata      map[string]interface{}   `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

const (
	DepositStatusPending      = "pending"
	DepositStatusVerifying    = "verifying"
	DepositStatusVerified     = "verified"
	DepositStatusCredited     = "credited"
	DepositStatusRejected     = "rejected"
	DepositStatusFailed       = "failed"
	DepositStatusManualReview = "manual_review"
	DepositStatusExpired      = "expired"
)

type Deposit struct {
	ID                   string                 `json:"id"`
	UserID               string                 `json:"user_id"`
	WalletID             string                 `json:"wallet_id"`
	Amount               float64                `json:"amount"`
	Currency             string                 `json:"currency"`
	PaymentMethod        string                 `json:"payment_method"`
	Status               string                 `json:"status"`
	ExternalReference    string                 `json:"external_reference,omitempty"`
	ReceiptToken         string                 `json:"receipt_token,omitempty"`
	ReceiptURL           string                 `json:"receipt_url,omitempty"`
	PayerName            string                 `json:"payer_name,omitempty"`
	PayerAccount         string                 `json:"payer_account,omitempty"`
	ReceiverName         string                 `json:"receiver_name,omitempty"`
	ReceiverAccount      string                 `json:"receiver_account,omitempty"`
	ActualAmountCredited float64                `json:"actual_amount_credited,omitempty"`
	FeeAmount            float64                `json:"fee_amount,omitempty"`
	ErrorMessage         string                 `json:"error_message,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	VerifiedAt           *time.Time             `json:"verified_at,omitempty"`
	CreditedAt           *time.Time             `json:"credited_at,omitempty"`
	ExpiresAt            time.Time              `json:"expires_at"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

type VerificationRequest struct {
	Method           string
	TransactionRef   string
	ReceiptToken     string
	RawURL           string
	ExpectedAmount   float64
	ExpectedReceiver string
}

type VerificationResult struct {
	Provider        string                 `json:"provider"`
	TransactionID   string                 `json:"transaction_id"`
	Status          string                 `json:"status"`
	AmountCredited  float64                `json:"amount_credited"`
	AmountDebited   float64                `json:"amount_debited"`
	FeeAmount       float64                `json:"fee_amount"`
	Currency        string                 `json:"currency"`
	ReceiverName    string                 `json:"receiver_name"`
	ReceiverAccount string                 `json:"receiver_account"`
	PayerName       string                 `json:"payer_name"`
	PayerAccount    string                 `json:"payer_account"`
	TransactionTime time.Time              `json:"transaction_time"`
	ReceiptURL      string                 `json:"receipt_url"`
	RawPayload      map[string]interface{} `json:"raw_payload,omitempty"`
}
