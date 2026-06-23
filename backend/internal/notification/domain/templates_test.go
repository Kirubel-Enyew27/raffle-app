package domain

import (
	"strings"
	"testing"
)

func TestRender_AllEvents(t *testing.T) {
	p := Payload{
		UserName:    "Alice",
		UserEmail:   "alice@example.com",
		Amount:      100.0,
		RaffleTitle: "Grand Raffle",
		TicketCount: 3,
		DrawDate:    "2026-07-01",
		PrizeAmount: 500.0,
		PaymentRef:  "tx-abc",
	}

	cases := []struct {
		event           EventType
		wantSubjectPart string
		wantBodyPart    string
	}{
		{EventRegistration, "Welcome", "Alice"},
		{EventDeposit, "Deposit", "100.00"},
		{EventWithdrawal, "Withdrawal", "100.00"},
		{EventTicketPurchase, "Grand Raffle", "3 ticket"},
		{EventDrawAnnounce, "Grand Raffle", "2026-07-01"},
		{EventWinner, "Congratulations", "500.00"},
		{EventPrizePaid, "Prize Payment", "tx-abc"},
	}

	for _, tc := range cases {
		subj, body := Render(tc.event, p)
		if !strings.Contains(subj, tc.wantSubjectPart) {
			t.Errorf("[%s] subject %q missing %q", tc.event, subj, tc.wantSubjectPart)
		}
		if !strings.Contains(body, tc.wantBodyPart) {
			t.Errorf("[%s] body %q missing %q", tc.event, body, tc.wantBodyPart)
		}
	}
}

func TestRender_UnknownEvent(t *testing.T) {
	subj, body := Render("unknown_event", Payload{})
	if subj == "" || body == "" {
		t.Error("expected non-empty fallback for unknown event")
	}
}
