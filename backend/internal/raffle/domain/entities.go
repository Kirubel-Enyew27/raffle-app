package domain

import "time"

type Raffle struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	TicketPrice  float64   `json:"ticket_price"`
	TotalTickets int       `json:"total_tickets"`
	SoldTickets  int       `json:"sold_tickets"`
	Status       string    `json:"status"`
	DrawDate     time.Time `json:"draw_date"`
	CreatorID    string    `json:"creator_id"`
	PrizePool    float64   `json:"prize_pool"`
	Currency     string    `json:"currency"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
