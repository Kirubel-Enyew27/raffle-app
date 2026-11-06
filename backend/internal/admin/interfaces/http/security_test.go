package http

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	auditapp "github.com/raffle-app/backend/internal/audit/application"
	auditdomain "github.com/raffle-app/backend/internal/audit/domain"
	audithttp "github.com/raffle-app/backend/internal/audit/interfaces/http"
	drawhttp "github.com/raffle-app/backend/internal/draw/interfaces/http"
	rafflehttp "github.com/raffle-app/backend/internal/raffle/interfaces/http"
	reportapp "github.com/raffle-app/backend/internal/reporting/application"
	reportdomain "github.com/raffle-app/backend/internal/reporting/domain"
	reporthttp "github.com/raffle-app/backend/internal/reporting/interfaces/http"
	tickethttp "github.com/raffle-app/backend/internal/ticket/interfaces/http"
	walletapp "github.com/raffle-app/backend/internal/wallet/application"
	walletdomain "github.com/raffle-app/backend/internal/wallet/domain"
	wallethttp "github.com/raffle-app/backend/internal/wallet/interfaces/http"
	winnerhttp "github.com/raffle-app/backend/internal/winner/interfaces/http"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"github.com/spf13/viper"
)

func setupCrossContextSecurityRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	viper.Set("JWT_SECRET", testJWTSecret)

	r := gin.New()
	api := r.Group("/api/v1")

	// Register audit routes
	auditSvc := auditapp.NewAuditService(&mockAuditRepo{})
	audithttp.RegisterAuditRoutes(api, audithttp.NewAuditHandler(auditSvc))

	// Register reporting routes
	reportSvc := reportapp.NewReportService(&mockReportRepo{})
	reporthttp.RegisterReportRoutes(api, reporthttp.NewReportHandler(reportSvc))

	// Register wallet admin routes
	walletSvc := walletapp.NewWalletService(&mockWalletRepo{}, nil)
	wallethttp.RegisterWalletRoutes(api, wallethttp.NewWalletHandler(walletSvc))

	// Register raffle admin routes
	rafflehttp.RegisterRaffleRoutes(api, rafflehttp.NewRaffleHandler(nil))

	// Register draw admin routes
	drawhttp.RegisterDrawRoutes(api, drawhttp.NewDrawHandler(nil))

	// Register ticket admin routes
	tickethttp.RegisterTicketRoutes(api, tickethttp.NewTicketHandler(nil))

	// Register winner admin routes
	winnerhttp.RegisterWinnerRoutes(api, winnerhttp.NewWinnerHandler(nil))

	return r
}

type mockAuditRepo struct{}

func (m *mockAuditRepo) Create(ctx context.Context, log *auditdomain.AuditLog) error { return nil }
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

type mockReportRepo struct{}

func (m *mockReportRepo) Revenue(ctx context.Context, period reportdomain.Period, f reportdomain.Filter) ([]reportdomain.RevenueRow, int, error) {
	return []reportdomain.RevenueRow{{Period: "2026-11-01", TicketRevenue: 100}}, 1, nil
}
func (m *mockReportRepo) TicketSales(ctx context.Context, period reportdomain.Period, f reportdomain.Filter) ([]reportdomain.TicketSalesRow, int, error) {
	return []reportdomain.TicketSalesRow{{Period: "2026-11-01", TicketsSold: 10}}, 1, nil
}
func (m *mockReportRepo) ActiveUsers(ctx context.Context, period reportdomain.Period, f reportdomain.Filter) ([]reportdomain.ActiveUsersRow, int, error) {
	return []reportdomain.ActiveUsersRow{{Period: "2026-11-01", ActiveUsers: 5}}, 1, nil
}
func (m *mockReportRepo) WinnerSummary(ctx context.Context, f reportdomain.Filter) ([]reportdomain.WinnerSummaryRow, int, error) {
	return []reportdomain.WinnerSummaryRow{{WinnerID: "win-1"}}, 1, nil
}
func (m *mockReportRepo) ProfitSummary(ctx context.Context, f reportdomain.Filter) (*reportdomain.ProfitSummary, error) {
	return &reportdomain.ProfitSummary{NetProfit: 500}, nil
}

type mockWalletRepo struct{}

