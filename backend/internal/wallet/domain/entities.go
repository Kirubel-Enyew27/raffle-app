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
