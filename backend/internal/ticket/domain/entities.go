package domain

import "time"

type Ticket struct {
	ID          string    `json:"id"`
	RaffleID    string    `json:"raffle_id"`
	UserID      string    `json:"user_id"`
	WalletTxID  string    `json:"wallet_tx_id"`
	TicketNumber int     `json:"ticket_number"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PurchaseTicketsInput struct {
	Quantity   int    `json:"quantity" binding:"required,gt=0"`
	RaffleID   string `json:"raffle_id" binding:"required"`
	UserID     string `json:"user_id"`
	WalletTxID string `json:"wallet_tx_id"`
	IdempotencyKey string `json:"idempotency_key"`
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

type PurchaseResult struct {
	Tickets    []*Ticket
	WalletTxID string
	TotalSpent float64
}
