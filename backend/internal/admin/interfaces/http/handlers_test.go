package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/admin/application"
	"github.com/raffle-app/backend/internal/admin/domain"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"github.com/spf13/viper"
)

type mockAdminRepo struct {
	overviewFn    func(ctx context.Context, from, to time.Time, period string) (*domain.Overview, error)
	gamesFn       func(ctx context.Context, from, to time.Time, period string) ([]domain.GameSummary, error)
	usersFn       func(ctx context.Context, filter domain.UserFilter) ([]domain.UserRow, int, error)
	userDetailFn  func(ctx context.Context, id string) (*domain.UserDetail, error)
	matchesFn     func(ctx context.Context, filter domain.MatchFilter) ([]domain.MatchRow, int, error)
	depositsFn    func(ctx context.Context, filter domain.FinanceFilter) ([]domain.DepositRow, int, error)
	withdrawalsFn func(ctx context.Context, filter domain.FinanceFilter) ([]domain.WithdrawalRow, int, error)
	liveFn        func(ctx context.Context) (*domain.LiveSnapshot, error)
}

func (m *mockAdminRepo) Overview(ctx context.Context, from, to time.Time, period string) (*domain.Overview, error) {
	if m.overviewFn != nil {
		return m.overviewFn(ctx, from, to, period)
	}
	return &domain.Overview{TotalUsers: 10, TotalMatches: 5}, nil
}

func (m *mockAdminRepo) Games(ctx context.Context, from, to time.Time, period string) ([]domain.GameSummary, error) {
	if m.gamesFn != nil {
		return m.gamesFn(ctx, from, to, period)
	}
	return []domain.GameSummary{{Slug: "tic-tac-toe", Title: "Tic-Tac-Toe", TotalMatches: 5}}, nil
}

func (m *mockAdminRepo) Users(ctx context.Context, filter domain.UserFilter) ([]domain.UserRow, int, error) {
	if m.usersFn != nil {
		return m.usersFn(ctx, filter)
	}
	return []domain.UserRow{{ID: "user-1", Email: "test@example.com"}}, 1, nil
}

func (m *mockAdminRepo) UserDetail(ctx context.Context, id string) (*domain.UserDetail, error) {
	if m.userDetailFn != nil {
		return m.userDetailFn(ctx, id)
	}
	if id == "00000000-0000-0000-0000-000000000000" {
		return nil, sql.ErrNoRows
	}
	return &domain.UserDetail{
		UserRow: domain.UserRow{ID: id, Email: "test@example.com"},
	}, nil
}

func (m *mockAdminRepo) Matches(ctx context.Context, filter domain.MatchFilter) ([]domain.MatchRow, int, error) {
	if m.matchesFn != nil {
		return m.matchesFn(ctx, filter)
	}
	return []domain.MatchRow{{ID: "match-1", GameSlug: "tic-tac-toe"}}, 1, nil
}

func (m *mockAdminRepo) Deposits(ctx context.Context, filter domain.FinanceFilter) ([]domain.DepositRow, int, error) {
	if m.depositsFn != nil {
		return m.depositsFn(ctx, filter)
	}
	return []domain.DepositRow{{ID: "dep-1", Amount: 100}}, 1, nil
}

func (m *mockAdminRepo) Withdrawals(ctx context.Context, filter domain.FinanceFilter) ([]domain.WithdrawalRow, int, error) {
	if m.withdrawalsFn != nil {
		return m.withdrawalsFn(ctx, filter)
	}
	return []domain.WithdrawalRow{{ID: "with-1", Amount: 50}}, 1, nil
}

func (m *mockAdminRepo) Live(ctx context.Context) (*domain.LiveSnapshot, error) {
	if m.liveFn != nil {
		return m.liveFn(ctx)
	}
	return &domain.LiveSnapshot{ActiveMatches: 1, LivePlayers: 2}, nil
}

const testJWTSecret = "test-jwt-secret-for-admin-handlers-suite-12345"

