package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/winner/domain"
)

type mockWinnerRepo struct {
	winners map[string]*domain.Winner
}

func newMockWinnerRepo() *mockWinnerRepo {
	return &mockWinnerRepo{winners: make(map[string]*domain.Winner)}
}

func (m *mockWinnerRepo) Create(ctx context.Context, winner *domain.Winner) error {
	winner.ID = "winner-" + winner.RaffleID
	winner.CreatedAt = time.Now()
	winner.UpdatedAt = time.Now()
	key := winner.DrawID + ":" + winner.TicketID
	m.winners[key] = winner
	return nil
}

func (m *mockWinnerRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]domain.Winner, error) {
	var result []domain.Winner
	for _, w := range m.winners {
		if w.RaffleID == raffleID {
			result = append(result, *w)
		}
	}
	return result, nil
}

func (m *mockWinnerRepo) FindByDrawID(ctx context.Context, drawID string) ([]domain.Winner, error) {
	var result []domain.Winner
	for _, w := range m.winners {
		if w.DrawID == drawID {
			result = append(result, *w)
		}
	}
	return result, nil
}

func (m *mockWinnerRepo) FindByID(ctx context.Context, id string) (*domain.Winner, error) {
	for _, w := range m.winners {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, errors.New("winner not found")
}

func (m *mockWinnerRepo) MarkPrizePaid(ctx context.Context, id string, paymentDate time.Time, paymentReference string) error {
	for _, w := range m.winners {
		if w.ID == id {
			w.PrizePaid = true
			w.PaymentDate = &paymentDate
			w.PaymentReference = paymentReference
			w.UpdatedAt = time.Now()
			return nil
		}
	}
	return errors.New("winner not found")
}

func (m *mockWinnerRepo) ExistsByDrawIDAndTicketID(ctx context.Context, drawID, ticketID string) (bool, error) {
	key := drawID + ":" + ticketID
	_, ok := m.winners[key]
	return ok, nil
}

type mockRaffleRepo struct {
	raffles map[string]*domain.Raffle
}

func newMockRaffleRepo() *mockRaffleRepo {
	return &mockRaffleRepo{raffles: make(map[string]*domain.Raffle)}
}

func (m *mockRaffleRepo) FindByID(ctx context.Context, id string) (*domain.Raffle, error) {
	r, ok := m.raffles[id]
	if !ok {
		return nil, errors.New("raffle not found")
	}
	return r, nil
}

type mockDrawRepo struct {
	draws map[string]*domain.Draw
}

func newMockDrawRepo() *mockDrawRepo {
	return &mockDrawRepo{draws: make(map[string]*domain.Draw)}
}

func (m *mockDrawRepo) FindByRaffleID(ctx context.Context, raffleID string) (*domain.Draw, error) {
	for _, d := range m.draws {
		if d.RaffleID == raffleID {
			return d, nil
		}
	}
	return nil, errors.New("draw not found")
}

func (m *mockDrawRepo) GetProofByRaffleID(ctx context.Context, raffleID string) (*domain.DrawProof, error) {
	return &domain.DrawProof{
		CommitHash:      "abcdef",
		VerificationURL: "/api/v1/draw/verify",
	}, nil
}

type mockUserRepo struct {
	users map[string]*domain.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*domain.User)}
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

type mockTicketRepo struct {
	tickets map[string]*domain.Ticket
}

func newMockTicketRepo() *mockTicketRepo {
	return &mockTicketRepo{tickets: make(map[string]*domain.Ticket)}
}

func (m *mockTicketRepo) FindByID(ctx context.Context, id string) (*domain.Ticket, error) {
	t, ok := m.tickets[id]
	if !ok {
		return nil, errors.New("ticket not found")
	}
	return t, nil
}

func TestCreateWinner_Success(t *testing.T) {
	winnerRepo := newMockWinnerRepo()
	raffleRepo := newMockRaffleRepo()
	drawRepo := newMockDrawRepo()
	userRepo := newMockUserRepo()
	ticketRepo := newMockTicketRepo()

	raffleRepo.raffles["raffle-1"] = &domain.Raffle{ID: "raffle-1", Title: "Test", Status: "active"}
	drawRepo.draws["draw-1"] = &domain.Draw{ID: "draw-1", RaffleID: "raffle-1", Status: "completed"}
	userRepo.users["user-1"] = &domain.User{ID: "user-1", Email: "test@example.com"}
	ticketRepo.tickets["ticket-1"] = &domain.Ticket{ID: "ticket-1", RaffleID: "raffle-1", UserID: "user-1", TicketNumber: 1}

	svc := NewWinnerService(winnerRepo, raffleRepo, drawRepo, userRepo, ticketRepo, nil)

	winner, err := svc.CreateWinner(context.Background(), "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if winner.PrizeAmount != 100.0 {
		t.Errorf("expected prize amount 100, got %f", winner.PrizeAmount)
	}
	if winner.PrizePaid {
		t.Error("winner should not be marked as paid initially")
	}
}

