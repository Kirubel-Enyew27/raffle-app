package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	auditdomain "github.com/raffle-app/backend/internal/audit/domain"
	"github.com/raffle-app/backend/internal/raffle/domain"
)

type mockRaffleRepo struct {
	raffles map[string]*domain.Raffle
}

func newMockRaffleRepo() *mockRaffleRepo {
	return &mockRaffleRepo{raffles: make(map[string]*domain.Raffle)}
}

func (m *mockRaffleRepo) Create(ctx context.Context, raffle *domain.Raffle) error {
	m.raffles[raffle.ID] = raffle
	return nil
}

func (m *mockRaffleRepo) FindByID(ctx context.Context, id string) (*domain.Raffle, error) {
	r, ok := m.raffles[id]
	if !ok {
		return nil, errors.New("raffle not found")
	}
	copy := *r
	return &copy, nil
}

func (m *mockRaffleRepo) FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*domain.Raffle, error) {
	return m.FindByID(ctx, id)
}

func (m *mockRaffleRepo) Update(ctx context.Context, raffle *domain.Raffle) error {
	m.raffles[raffle.ID] = raffle
	return nil
}

func (m *mockRaffleRepo) Delete(ctx context.Context, id string) error {
	delete(m.raffles, id)
	return nil
}

func (m *mockRaffleRepo) List(ctx context.Context, limit, offset int) ([]*domain.Raffle, error) {
	var result []*domain.Raffle
	for _, r := range m.raffles {
		result = append(result, r)
	}
	return result, nil
}

func (m *mockRaffleRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.raffles)), nil
}

type mockAuditRepo struct {
	logs []auditdomain.AuditLog
}

func (m *mockAuditRepo) Create(ctx context.Context, log *auditdomain.AuditLog) error {
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) FindByID(ctx context.Context, id string) (*auditdomain.AuditLog, error) {
	return nil, nil
}

func (m *mockAuditRepo) FindByFilter(ctx context.Context, filter auditdomain.AuditLogFilter) ([]auditdomain.AuditLog, int, error) {
	return nil, 0, nil
}

func (m *mockAuditRepo) Count(ctx context.Context, filter auditdomain.AuditLogFilter) (int, error) {
	return 0, nil
}

func (m *mockAuditRepo) DeleteOlderThan(ctx context.Context, cutoffDate time.Time) (int64, error) {
	return 0, nil
}

func validRaffle() *domain.Raffle {
	return &domain.Raffle{
		ID:           "raffle-1",
		Title:        "Test Raffle",
		Description:  "A test raffle",
		TicketPrice:   10.0,
		TotalTickets:  100,
		DrawDate:      time.Now().Add(24 * time.Hour),
		CreatorID:     "user-1",
		Status:        "draft",
		PrizePool:     500.0,
		Currency:      "USD",
	}
}

