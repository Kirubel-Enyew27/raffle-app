package domain

import "fmt"

// Render returns (subject, body) for the given event and payload.
func Render(event EventType, p Payload) (subject, body string) {
	switch event {
	case EventRegistration:
		return "Welcome to RaffleApp",
			fmt.Sprintf("Hi %s, your account has been created successfully.", p.UserName)
	case EventDeposit:
		return "Deposit Confirmed",
			fmt.Sprintf("Hi %s, your deposit of %.2f has been confirmed.", p.UserName, p.Amount)
	case EventWithdrawal:
		return "Withdrawal Processed",
			fmt.Sprintf("Hi %s, your withdrawal of %.2f has been processed.", p.UserName, p.Amount)
	case EventTicketPurchase:
		return fmt.Sprintf("Tickets Purchased – %s", p.RaffleTitle),
			fmt.Sprintf("Hi %s, you have purchased %d ticket(s) for \"%s\".", p.UserName, p.TicketCount, p.RaffleTitle)
	case EventDrawAnnounce:
		return fmt.Sprintf("Draw Announcement – %s", p.RaffleTitle),
			fmt.Sprintf("The draw for \"%s\" will take place on %s. Good luck!", p.RaffleTitle, p.DrawDate)
	case EventWinner:
		return fmt.Sprintf("Congratulations! You won – %s", p.RaffleTitle),
			fmt.Sprintf("Hi %s, you are the winner of \"%s\" with a prize of %.2f!", p.UserName, p.RaffleTitle, p.PrizeAmount)
	case EventPrizePaid:
		return "Prize Payment Confirmed",
			fmt.Sprintf("Hi %s, your prize of %.2f has been paid. Reference: %s.", p.UserName, p.PrizeAmount, p.PaymentRef)
	default:
		return "RaffleApp Notification", "You have a new notification."
	}
}
