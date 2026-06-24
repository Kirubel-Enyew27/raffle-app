package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/winner/application"
	"github.com/raffle-app/backend/internal/winner/domain"
)

func init() { gin.SetMode(gin.TestMode) }

// --- mocks ---

type mockWinnerRepo struct {
	winners map[string]*domain.Winner
}

func newMockWinnerRepo() *mockWinnerRepo {
	return &mockWinnerRepo{winners: make(map[string]*domain.Winner)}
}

func (m *mockWinnerRepo) Create(ctx context.Context, w *domain.Winner) error {
	w.ID = "w-1"
	m.winners[w.ID] = w
	return nil
}
func (m *mockWinnerRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]domain.Winner, error) {
	var res []domain.Winner
	for _, w := range m.winners {
		if w.RaffleID == raffleID {
			res = append(res, *w)
		}
	}
	return res, nil
}
func (m *mockWinnerRepo) FindByDrawID(ctx context.Context, drawID string) ([]domain.Winner, error) {
	return nil, nil
}
func (m *mockWinnerRepo) FindByID(ctx context.Context, id string) (*domain.Winner, error) {
	w, ok := m.winners[id]
	if !ok {
		return nil, errors.New("winner not found")
	}
	return w, nil
}
func (m *mockWinnerRepo) MarkPrizePaid(ctx context.Context, id string, paymentDate time.Time, paymentReference string) error {
	w, ok := m.winners[id]
	if !ok {
		return errors.New("not found")
	}
	w.PrizePaid = true
	w.PaymentDate = &paymentDate
	w.PaymentReference = paymentReference
	return nil
}
func (m *mockWinnerRepo) ExistsByDrawIDAndTicketID(ctx context.Context, drawID, ticketID string) (bool, error) {
	return false, nil
}

func (m *mockWinnerRepo) FindAll(ctx context.Context, limit, offset int, paidOnly *bool) ([]domain.Winner, int, error) {
	var list []domain.Winner
	for _, w := range m.winners {
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

type stubRepo struct{}

func (s *stubRepo) FindByID(ctx context.Context, id string) (*domain.Raffle, error) {
	return &domain.Raffle{ID: id, Title: "Test Raffle"}, nil
}

type stubDrawRepo struct{}

func (s *stubDrawRepo) FindByRaffleID(ctx context.Context, raffleID string) (*domain.Draw, error) {
	return &domain.Draw{ID: "draw-1", RaffleID: raffleID, DrawTimestamp: time.Now()}, nil
}
func (s *stubDrawRepo) GetProofByRaffleID(ctx context.Context, raffleID string) (*domain.DrawProof, error) {
	return &domain.DrawProof{VerificationURL: "/api/v1/draw/verify"}, nil
}

type stubUserRepo struct{}

func (s *stubUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Email: "user@example.com"}, nil
}

type stubTicketRepo struct{}

func (s *stubTicketRepo) FindByID(ctx context.Context, id string) (*domain.Ticket, error) {
	return &domain.Ticket{ID: id, TicketNumber: 7}, nil
}

func newTestHandler() *WinnerHandler {
	svc := application.NewWinnerService(
		newMockWinnerRepo(),
		&stubRepo{},
		&stubDrawRepo{},
		&stubUserRepo{},
		&stubTicketRepo{},
		nil,
	)
	return NewWinnerHandler(svc)
}

func newTestHandlerWithRepo(repo *mockWinnerRepo) *WinnerHandler {
	svc := application.NewWinnerService(repo, &stubRepo{}, &stubDrawRepo{}, &stubUserRepo{}, &stubTicketRepo{}, nil)
	return NewWinnerHandler(svc)
}

// --- tests ---

func TestGetWinnersByRaffle_Empty(t *testing.T) {
	r := gin.New()
	h := newTestHandler()
	r.GET("/winners/raffle/:raffle_id", h.GetWinnersByRaffle)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/winners/raffle/raffle-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetWinnerDetail_NotFound(t *testing.T) {
	r := gin.New()
	h := newTestHandler()
	r.GET("/winners/:id", h.GetWinnerDetail)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/winners/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetWinnerDetail_Found(t *testing.T) {
	repo := newMockWinnerRepo()
	repo.winners["w-1"] = &domain.Winner{
		ID: "w-1", RaffleID: "r-1", DrawID: "d-1", TicketID: "t-1", UserID: "u-1", PrizeAmount: 100,
	}

	r := gin.New()
	h := newTestHandlerWithRepo(repo)
	r.GET("/winners/:id", h.GetWinnerDetail)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/winners/w-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["code"] != "SUCCESS" {
		t.Errorf("expected code SUCCESS, got %v", resp["code"])
	}
}

func TestMarkPrizePaid_MissingBody(t *testing.T) {
	r := gin.New()
	h := newTestHandler()
	r.POST("/winners/:id/paid", h.MarkPrizePaid)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/winners/w-1/paid", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestMarkPrizePaid_Success(t *testing.T) {
	repo := newMockWinnerRepo()
	repo.winners["w-1"] = &domain.Winner{
		ID: "w-1", RaffleID: "r-1", DrawID: "d-1", TicketID: "t-1", UserID: "u-1",
		PrizeAmount: 100, PrizePaid: false,
	}

	r := gin.New()
	h := newTestHandlerWithRepo(repo)
	r.POST("/winners/:id/paid", h.MarkPrizePaid)

	body, _ := json.Marshal(map[string]string{"payment_reference": "tx-abc"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/winners/w-1/paid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] != "PRIZE_PAID" {
		t.Errorf("expected code PRIZE_PAID, got %v", resp["code"])
	}
}

func TestMarkPrizePaid_AlreadyPaid(t *testing.T) {
	repo := newMockWinnerRepo()
	repo.winners["w-1"] = &domain.Winner{
		ID: "w-1", PrizePaid: true,
	}

	r := gin.New()
	h := newTestHandlerWithRepo(repo)
	r.POST("/winners/:id/paid", h.MarkPrizePaid)

	body, _ := json.Marshal(map[string]string{"payment_reference": "tx-abc"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/winners/w-1/paid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