func setupAdminRouter(repo domain.AdminRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	viper.Set("JWT_SECRET", testJWTSecret)

	r := gin.New()
	api := r.Group("/api/v1")
	svc := application.NewAdminService(repo)
	handler := NewAdminHandler(svc)
	RegisterAdminRoutes(api, handler)
	return r
}

func makeToken(t *testing.T, role string) string {
	t.Helper()
	token, err := appjwt.GenerateTokenHMAC("test-user-id", "test@example.com", role, []byte(testJWTSecret), time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func TestAdminRoutes_AuthenticationRequired(t *testing.T) {
	r := setupAdminRouter(&mockAdminRepo{})

	endpoints := []string{
		"/api/v1/admin/overview",
		"/api/v1/admin/users",
		"/api/v1/admin/users/11111111-1111-1111-1111-111111111111",
		"/api/v1/admin/matches",
		"/api/v1/admin/games",
		"/api/v1/admin/live",
		"/api/v1/admin/deposits",
		"/api/v1/admin/withdrawals",
	}

	for _, ep := range endpoints {
		t.Run("NoAuth_"+ep, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, ep, nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized for %s, got %d", ep, w.Code)
			}
		})
	}
}

func TestAdminRoutes_NonAdminForbidden(t *testing.T) {
	r := setupAdminRouter(&mockAdminRepo{})
	userToken := makeToken(t, "user")

	endpoints := []string{
		"/api/v1/admin/overview",
		"/api/v1/admin/users",
		"/api/v1/admin/users/11111111-1111-1111-1111-111111111111",
		"/api/v1/admin/matches",
		"/api/v1/admin/games",
		"/api/v1/admin/live",
		"/api/v1/admin/deposits",
		"/api/v1/admin/withdrawals",
	}

	for _, ep := range endpoints {
		t.Run("NonAdmin_"+ep, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for non-admin on %s, got %d", ep, w.Code)
			}
		})
	}
}

func TestAdminRoutes_AdminAuthorized(t *testing.T) {
	r := setupAdminRouter(&mockAdminRepo{})
	adminToken := makeToken(t, "admin")

	endpoints := []struct {
		name string
		path string
	}{
		{"Overview", "/api/v1/admin/overview"},
		{"Users", "/api/v1/admin/users?limit=10&offset=0"},
		{"UserDetail", "/api/v1/admin/users/11111111-1111-1111-1111-111111111111"},
		{"Matches", "/api/v1/admin/matches?limit=10&offset=0"},
		{"Games", "/api/v1/admin/games"},
		{"Live", "/api/v1/admin/live"},
		{"Deposits", "/api/v1/admin/deposits?limit=10&offset=0"},
		{"Withdrawals", "/api/v1/admin/withdrawals?limit=10&offset=0"},
	}

	for _, ep := range endpoints {
		t.Run("Admin_"+ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 OK for admin on %s, got %d: %s", ep.path, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode JSON response for %s: %v", ep.path, err)
			}
			if resp["code"] != "success" {
				t.Errorf("expected code=success, got %v", resp["code"])
			}
			if resp["data"] == nil {
				t.Errorf("expected non-nil data for %s", ep.path)
			}
		})
	}
}

func TestAdminRoutes_DateFilterValidation(t *testing.T) {
	r := setupAdminRouter(&mockAdminRepo{})
	adminToken := makeToken(t, "admin")

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "InvalidFromFormat",
			query:      "?from=invalid-date",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "from must use YYYY-MM-DD",
		},
		{
			name:       "InvalidToFormat",
			query:      "?from=2026-11-01&to=invalid-date",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "to must use YYYY-MM-DD",
		},
		{
			name:       "FromAfterTo",
			query:      "?from=2026-11-10&to=2026-11-01",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "from must be before to",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/overview"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminRoutes_UserDetailValidation(t *testing.T) {
	r := setupAdminRouter(&mockAdminRepo{})
	adminToken := makeToken(t, "admin")

	t.Run("InvalidUUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users/not-a-valid-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for malformed UUID, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("NotFoundUser", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users/00000000-0000-0000-0000-000000000000", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
	})
}
