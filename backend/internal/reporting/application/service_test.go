package application

import (
	"context"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/reporting/domain"
)

// --- in-memory fake ---

type fakeRepo struct {
	revenue    []domain.RevenueRow
	tickets    []domain.TicketSalesRow
	users      []domain.ActiveUsersRow
	winners    []domain.WinnerSummaryRow
	profit     *domain.ProfitSummary
}

func (f *fakeRepo) Revenue(_ context.Context, _ domain.Period, _ domain.Filter) ([]domain.RevenueRow, int, error) {
	return f.revenue, len(f.revenue), nil
}
func (f *fakeRepo) TicketSales(_ context.Context, _ domain.Period, _ domain.Filter) ([]domain.TicketSalesRow, int, error) {
	return f.tickets, len(f.tickets), nil
}
func (f *fakeRepo) ActiveUsers(_ context.Context, _ domain.Period, _ domain.Filter) ([]domain.ActiveUsersRow, int, error) {
	return f.users, len(f.users), nil
}
func (f *fakeRepo) WinnerSummary(_ context.Context, _ domain.Filter) ([]domain.WinnerSummaryRow, int, error) {
	return f.winners, len(f.winners), nil
}
func (f *fakeRepo) ProfitSummary(_ context.Context, _ domain.Filter) (*domain.ProfitSummary, error) {
	return f.profit, nil
}

func newSvc(repo domain.ReportRepository) *ReportService { return NewReportService(repo) }

// --- tests ---

func TestRevenue_DefaultFilter(t *testing.T) {
	repo := &fakeRepo{revenue: []domain.RevenueRow{
		{Period: "2026-06-01", TicketRevenue: 100, DepositVolume: 200, Profit: 100},
	}}
	svc := newSvc(repo)
	page, err := svc.Revenue(context.Background(), domain.PeriodDaily, domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 row, got %d", len(page.Items))
	}
	if page.Items[0].TicketRevenue != 100 {
		t.Errorf("unexpected revenue: %f", page.Items[0].TicketRevenue)
	}
	if page.Limit != defaultLimit {
		t.Errorf("expected default limit %d, got %d", defaultLimit, page.Limit)
	}
}

func TestRevenue_LimitClamped(t *testing.T) {
	svc := newSvc(&fakeRepo{})
	page, err := svc.Revenue(context.Background(), domain.PeriodMonthly, domain.Filter{Limit: 9999})
	if err != nil {
		t.Fatal(err)
	}
	if page.Limit != maxLimit {
		t.Errorf("expected limit clamped to %d, got %d", maxLimit, page.Limit)
	}
}

func TestTicketSales(t *testing.T) {
	repo := &fakeRepo{tickets: []domain.TicketSalesRow{
		{Period: "2026-06-01", TicketsSold: 50, RafflesHeld: 2},
	}}
	svc := newSvc(repo)
	page, err := svc.TicketSales(context.Background(), domain.PeriodWeekly, domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].TicketsSold != 50 {
		t.Errorf("expected 50 tickets sold, got %d", page.Items[0].TicketsSold)
	}
}

func TestActiveUsers(t *testing.T) {
	repo := &fakeRepo{users: []domain.ActiveUsersRow{{Period: "2026-06-01", ActiveUsers: 42}}}
	svc := newSvc(repo)
	page, err := svc.ActiveUsers(context.Background(), domain.PeriodDaily, domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].ActiveUsers != 42 {
		t.Errorf("expected 42 active users, got %d", page.Items[0].ActiveUsers)
	}
}

func TestWinnerSummary(t *testing.T) {
	now := time.Now()
	repo := &fakeRepo{winners: []domain.WinnerSummaryRow{
		{WinnerID: "w-1", UserEmail: "a@b.com", PrizeAmount: 500, PrizePaid: true, CreatedAt: now},
	}}
	svc := newSvc(repo)
	page, err := svc.WinnerSummary(context.Background(), domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].PrizeAmount != 500 {
		t.Errorf("expected prize 500, got %f", page.Items[0].PrizeAmount)
	}
}

func TestProfitSummary(t *testing.T) {
	repo := &fakeRepo{profit: &domain.ProfitSummary{
		TotalTicketRevenue:  1000,
		TotalPrizePaid:      400,
		NetProfit:           600,
		TotalDepositVolume:  2000,
		TotalWithdrawVolume: 500,
		TotalTicketsSold:    100,
		TotalWinners:        5,
		TotalActiveUsers:    80,
	}}
	svc := newSvc(repo)
	s, err := svc.ProfitSummary(context.Background(), domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if s.NetProfit != 600 {
		t.Errorf("expected net profit 600, got %f", s.NetProfit)
	}
	if s.TotalActiveUsers != 80 {
		t.Errorf("expected 80 active users, got %d", s.TotalActiveUsers)
	}
}

func TestDefaultDateRange(t *testing.T) {
	// When no dates given the service should set From ~30 days before To.
	var capturedFilter domain.Filter
	repo := &captureRepo{capture: func(f domain.Filter) { capturedFilter = f }}
	svc := newSvc(repo)
	svc.Revenue(context.Background(), domain.PeriodDaily, domain.Filter{})

	diff := capturedFilter.To.Sub(capturedFilter.From)
	days := diff.Hours() / 24
	if days < 29 || days > 31 {
		t.Errorf("expected ~30 day range, got %.1f days", days)
	}
}

// captureRepo captures the filter passed to Revenue for inspection.
type captureRepo struct {
	capture func(domain.Filter)
}

func (c *captureRepo) Revenue(_ context.Context, _ domain.Period, f domain.Filter) ([]domain.RevenueRow, int, error) {
	c.capture(f)
	return nil, 0, nil
}
func (c *captureRepo) TicketSales(_ context.Context, _ domain.Period, _ domain.Filter) ([]domain.TicketSalesRow, int, error) {
	return nil, 0, nil
}
func (c *captureRepo) ActiveUsers(_ context.Context, _ domain.Period, _ domain.Filter) ([]domain.ActiveUsersRow, int, error) {
	return nil, 0, nil
}
func (c *captureRepo) WinnerSummary(_ context.Context, _ domain.Filter) ([]domain.WinnerSummaryRow, int, error) {
	return nil, 0, nil
}
func (c *captureRepo) ProfitSummary(_ context.Context, _ domain.Filter) (*domain.ProfitSummary, error) {
	return &domain.ProfitSummary{}, nil
}