func TestCreateWinner_Duplicate(t *testing.T) {
	winnerRepo := newMockWinnerRepo()
	raffleRepo := newMockRaffleRepo()
	drawRepo := newMockDrawRepo()
	userRepo := newMockUserRepo()
	ticketRepo := newMockTicketRepo()

	raffleRepo.raffles["raffle-1"] = &domain.Raffle{ID: "raffle-1", Title: "Test", Status: "active"}
	drawRepo.draws["draw-1"] = &domain.Draw{ID: "draw-1", RaffleID: "raffle-1", Status: "completed"}
	userRepo.users["user-1"] = &domain.User{ID: "user-1", Email: "test@example.com"}
	ticketRepo.tickets["ticket-1"] = &domain.Ticket{ID: "ticket-1", RaffleID: "raffle-1", UserID: "user-1", TicketNumber: 1}

	svc := NewWinnerService(winnerRepo, raffleRepo, drawRepo, userRepo, ticketRepo, nil)

	_, err := svc.CreateWinner(context.Background(), "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.CreateWinner(context.Background(), "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err == nil {
		t.Fatal("expected error for duplicate winner")
	}
}

func TestMarkPrizePaid(t *testing.T) {
	winnerRepo := newMockWinnerRepo()
	raffleRepo := newMockRaffleRepo()
	drawRepo := newMockDrawRepo()
	userRepo := newMockUserRepo()
	ticketRepo := newMockTicketRepo()

	raffleRepo.raffles["raffle-1"] = &domain.Raffle{ID: "raffle-1", Title: "Test", Status: "active"}
	drawRepo.draws["draw-1"] = &domain.Draw{ID: "draw-1", RaffleID: "raffle-1", Status: "completed"}
	userRepo.users["user-1"] = &domain.User{ID: "user-1", Email: "test@example.com"}
	ticketRepo.tickets["ticket-1"] = &domain.Ticket{ID: "ticket-1", RaffleID: "raffle-1", UserID: "user-1", TicketNumber: 1}

	svc := NewWinnerService(winnerRepo, raffleRepo, drawRepo, userRepo, ticketRepo, nil)

	winner, err := svc.CreateWinner(context.Background(), "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err != nil {
		t.Fatal(err)
	}

	paid, err := svc.MarkPrizePaid(context.Background(), winner.ID, "tx-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !paid.PrizePaid {
		t.Error("winner should be marked as paid")
	}
	if paid.PaymentReference != "tx-123" {
		t.Errorf("expected payment reference tx-123, got %s", paid.PaymentReference)
	}

	_, err = svc.MarkPrizePaid(context.Background(), winner.ID, "tx-456")
	if err == nil {
		t.Fatal("expected error when marking already paid winner")
	}
}

func TestGetWinnersByRaffle(t *testing.T) {
	winnerRepo := newMockWinnerRepo()
	raffleRepo := newMockRaffleRepo()
	drawRepo := newMockDrawRepo()
	userRepo := newMockUserRepo()
	ticketRepo := newMockTicketRepo()

	raffleRepo.raffles["raffle-1"] = &domain.Raffle{ID: "raffle-1", Title: "Test Raffle", Status: "active"}
	drawRepo.draws["draw-1"] = &domain.Draw{ID: "draw-1", RaffleID: "raffle-1", Status: "completed", WinningTicket: "ticket-1"}
	userRepo.users["user-1"] = &domain.User{ID: "user-1", Email: "winner@example.com"}
	ticketRepo.tickets["ticket-1"] = &domain.Ticket{ID: "ticket-1", RaffleID: "raffle-1", UserID: "user-1", TicketNumber: 42}

	svc := NewWinnerService(winnerRepo, raffleRepo, drawRepo, userRepo, ticketRepo, nil)

	_, err := svc.CreateWinner(context.Background(), "raffle-1", "draw-1", "ticket-1", "user-1", 200.0)
	if err != nil {
		t.Fatal(err)
	}

	details, err := svc.GetWinnersByRaffle(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 winner detail, got %d", len(details))
	}
	if details[0].RaffleTitle != "Test Raffle" {
		t.Errorf("expected raffle title 'Test Raffle', got %s", details[0].RaffleTitle)
	}
	if details[0].UserEmail != "winner@example.com" {
		t.Errorf("expected user email 'winner@example.com', got %s", details[0].UserEmail)
	}
	if details[0].TicketNumber != 42 {
		t.Errorf("expected ticket number 42, got %d", details[0].TicketNumber)
	}
}
