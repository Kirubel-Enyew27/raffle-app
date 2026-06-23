package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/audit/domain"
)

type mockAuditRepo struct {
	logs []domain.AuditLog
}

func (m *mockAuditRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	log.ID = "log-1"
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) FindByID(ctx context.Context, id string) (*domain.AuditLog, error) {
	for _, log := range m.logs {
		if log.ID == id {
			return &log, nil
		}
	}
	return nil, errors.New("log not found")
}

func (m *mockAuditRepo) FindByFilter(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLog, int, error) {
	return m.logs, len(m.logs), nil
}

func (m *mockAuditRepo) Count(ctx context.Context, filter domain.AuditLogFilter) (int, error) {
	return len(m.logs), nil
}

func (m *mockAuditRepo) DeleteOlderThan(ctx context.Context, cutoffDate time.Time) (int64, error) {
	return 0, nil
}

func TestGetAuditLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockAuditRepo{}
	svc := application.NewAuditService(repo)
	handler := NewAuditHandler(svc)

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Set("role", "admin")
		c.Next()
	})

	actorID := "user-123"
	_ = svc.Record(context.Background(), &actorID, "user", "login", "auth", nil, "127.0.0.1", nil, nil)

	r.GET("/audit/logs", handler.GetAuditLogs)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/audit/logs?limit=10&offset=0", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}

	code := resp["code"].(string)
	if code != "SUCCESS" {
		t.Errorf("expected code SUCCESS, got %s", code)
	}
}

func TestGetAuditLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockAuditRepo{}
	svc := application.NewAuditService(repo)
	handler := NewAuditHandler(svc)

	r := gin.Default()

	actorID := "user-123"
	_ = svc.Record(context.Background(), &actorID, "user", "login", "auth", nil, "127.0.0.1", nil, nil)

	r.GET("/audit/logs/:id", handler.GetAuditLog)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/audit/logs/log-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}

	code := resp["code"].(string)
	if code != "SUCCESS" {
		t.Errorf("expected code SUCCESS, got %s", code)
	}
}
