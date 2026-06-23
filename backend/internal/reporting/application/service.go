package application

import (
	"context"
	"time"

	"github.com/raffle-app/backend/internal/reporting/domain"
)

const (
	defaultLimit = 30
	maxLimit     = 200
)

type ReportService struct {
	repo domain.ReportRepository
}

func NewReportService(repo domain.ReportRepository) *ReportService {
	return &ReportService{repo: repo}
}

func (s *ReportService) clamp(f domain.Filter) domain.Filter {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}
	if f.To.IsZero() {
		f.To = time.Now()
	}
	if f.From.IsZero() {
		f.From = f.To.AddDate(0, -1, 0) // default: last 30 days
	}
	return f
}

func (s *ReportService) Revenue(ctx context.Context, period domain.Period, f domain.Filter) (domain.Page[domain.RevenueRow], error) {
	f = s.clamp(f)
	rows, total, err := s.repo.Revenue(ctx, period, f)
	if err != nil {
		return domain.Page[domain.RevenueRow]{}, err
	}
	return domain.Page[domain.RevenueRow]{Items: rows, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *ReportService) TicketSales(ctx context.Context, period domain.Period, f domain.Filter) (domain.Page[domain.TicketSalesRow], error) {
	f = s.clamp(f)
	rows, total, err := s.repo.TicketSales(ctx, period, f)
	if err != nil {
		return domain.Page[domain.TicketSalesRow]{}, err
	}
	return domain.Page[domain.TicketSalesRow]{Items: rows, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *ReportService) ActiveUsers(ctx context.Context, period domain.Period, f domain.Filter) (domain.Page[domain.ActiveUsersRow], error) {
	f = s.clamp(f)
	rows, total, err := s.repo.ActiveUsers(ctx, period, f)
	if err != nil {
		return domain.Page[domain.ActiveUsersRow]{}, err
	}
	return domain.Page[domain.ActiveUsersRow]{Items: rows, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *ReportService) WinnerSummary(ctx context.Context, f domain.Filter) (domain.Page[domain.WinnerSummaryRow], error) {
	f = s.clamp(f)
	rows, total, err := s.repo.WinnerSummary(ctx, f)
	if err != nil {
		return domain.Page[domain.WinnerSummaryRow]{}, err
	}
	return domain.Page[domain.WinnerSummaryRow]{Items: rows, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *ReportService) ProfitSummary(ctx context.Context, f domain.Filter) (*domain.ProfitSummary, error) {
	f = s.clamp(f)
	return s.repo.ProfitSummary(ctx, f)
}
