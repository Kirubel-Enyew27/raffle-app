// Integration tests for the winner application layer.
// Uses in-memory fakes wired together end-to-end (no real DB required).
package application_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/winner/application"
	"github.com/raffle-app/backend/internal/winner/domain"
)

// ---- in-memory repository implementations ----

type memWinnerRepo struct {
	winners map[string]*domain.Winner
	seq     int
}

func newMemWinnerRepo() *memWinnerRepo {
	return &memWinnerRepo{winners: make(map[string]*domain.Winner)}
}

func (m *memWinnerRepo) Create(ctx context.Context, w *domain.Winner) error {
	m.seq++
	w.ID = "w-" + string(rune('0'+m.seq))
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	key := w.DrawID + ":" + w.TicketID
	if _, exists := m.winners[key]; exists {
		return errors.New("duplicate")
	}
	clone := *w
	m.winners[key] = &clone
	// also index by ID
	m.winners[w.ID] = &clone
	return nil
}

func (m *memWinnerRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]domain.Winner, error) {
	seen := map[string]bool{}
	var res []domain.Winner
	for _, w := range m.winners {
		if w.RaffleID == raffleID && !seen[w.ID] {
			res = append(res, *w)
			seen[w.ID] = true
		}
	}
	return res, nil
}

func (m *memWinnerRepo) FindByDrawID(ctx context.Context, drawID string) ([]domain.Winner, error) {
	seen := map[string]bool{}
	var res []domain.Winner
	for _, w := range m.winners {
		if w.DrawID == drawID && !seen[w.ID] {
			res = append(res, *w)
			seen[w.ID] = true
		}
	}
	return res, nil
}

func (m *memWinnerRepo) FindByID(ctx context.Context, id string) (*domain.Winner, error) {
	w, ok := m.winners[id]
	if !ok {
		return nil, errors.New("winner not found")
	}
	clone := *w
	return &clone, nil
}

func (m *memWinnerRepo) MarkPrizePaid(ctx context.Context, id string, paymentDate time.Time, ref string) error {
	w, ok := m.winners[id]
	if !ok {
		return errors.New("winner not found")
	}
	w.PrizePaid = true
	w.PaymentDate = &paymentDate
	w.PaymentReference = ref
	w.UpdatedAt = time.Now()
	return nil
}

func (m *memWinnerRepo) ExistsByDrawIDAndTicketID(ctx context.Context, drawID, ticketID string) (bool, error) {
	_, ok := m.winners[drawID+":"+ticketID]
	return ok, nil
}

func (m *memWinnerRepo) FindAll(ctx context.Context, limit, offset int, paidOnly *bool) ([]domain.Winner, int, error) {
	var list []domain.Winner
	seen := make(map[string]bool)
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
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	if offset >= len(list) {
		return []domain.Winner{}, total, nil
	}
	end := offset + limit
	if end > len(list) || limit <= 0 {
		end = len(list)
	}
	return list[offset:end], total, nil
}

type memRaffleRepo struct{}

func (r *memRaffleRepo) FindByID(_ context.Context, id string) (*domain.Raffle, error) {
	return &domain.Raffle{ID: id, Title: "Grand Raffle", Status: "completed"}, nil
}

type memDrawRepo struct{}

func (r *memDrawRepo) FindByRaffleID(_ context.Context, raffleID string) (*domain.Draw, error) {
	return &domain.Draw{ID: "draw-1", RaffleID: raffleID, DrawTimestamp: time.Now(), Status: "completed"}, nil
}
func (r *memDrawRepo) GetProofByRaffleID(_ context.Context, raffleID string) (*domain.DrawProof, error) {
	return &domain.DrawProof{CommitHash: "abc", VerificationURL: "/api/v1/draw/verify"}, nil
}

type memUserRepo struct{}

func (r *memUserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Email: "winner@example.com"}, nil
}

type memTicketRepo struct{}

func (r *memTicketRepo) FindByID(_ context.Context, id string) (*domain.Ticket, error) {
	return &domain.Ticket{ID: id, RaffleID: "raffle-1", UserID: "user-1", TicketNumber: 42}, nil
}

func newService() (*application.WinnerService, *memWinnerRepo) {
	repo := newMemWinnerRepo()
	svc := application.NewWinnerService(repo, &memRaffleRepo{}, &memDrawRepo{}, &memUserRepo{}, &memTicketRepo{}, nil)
	return svc, repo
}

// ---- tests ----

func TestIntegration_CreateAndRetrieveWinner(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	w, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 500.0)
	if err != nil {
		t.Fatalf("CreateWinner: %v", err)
	}
	if w.ID == "" {
		t.Fatal("expected winner ID to be set")
	}

	detail, err := svc.GetWinnerByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetWinnerByID: %v", err)
	}
	if detail.PrizeAmount != 500.0 {
		t.Errorf("expected prize 500, got %f", detail.PrizeAmount)
	}
	if detail.UserEmail != "winner@example.com" {
		t.Errorf("expected email winner@example.com, got %s", detail.UserEmail)
	}
	if detail.TicketNumber != 42 {
		t.Errorf("expected ticket number 42, got %d", detail.TicketNumber)
	}
	if detail.RaffleTitle != "Grand Raffle" {
		t.Errorf("expected raffle title 'Grand Raffle', got %s", detail.RaffleTitle)
	}
}

func TestIntegration_PreventDuplicateWinner(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	if _, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0); err != nil {
		t.Fatalf("first CreateWinner: %v", err)
	}
	_, err := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestIntegration_InvalidPrizeAmount(t *testing.T) {
	svc, _ := newService()
	_, err := svc.CreateWinner(context.Background(), "raffle-1", "draw-1", "ticket-1", "user-1", 0)
	if err == nil {
		t.Fatal("expected error for zero prize amount")
	}
}

func TestIntegration_ListWinnersByRaffle(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 100.0)

	details, err := svc.GetWinnersByRaffle(ctx, "raffle-1")
	if err != nil {
		t.Fatalf("GetWinnersByRaffle: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 winner, got %d", len(details))
	}
	if details[0].TicketNumber != 42 {
		t.Errorf("expected ticket number 42, got %d", details[0].TicketNumber)
	}
}

func TestIntegration_MarkPrizePaid(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	w, _ := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 200.0)

	paid, err := svc.MarkPrizePaid(ctx, w.ID, "tx-ref-001")
	if err != nil {
		t.Fatalf("MarkPrizePaid: %v", err)
	}
	if !paid.PrizePaid {
		t.Error("expected PrizePaid=true")
	}
	if paid.PaymentReference != "tx-ref-001" {
		t.Errorf("expected ref tx-ref-001, got %s", paid.PaymentReference)
	}
	if paid.PaymentDate == nil {
		t.Error("expected PaymentDate to be set")
	}
}

func TestIntegration_MarkPrizePaid_AlreadyPaid(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	w, _ := svc.CreateWinner(ctx, "raffle-1", "draw-1", "ticket-1", "user-1", 200.0)
	svc.MarkPrizePaid(ctx, w.ID, "tx-1")

	_, err := svc.MarkPrizePaid(ctx, w.ID, "tx-2")
	if err == nil {
		t.Fatal("expected error for already-paid winner")
	}
}

func TestIntegration_MarkPrizePaid_WinnerNotFound(t *testing.T) {
	svc, _ := newService()
	_, err := svc.MarkPrizePaid(context.Background(), "nonexistent", "tx-x")
	if err == nil {
		t.Fatal("expected error for nonexistent winner")
	}
}
