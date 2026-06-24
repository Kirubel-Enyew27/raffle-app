package application

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/winner/domain"
)

// --- mocks ---

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
	m.winners[winner.ID] = winner
	return nil
}

func (m *mockWinnerRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]domain.Winner, error) {
	seen := map[string]bool{}
	var result []domain.Winner
	for _, w := range m.winners {
		if w.RaffleID == raffleID && !seen[w.ID] {
			result = append(result, *w)
			seen[w.ID] = true
		}
	}
	return result, nil
}

func (m *mockWinnerRepo) FindByDrawID(ctx context.Context, drawID string) ([]domain.Winner, error) {
	seen := map[string]bool{}
	var result []domain.Winner
	for _, w := range m.winners {
		if w.DrawID == drawID && !seen[w.ID] {
			result = append(result, *w)
			seen[w.ID] = true
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

func (m *mockWinnerRepo) FindAll(ctx context.Context, limit, offset int, paidOnly *bool) ([]domain.Winner, int, error) {
	var list []domain.Winner
	seen := map[string]bool{}
	for _, w := range m.winners {
		if seen[w.ID] {
			continue
		}
		seen[w.ID] = true
		if paidOnly != nil {
			if *paidOnly && !w.PrizePaid {
				continue
			}
			if !*paidOnly && w.PrizePaid {
				continue
			}
		}
		list = append(list, *w)
	}
	total := len(list)
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	if offset >= len(list) {
		return []domain.Winner{}, total, nil
	}
	end := offset + limit
	if end > len(list) || limit <= 0 {
		end = len(list)
	}
	return list[offset:end], total, nil
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
	draws  map[string]*domain.Draw
	proofs map[string]*domain.DrawProof
}

func newMockDrawRepo() *mockDrawRepo {
	return &mockDrawRepo{
		draws:  make(map[string]*domain.Draw),
		proofs: make(map[string]*domain.DrawProof),
	}
}

func (m *mockDrawRepo) FindByID(ctx context.Context, drawID string) (*domain.Draw, error) {
	d, ok := m.draws[drawID]
	if !ok {
		return nil, errors.New("draw not found")
	}
	return d, nil
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
	if p, ok := m.proofs[raffleID]; ok {
		return p, nil
	}
	return &domain.DrawProof{CommitHash: "abcdef", ServerSeedHash: "abcdef", VerificationURL: "/api/v1/draw/verify"}, nil
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

func newTestService() (*WinnerService, *mockWinnerRepo, *mockRaffleRepo, *mockDrawRepo, *mockUserRepo, *mockTicketRepo) {
	wr := newMockWinnerRepo()
	rr := newMockRaffleRepo()
	dr := newMockDrawRepo()
	ur := newMockUserRepo()
	tr := newMockTicketRepo()

	rr.raffles["raffle-1"] = &domain.Raffle{ID: "raffle-1", Title: "Test", Status: "active"}
	dr.draws["draw-1"] = &domain.Draw{ID: "draw-1", RaffleID: "raffle-1", Status: "completed", WinningTicket: "ticket-1"}
	ur.users["user-1"] = &domain.User{ID: "user-1", Email: "test@example.com"}
	tr.tickets["ticket-1"] = &domain.Ticket{ID: "ticket-1", RaffleID: "raffle-1", UserID: "user-1", TicketNumber: 1}

	svc := NewWinnerService(wr, rr, dr, ur, tr, nil)
	return svc, wr, rr, dr, ur, tr
}

// --- ProcessDrawResult tests ---

func TestProcessDrawResult_Success(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	winner, err := svc.ProcessDrawResult(context.Background(), domain.ProcessDrawInput{
		RaffleID:        "raffle-1",
		DrawID:          "draw-1",
		WinningTicketID: "ticket-1",
		WinningUserID:   "user-1",
		PrizeAmount:     100.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if winner.PrizeAmount != 100.0 {
		t.Errorf("expected prize 100, got %f", winner.PrizeAmount)
	}
	if winner.PrizePaid {
		t.Error("winner should not be paid initially")
	}
	if winner.DrawID != "draw-1" {
		t.Errorf("expected drawID draw-1, got %s", winner.DrawID)
	}
}

func TestProcessDrawResult_InvalidPrize(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	_, err := svc.ProcessDrawResult(context.Background(), domain.ProcessDrawInput{
		RaffleID: "raffle-1", DrawID: "draw-1",
		WinningTicketID: "ticket-1", WinningUserID: "user-1",
		PrizeAmount: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero prize")
	}
}

func TestProcessDrawResult_Duplicate(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	input := domain.ProcessDrawInput{
		RaffleID: "raffle-1", DrawID: "draw-1",
		WinningTicketID: "ticket-1", WinningUserID: "user-1",
		PrizeAmount: 100.0,
	}
	if _, err := svc.ProcessDrawResult(ctx, input); err != nil {
		t.Fatal(err)
	}
	_, err := svc.ProcessDrawResult(ctx, input)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestProcessDrawResult_MissingFields(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	_, err := svc.ProcessDrawResult(context.Background(), domain.ProcessDrawInput{
		RaffleID: "raffle-1", DrawID: "draw-1",
		WinningTicketID: "", WinningUserID: "user-1",
		PrizeAmount: 100.0,
	})
	if err == nil {
		t.Fatal("expected error for missing ticket ID")
	}
}

// --- CreateWinner (delegates to ProcessDrawResult) ---

func TestCreateWinner_Success(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	winner, err := svc.CreateWinner(context.Background(), "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if winner.PrizeAmount != 100.0 {
		t.Errorf("expected prize 100, got %f", winner.PrizeAmount)
	}
}

func TestCreateWinner_Duplicate(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	if _, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err == nil {
		t.Fatal("expected error for duplicate winner")
	}
}

// --- GetWinningTicket ---

func TestGetWinningTicket_Success(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	winner, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err != nil {
		t.Fatal(err)
	}

	ticket, err := svc.GetWinningTicket(ctx, winner.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.TicketID != "ticket-1" {
		t.Errorf("expected ticket-1, got %s", ticket.TicketID)
	}
	if ticket.TicketNumber != 1 {
		t.Errorf("expected ticket number 1, got %d", ticket.TicketNumber)
	}
	if ticket.UserEmail != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", ticket.UserEmail)
	}
}

func TestGetWinningTicket_WinnerNotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	_, err := svc.GetWinningTicket(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent winner")
	}
}

// --- GetDrawVerification ---

func TestGetDrawVerification_Success(t *testing.T) {
	svc, _, _, dr, _, _ := newTestService()
	ctx := context.Background()

	dr.proofs["raffle-1"] = &domain.DrawProof{
		CommitHash:      "commit-hash",
		ServerSeedHash:  "server-seed-hash",
		RevealedSeed:    "revealed-seed",
		CombinedHash:    "combined-hash",
		WinningNumber:   1,
		VerificationURL: "/api/v1/draw/verify",
	}

	winner, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err != nil {
		t.Fatal(err)
	}

	verification, err := svc.GetDrawVerification(ctx, winner.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verification.CommitHash != "commit-hash" {
		t.Errorf("expected commit-hash, got %s", verification.CommitHash)
	}
	if verification.ServerSeedHash != "server-seed-hash" {
		t.Errorf("expected server-seed-hash, got %s", verification.ServerSeedHash)
	}
	if verification.RevealedSeed != "revealed-seed" {
		t.Errorf("expected revealed-seed, got %s", verification.RevealedSeed)
	}
	if verification.WinnerID != winner.ID {
		t.Errorf("expected winner ID %s, got %s", winner.ID, verification.WinnerID)
	}
	if verification.WinningTicketID != "ticket-1" {
		t.Errorf("expected ticket-1, got %s", verification.WinningTicketID)
	}
}

func TestGetDrawVerification_WinnerNotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	_, err := svc.GetDrawVerification(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent winner")
	}
}

// --- MarkPrizePaid ---

func TestMarkPrizePaid(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	winner, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err != nil {
		t.Fatal(err)
	}

	paid, err := svc.MarkPrizePaid(ctx, winner.ID, "tx-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !paid.PrizePaid {
		t.Error("winner should be marked as paid")
	}
	if paid.PaymentReference != "tx-123" {
		t.Errorf("expected tx-123, got %s", paid.PaymentReference)
	}
	if paid.PaymentDate == nil {
		t.Error("expected payment date to be set")
	}

	_, err = svc.MarkPrizePaid(ctx, winner.ID, "tx-456")
	if err == nil {
		t.Fatal("expected error when paying already paid winner")
	}
}

// --- GetWinnersByRaffle ---

func TestGetWinnersByRaffle(t *testing.T) {
	svc, _, rr, dr, ur, tr := newTestService()

	rr.raffles["raffle-1"] = &domain.Raffle{ID: "raffle-1", Title: "Test Raffle", Status: "active"}
	dr.draws["draw-1"] = &domain.Draw{ID: "draw-1", RaffleID: "raffle-1", Status: "completed"}
	ur.users["user-1"] = &domain.User{ID: "user-1", Email: "winner@example.com"}
	tr.tickets["ticket-1"] = &domain.Ticket{ID: "ticket-1", RaffleID: "raffle-1", UserID: "user-1", TicketNumber: 42}

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
		t.Errorf("expected 'Test Raffle', got %s", details[0].RaffleTitle)
	}
	if details[0].UserEmail != "winner@example.com" {
		t.Errorf("expected winner@example.com, got %s", details[0].UserEmail)
	}
	if details[0].TicketNumber != 42 {
		t.Errorf("expected ticket number 42, got %d", details[0].TicketNumber)
	}
}