func (m *mockWalletRepo) Create(ctx context.Context, wallet *walletdomain.Wallet) error { return nil }
func (m *mockWalletRepo) CreateTx(ctx context.Context, tx *sql.Tx, wallet *walletdomain.Wallet) error {
	return nil
}
func (m *mockWalletRepo) FindByUserID(ctx context.Context, userID string) (*walletdomain.Wallet, error) {
	return nil, nil
}
func (m *mockWalletRepo) FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*walletdomain.Wallet, error) {
	return nil, nil
}
func (m *mockWalletRepo) FindByID(ctx context.Context, id string) (*walletdomain.Wallet, error) {
	return nil, nil
}
func (m *mockWalletRepo) GetBalance(ctx context.Context, walletID string) (float64, error) {
	return 0, nil
}
func (m *mockWalletRepo) UpdateBalance(ctx context.Context, walletID string, newBalance float64) error {
	return nil
}
func (m *mockWalletRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	return nil
}
func (m *mockWalletRepo) Debit(ctx context.Context, walletID string, amount float64) error { return nil }
func (m *mockWalletRepo) CreateTransaction(ctx context.Context, tx *walletdomain.WalletTransaction) error {
	return nil
}
func (m *mockWalletRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *walletdomain.WalletTransaction) error {
	return nil
}
func (m *mockWalletRepo) FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int, txType string) ([]*walletdomain.WalletTransaction, error) {
	return nil, nil
}
func (m *mockWalletRepo) CountTransactionsByWalletID(ctx context.Context, walletID string, txType string) (int64, error) {
	return 0, nil
}
func (m *mockWalletRepo) FindTransactionsByStatus(ctx context.Context, txType, status string) ([]*walletdomain.WalletTransaction, error) {
	return nil, nil
}
func (m *mockWalletRepo) FindTransactionByID(ctx context.Context, id string) (*walletdomain.WalletTransaction, error) {
	return nil, nil
}
func (m *mockWalletRepo) UpdateTransactionStatus(ctx context.Context, id, status string) error {
	return nil
}
func (m *mockWalletRepo) ApproveWithdrawal(ctx context.Context, id string) (*walletdomain.WalletTransaction, *walletdomain.Wallet, error) {
	return &walletdomain.WalletTransaction{ID: id, Status: "completed"}, &walletdomain.Wallet{ID: "w-1", Balance: 100}, nil
}
func (m *mockWalletRepo) RejectWithdrawal(ctx context.Context, id string) (*walletdomain.WalletTransaction, error) {
	return &walletdomain.WalletTransaction{ID: id, Status: "rejected"}, nil
}
func (m *mockWalletRepo) ExistsByReference(ctx context.Context, reference string) (bool, error) {
	return false, nil
}

func TestCrossContext_AdminGuardsBlockNonAdmins(t *testing.T) {
	r := setupCrossContextSecurityRouter()
	userToken, err := appjwt.GenerateTokenHMAC("regular-user-id", "user@example.com", "user", []byte(testJWTSecret), time.Hour)
	if err != nil {
		t.Fatalf("failed to generate user token: %v", err)
	}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		// Reports
		{"GetRevenueReport", http.MethodGet, "/api/v1/reports/revenue"},
		{"GetProfitSummaryReport", http.MethodGet, "/api/v1/reports/profit"},
		{"GetTicketSalesReport", http.MethodGet, "/api/v1/reports/tickets"},
		{"GetActiveUsersReport", http.MethodGet, "/api/v1/reports/active-users"},
		{"GetWinnersReport", http.MethodGet, "/api/v1/reports/winners"},

		// Audit
		{"GetAuditLogs", http.MethodGet, "/api/v1/audit/logs"},
		{"GetAuditLogByID", http.MethodGet, "/api/v1/audit/logs/some-id"},

		// Wallet admin
		{"ListPendingWithdrawals", http.MethodGet, "/api/v1/admin/wallets/pending-withdrawals"},
		{"ApproveWithdrawal", http.MethodPost, "/api/v1/admin/wallets/approve-withdrawal/11111111-1111-1111-1111-111111111111"},
		{"RejectWithdrawal", http.MethodPost, "/api/v1/admin/wallets/reject-withdrawal/11111111-1111-1111-1111-111111111111"},

		// Raffle admin
		{"CreateRaffle", http.MethodPost, "/api/v1/raffles"},
		{"UpdateRaffle", http.MethodPut, "/api/v1/raffles/11111111-1111-1111-1111-111111111111"},
		{"CloseRaffle", http.MethodPost, "/api/v1/raffles/11111111-1111-1111-1111-111111111111/close"},
		{"ScheduleDraw", http.MethodPost, "/api/v1/raffles/11111111-1111-1111-1111-111111111111/schedule-draw"},

		// Draw admin
		{"CommitDrawSeed", http.MethodPost, "/api/v1/draw/commit/11111111-1111-1111-1111-111111111111"},
		{"ExecuteDraw", http.MethodPost, "/api/v1/draw/execute/11111111-1111-1111-1111-111111111111"},

		// Tickets admin
		{"ListRaffleTickets", http.MethodGet, "/api/v1/raffles/11111111-1111-1111-1111-111111111111/tickets"},

		// Winners admin
		{"MarkPrizePaid", http.MethodPost, "/api/v1/winners/11111111-1111-1111-1111-111111111111/paid"},
	}

	for _, tc := range tests {
		t.Run("NonAdminForbidden_"+tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden for non-admin on %s %s, got %d", tc.method, tc.path, w.Code)
			}
		})
	}
}

func TestWalletAdmin_InvalidUUIDValidation(t *testing.T) {
	r := setupCrossContextSecurityRouter()
	adminToken, err := appjwt.GenerateTokenHMAC("admin-user-id", "admin@example.com", "admin", []byte(testJWTSecret), time.Hour)
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	t.Run("ApproveInvalidUUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/wallets/approve-withdrawal/not-a-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for malformed txID on approve, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("RejectInvalidUUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/wallets/reject-withdrawal/not-a-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for malformed txID on reject, got %d: %s", w.Code, w.Body.String())
		}
	})
}
