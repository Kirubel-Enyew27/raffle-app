package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/reporting/application"
	"github.com/raffle-app/backend/internal/reporting/domain"
)

func init() { gin.SetMode(gin.TestMode) }

// --- stub repo ---

type stubRepo struct {
	profit *domain.ProfitSummary
}

func (s *stubRepo) Revenue(_ context.Context, _ domain.Period, f domain.Filter) ([]domain.RevenueRow, int, error) {
	return []domain.RevenueRow{{Period: "2026-06-01", TicketRevenue: 200, Profit: 100}}, 1, nil
}
func (s *stubRepo) TicketSales(_ context.Context, _ domain.Period, f domain.Filter) ([]domain.TicketSalesRow, int, error) {
	return []domain.TicketSalesRow{{Period: "2026-06-01", TicketsSold: 10}}, 1, nil
}
func (s *stubRepo) ActiveUsers(_ context.Context, _ domain.Period, f domain.Filter) ([]domain.ActiveUsersRow, int, error) {
	return []domain.ActiveUsersRow{{Period: "2026-06-01", ActiveUsers: 5}}, 1, nil
}
func (s *stubRepo) WinnerSummary(_ context.Context, f domain.Filter) ([]domain.WinnerSummaryRow, int, error) {
	now := time.Now()
	return []domain.WinnerSummaryRow{{WinnerID: "w-1", UserEmail: "a@b.com", PrizeAmount: 500, CreatedAt: now}}, 1, nil
}
func (s *stubRepo) ProfitSummary(_ context.Context, f domain.Filter) (*domain.ProfitSummary, error) {
	if s.profit != nil {
		return s.profit, nil
	}
	return &domain.ProfitSummary{NetProfit: 999}, nil
}

func newRouter() *gin.Engine {
	svc := application.NewReportService(&stubRepo{})
	h := NewReportHandler(svc)
	r := gin.New()
	g := r.Group("/api/v1")
	RegisterReportRoutes(g, h)
	return r
}

func get(t *testing.T, r *gin.Engine, path string) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer token")
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

func TestRevenueHandler(t *testing.T) {
	code, resp := get(t, newRouter(), "/api/v1/reports/revenue?period=daily&from=2026-01-01&to=2026-06-30")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if resp["code"] != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %v", resp["code"])
	}
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestTicketSalesHandler(t *testing.T) {
	code, resp := get(t, newRouter(), "/api/v1/reports/tickets?period=weekly")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if resp["code"] != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %v", resp["code"])
	}
}

func TestActiveUsersHandler(t *testing.T) {
	code, resp := get(t, newRouter(), "/api/v1/reports/active-users?period=monthly")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if resp["code"] != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %v", resp["code"])
	}
}

func TestWinnerSummaryHandler(t *testing.T) {
	code, resp := get(t, newRouter(), "/api/v1/reports/winners?limit=10")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestProfitSummaryHandler(t *testing.T) {
	code, resp := get(t, newRouter(), "/api/v1/reports/profit?from=2026-01-01&to=2026-06-30")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	data := resp["data"].(map[string]interface{})
	if data["net_profit"] == nil {
		t.Error("expected net_profit in response")
	}
}

func TestPaginationParams(t *testing.T) {
	code, resp := get(t, newRouter(), "/api/v1/reports/revenue?limit=5&offset=10")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	data := resp["data"].(map[string]interface{})
	if int(data["offset"].(float64)) != 10 {
		t.Errorf("expected offset 10, got %v", data["offset"])
	}
}

func TestPeriodQueryParam(t *testing.T) {
	for _, p := range []string{"daily", "weekly", "monthly"} {
		code, _ := get(t, newRouter(), "/api/v1/reports/revenue?period="+p)
		if code != http.StatusOK {
			t.Errorf("period=%s: expected 200, got %d", p, code)
		}
	}
}
