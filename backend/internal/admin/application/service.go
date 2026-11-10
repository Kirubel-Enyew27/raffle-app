package application

import (
	"context"
	"strings"
	"time"

	"github.com/raffle-app/backend/internal/admin/domain"
)

const (
	defaultLimit = 30
	maxLimit     = 100
)

type AdminService struct {
	repo domain.AdminRepository
}

func NewAdminService(repo domain.AdminRepository) *AdminService {
	return &AdminService{repo: repo}
}

func normalizeRange(from, to time.Time) (time.Time, time.Time) {
	if to.IsZero() {
		to = time.Now().UTC()
	}
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	if from.IsZero() {
		from = to.AddDate(0, 0, -30)
	}
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	if !from.Before(to) {
		from = to.AddDate(0, 0, -1)
	}
	return from, to
}

func normalizePeriod(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "weekly", "week":
		return "week"
	case "monthly", "month":
		return "month"
	default:
		return "day"
	}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func percentChange(current, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	value := (current - previous) / previous * 100
	return &value
}

func (s *AdminService) Overview(ctx context.Context, from, to time.Time, period string) (*domain.Overview, error) {
	from, to = normalizeRange(from, to)
	period = normalizePeriod(period)
	overview, err := s.repo.Overview(ctx, from, to, period)
	if err != nil {
		return nil, err
	}
	games, err := s.repo.Games(ctx, from, to, period)
	if err != nil {
		return nil, err
	}
	overview.Games = games
	ensureOverviewCollections(overview)
	return overview, nil
}

// ensureOverviewCollections keeps the JSON contract stable for the dashboard:
// Go marshals a nil slice as null, and the charts call .length on these arrays.
// Every collection must therefore be an empty array when there is no data.
func ensureOverviewCollections(overview *domain.Overview) {
	if overview.RegistrationTrend == nil {
		overview.RegistrationTrend = []domain.RegistrationPoint{}
	}
	if overview.MatchTrend == nil {
		overview.MatchTrend = []domain.MatchPoint{}
	}
	if overview.FinanceTrend == nil {
		overview.FinanceTrend = []domain.FinancePoint{}
	}
	if overview.RevenueTrend == nil {
		overview.RevenueTrend = []domain.RevenuePoint{}
	}
	if overview.Games == nil {
		overview.Games = []domain.GameSummary{}
	}
	for i := range overview.Games {
		if overview.Games[i].Trend == nil {
			overview.Games[i].Trend = []domain.GameTrendPoint{}
		}
	}
}

func (s *AdminService) Games(ctx context.Context, from, to time.Time, period string) ([]domain.GameSummary, error) {
	from, to = normalizeRange(from, to)
	games, err := s.repo.Games(ctx, from, to, normalizePeriod(period))
	if err != nil {
		return nil, err
	}
	if games == nil {
		games = []domain.GameSummary{}
	}
	return games, nil
}

func (s *AdminService) Users(ctx context.Context, filter domain.UserFilter) (domain.Page[domain.UserRow], error) {
	filter.Limit = clampLimit(filter.Limit)
	filter.Offset = clampOffset(filter.Offset)
	items, total, err := s.repo.Users(ctx, filter)
	if err != nil {
		return domain.Page[domain.UserRow]{}, err
	}
	return domain.Page[domain.UserRow]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *AdminService) UserDetail(ctx context.Context, id string) (*domain.UserDetail, error) {
	return s.repo.UserDetail(ctx, id)
}

func (s *AdminService) Matches(ctx context.Context, filter domain.MatchFilter) (domain.Page[domain.MatchRow], error) {
	filter.Limit = clampLimit(filter.Limit)
	filter.Offset = clampOffset(filter.Offset)
	items, total, err := s.repo.Matches(ctx, filter)
	if err != nil {
		return domain.Page[domain.MatchRow]{}, err
	}
	return domain.Page[domain.MatchRow]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *AdminService) Deposits(ctx context.Context, filter domain.FinanceFilter) (domain.Page[domain.DepositRow], error) {
	filter.Limit = clampLimit(filter.Limit)
	filter.Offset = clampOffset(filter.Offset)
	items, total, err := s.repo.Deposits(ctx, filter)
	if err != nil {
		return domain.Page[domain.DepositRow]{}, err
	}
	return domain.Page[domain.DepositRow]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *AdminService) Withdrawals(ctx context.Context, filter domain.FinanceFilter) (domain.Page[domain.WithdrawalRow], error) {
	filter.Limit = clampLimit(filter.Limit)
	filter.Offset = clampOffset(filter.Offset)
	items, total, err := s.repo.Withdrawals(ctx, filter)
	if err != nil {
		return domain.Page[domain.WithdrawalRow]{}, err
	}
	return domain.Page[domain.WithdrawalRow]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *AdminService) Live(ctx context.Context) (*domain.LiveSnapshot, error) {
	return s.repo.Live(ctx)
}
