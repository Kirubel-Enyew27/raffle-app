package domain

import (
	"time"
)

type Winner struct {
	ID               string
	RaffleID         string
	DrawID           string
	TicketID         string
	UserID           string
	PrizeAmount      float64
	PrizePaid        bool
	PaymentDate      *time.Time
	PaymentReference string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type WinnerDetail struct {
	Winner
	RaffleTitle    string
	TicketNumber   int
	UserEmail      string
	DrawTimestamp  time.Time
	DrawProof      DrawProof
}

type DrawProof struct {
	CommitHash       string
	ServerSeedHash   string
	RevealedSeed     string
	CombinedHash     string
	WinningNumber    int
	VerificationURL  string
}

type Raffle struct {
	ID       string
	Title    string
	Status   string
}

type Draw struct {
	ID            string
	RaffleID      string
	DrawTimestamp time.Time
	Status        string
	WinningTicket string
}

type User struct {
	ID    string
	Email string
}

type Ticket struct {
	ID           string
	RaffleID     string
	UserID       string
	TicketNumber int
}