func TestCreateRaffle_Success(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	auditRepo := &mockAuditRepo{}
	auditService := auditapp.NewAuditService(auditRepo)

	svc := NewRaffleService(raffleRepo, auditService)

	raffle := validRaffle()
	err := svc.CreateRaffle(context.Background(), raffle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raffleRepo.raffles[raffle.ID] == nil {
		t.Error("raffle should be persisted")
	}

	if len(auditRepo.logs) != 1 {
		t.Errorf("expected 1 audit log, got %d", len(auditRepo.logs))
	}
}

func TestCreateRaffle_EmptyTitle(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Title = ""

	err := svc.CreateRaffle(context.Background(), raffle)
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestCreateRaffle_ZeroPrice(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.TicketPrice = 0

	err := svc.CreateRaffle(context.Background(), raffle)
	if err == nil {
		t.Fatal("expected error for zero ticket price")
	}
}

func TestCreateRaffle_NegativePrice(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.TicketPrice = -10

	err := svc.CreateRaffle(context.Background(), raffle)
	if err == nil {
		t.Fatal("expected error for negative ticket price")
	}
}

func TestCreateRaffle_ZeroTickets(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.TotalTickets = 0

	err := svc.CreateRaffle(context.Background(), raffle)
	if err == nil {
		t.Fatal("expected error for zero total tickets")
	}
}

func TestCreateRaffle_MissingDrawDate(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.DrawDate = time.Time{}

	err := svc.CreateRaffle(context.Background(), raffle)
	if err == nil {
		t.Fatal("expected error for missing draw date")
	}
}

func TestCreateRaffle_MissingCreatorID(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.CreatorID = ""

	err := svc.CreateRaffle(context.Background(), raffle)
	if err == nil {
		t.Fatal("expected error for missing creator id")
	}
}

func TestCreateRaffle_InvalidStatus(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Status = "invalid"

	err := svc.CreateRaffle(context.Background(), raffle)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestGetRaffle_Success(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffleRepo.raffles[raffle.ID] = raffle

	result, err := svc.GetRaffle(context.Background(), raffle.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != raffle.Title {
		t.Errorf("expected title %s, got %s", raffle.Title, result.Title)
	}
}

func TestGetRaffle_NotFound(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	_, err := svc.GetRaffle(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent raffle")
	}
}

func TestListRaffles_Success(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffleRepo.raffles["r1"] = validRaffle()
	raffleRepo.raffles["r2"] = validRaffle()

	raffles, count, err := svc.ListRaffles(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
	if len(raffles) != 2 {
		t.Errorf("expected 2 raffles, got %d", len(raffles))
	}
}

func TestUpdateRaffle_Success(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Status = "active"
	raffleRepo.raffles[raffle.ID] = raffle

	update := &domain.Raffle{
		ID:    raffle.ID,
		Title: "Updated Title",
		Status: "active",
	}

	err := svc.UpdateRaffle(context.Background(), update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := raffleRepo.raffles[raffle.ID]
	if updated.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %s", updated.Title)
	}
	if updated.Status != "active" {
		t.Errorf("expected status 'active', got %s", updated.Status)
	}
}

func TestUpdateRaffle_MissingID(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	err := svc.UpdateRaffle(context.Background(), &domain.Raffle{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestUpdateRaffle_NotFound(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	err := svc.UpdateRaffle(context.Background(), &domain.Raffle{ID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent raffle")
	}
}

func TestUpdateRaffle_InvalidStatus(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffleRepo.raffles[raffle.ID] = raffle

	err := svc.UpdateRaffle(context.Background(), &domain.Raffle{
		ID:     raffle.ID,
		Status: "invalid_status",
	})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestCloseRaffle_Success(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Status = "active"
	raffleRepo.raffles[raffle.ID] = raffle

	err := svc.CloseRaffle(context.Background(), raffle.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	closed := raffleRepo.raffles[raffle.ID]
	if closed.Status != "closed" {
		t.Errorf("expected status 'closed', got %s", closed.Status)
	}
}

func TestCloseRaffle_AlreadyClosed(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Status = "closed"
	raffleRepo.raffles[raffle.ID] = raffle

	err := svc.CloseRaffle(context.Background(), raffle.ID)
	if err == nil {
		t.Fatal("expected error for already closed raffle")
	}
}

func TestCloseRaffle_NotFound(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	err := svc.CloseRaffle(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent raffle")
	}
}

func TestScheduleDrawDate_Success(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Status = "active"
	raffle.DrawDate = time.Now().Add(-time.Hour)
	raffleRepo.raffles[raffle.ID] = raffle

	newDate := time.Now().Add(48 * time.Hour)
	err := svc.ScheduleDrawDate(context.Background(), raffle.ID, newDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scheduled := raffleRepo.raffles[raffle.ID]
	if scheduled.Status != "scheduled" {
		t.Errorf("expected status 'scheduled', got %s", scheduled.Status)
	}
}

func TestScheduleDrawDate_ClosedRaffle(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Status = "closed"
	raffleRepo.raffles[raffle.ID] = raffle

	newDate := time.Now().Add(48 * time.Hour)
	err := svc.ScheduleDrawDate(context.Background(), raffle.ID, newDate)
	if err == nil {
		t.Fatal("expected error for closed raffle")
	}
}

func TestScheduleDrawDate_PastDate(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	raffle := validRaffle()
	raffle.Status = "active"
	raffleRepo.raffles[raffle.ID] = raffle

	newDate := time.Now().Add(-time.Hour)
	err := svc.ScheduleDrawDate(context.Background(), raffle.ID, newDate)
	if err == nil {
		t.Fatal("expected error for past draw date")
	}
}

func TestScheduleDrawDate_NotFound(t *testing.T) {
	raffleRepo := newMockRaffleRepo()
	svc := NewRaffleService(raffleRepo, nil)

	newDate := time.Now().Add(48 * time.Hour)
	err := svc.ScheduleDrawDate(context.Background(), "nonexistent", newDate)
	if err == nil {
		t.Fatal("expected error for nonexistent raffle")
	}
}
