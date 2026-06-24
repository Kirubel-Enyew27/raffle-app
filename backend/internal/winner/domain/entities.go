package domain

import (
	"time"
)

type Winner struct {
	ID               string     `json:"id"`
	RaffleID         string     `json:"raffle_id"`
	DrawID           string     `json:"draw_id"`
	TicketID         string     `json:"ticket_id"`
	UserID           string     `json:"user_id"`
	PrizeAmount      float64    `json:"prize_amount"`
	PrizePaid        bool       `json:"prize_paid"`
	PaymentDate      *time.Time `json:"payment_date,omitempty"`
	PaymentReference string     `json:"payment_reference,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type WinnerDetail struct {
	Winner
	RaffleTitle   string    `json:"raffle_title"`
	TicketNumber  int       `json:"ticket_number"`
	UserEmail     string    `json:"user_email"`
	DrawTimestamp time.Time `json:"draw_timestamp"`
	DrawProof     DrawProof `json:"draw_proof"`
}

type DrawProof struct {
	CommitHash      string `json:"commit_hash"`
	ServerSeedHash  string `json:"server_seed_hash"`
	RevealedSeed    string `json:"revealed_seed"`
	CombinedHash    string `json:"combined_hash"`
	WinningNumber   int    `json:"winning_number"`
	VerificationURL string `json:"verification_url"`
}

// ProcessDrawInput contains everything needed to record a draw result and create a winner.
type ProcessDrawInput struct {
	RaffleID        string
	DrawID          string
	WinningTicketID string
	WinningUserID   string
	PrizeAmount     float64
	DrawTimestamp   time.Time
	Proof           DrawProof
}

// WinningTicket is the winning ticket with its owner details.
type WinningTicket struct {
	TicketID      string    `json:"ticket_id"`
	TicketNumber  int       `json:"ticket_number"`
	RaffleID      string    `json:"raffle_id"`
	UserID        string    `json:"user_id"`
	UserEmail     string    `json:"user_email"`
	DrawTimestamp time.Time `json:"draw_timestamp"`
}

// DrawVerification holds all data needed to independently verify a draw.
type DrawVerification struct {
	DrawID          string    `json:"draw_id"`
	RaffleID        string    `json:"raffle_id"`
	DrawTimestamp   time.Time `json:"draw_timestamp"`
	CommitHash      string    `json:"commit_hash"`
	ServerSeedHash  string    `json:"server_seed_hash"`
	RevealedSeed    string    `json:"revealed_seed"`
	CombinedHash    string    `json:"combined_hash"`
	WinningNumber   int       `json:"winning_number"`
	VerificationURL string    `json:"verification_url"`
	WinnerID        string    `json:"winner_id"`
	WinningTicketID string    `json:"winning_ticket_id"`
}

type Raffle struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type Draw struct {
	ID            string    `json:"id"`
	RaffleID      string    `json:"raffle_id"`
	DrawTimestamp time.Time `json:"draw_timestamp"`
	Status        string    `json:"status"`
	WinningTicket string    `json:"winning_ticket"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Ticket struct {
	ID           string `json:"id"`
	RaffleID     string `json:"raffle_id"`
	UserID       string `json:"user_id"`
	TicketNumber int    `json:"ticket_number"`
}
