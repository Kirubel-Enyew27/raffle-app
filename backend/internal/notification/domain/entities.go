package domain

import "time"

// Channel is the delivery channel for a notification.
type Channel string

const (
	ChannelEmail  Channel = "email"
	ChannelInApp  Channel = "in_app"
)

// EventType identifies what triggered the notification.
type EventType string

const (
	EventRegistration   EventType = "registration"
	EventDeposit        EventType = "deposit"
	EventWithdrawal     EventType = "withdrawal"
	EventTicketPurchase EventType = "ticket_purchase"
	EventDrawAnnounce   EventType = "draw_announcement"
	EventWinner         EventType = "winner_announcement"
	EventPrizePaid      EventType = "prize_payment"
)

// Status of a notification record.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSent      Status = "sent"
	StatusFailed    Status = "failed"
)

// Notification is the persisted record of one delivery attempt.
type Notification struct {
	ID         string
	UserID     string
	Channel    Channel
	Event      EventType
	Subject    string
	Body       string
	Status     Status
	Retries    int
	Error      string
	ReadAt     *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Payload carries the data needed to render a notification.
// All fields are optional; only those relevant to the event need to be set.
type Payload struct {
	UserEmail    string
	UserName     string
	Amount       float64
	RaffleTitle  string
	TicketCount  int
	DrawDate     string
	PrizeAmount  float64
	PaymentRef   string
}
