package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/wallet/application"
	"github.com/raffle-app/backend/internal/wallet/domain"
)

type stubWalletRepo struct {
	lastTypeQuery string
}

func (s *stubWalletRepo) Create(ctx context.Context, wallet *domain.Wallet) error {
	return nil
}
func (s *stubWalletRepo) CreateTx(ctx context.Context, tx *sql.Tx, wallet *domain.Wallet) error {
	return nil
}
func (s *stubWalletRepo) FindByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	return &domain.Wallet{ID: "w-1", UserID: userID, Balance: 100}, nil
}
func (s *stubWalletRepo) FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*domain.Wallet, error) {
	return &domain.Wallet{ID: "w-1", UserID: userID, Balance: 100}, nil
}
func (s *stubWalletRepo) FindByID(ctx context.Context, id string) (*domain.Wallet, error) {
	return &domain.Wallet{ID: id, Balance: 100}, nil
}
func (s *stubWalletRepo) GetBalance(ctx context.Context, walletID string) (float64, error) {
	return 100, nil
}
func (s *stubWalletRepo) UpdateBalance(ctx context.Context, walletID string, newBalance float64) error {
	return nil
}
func (s *stubWalletRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	return nil
}
func (s *stubWalletRepo) Debit(ctx context.Context, walletID string, amount float64) error {
	return nil
}
func (s *stubWalletRepo) CreateTransaction(ctx context.Context, tx *domain.WalletTransaction) error {
	return nil
}
func (s *stubWalletRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *domain.WalletTransaction) error {
	return nil
}
func (s *stubWalletRepo) FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int, txType string) ([]*domain.WalletTransaction, error) {
	s.lastTypeQuery = txType
	return []*domain.WalletTransaction{
		{ID: "tx-1", WalletID: walletID, Type: "deposit", Status: "completed", Amount: 100},
	}, nil
}
func (s *stubWalletRepo) CountTransactionsByWalletID(ctx context.Context, walletID string, txType string) (int64, error) {
	return 1, nil
}
func (s *stubWalletRepo) FindTransactionsByStatus(ctx context.Context, txType, status string) ([]*domain.WalletTransaction, error) {
	return nil, nil
}
func (s *stubWalletRepo) FindTransactionByID(ctx context.Context, id string) (*domain.WalletTransaction, error) {
	return nil, nil
}
func (s *stubWalletRepo) UpdateTransactionStatus(ctx context.Context, id, status string) error {
	return nil
}

func TestGetTransactionsFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &stubWalletRepo{}
	svc := application.NewWalletService(repo, nil)
	h := NewWalletHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "user-123")
		c.Next()
	})
	r.GET("/transactions", h.GetTransactions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/transactions?type=withdrawal", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if repo.lastTypeQuery != "withdrawal" {
		t.Errorf("expected type filter 'withdrawal' to be passed to repo, got '%s'", repo.lastTypeQuery)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] != "success" {
		t.Errorf("expected success code, got %v", resp["code"])
	}
}
