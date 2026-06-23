package middleware

import (
	"context"
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
	return nil, nil
}
func (m *mockAuditRepo) FindByFilter(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLog, int, error) {
	return nil, 0, nil
}
func (m *mockAuditRepo) Count(ctx context.Context, filter domain.AuditLogFilter) (int, error) {
	return 0, nil
}
func (m *mockAuditRepo) DeleteOlderThan(ctx context.Context, cutoffDate time.Time) (int64, error) {
	return 0, nil
}

func newTestRouter(repo *mockAuditRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := application.NewAuditService(repo)
	mw := NewAuditMiddleware(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "user-456")
		c.Set("role", "user")
		c.Next()
	})
	r.Use(mw.Middleware())
	return r
}

func TestAuditMiddleware_RecordsMutation(t *testing.T) {
	repo := &mockAuditRepo{}
	r := newTestRouter(repo)
	r.POST("/api/v1/wallets/deposit", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/wallets/deposit", nil)
	r.ServeHTTP(w, req)

	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(repo.logs))
	}
	l := repo.logs[0]
	if *l.ActorID != "user-456" {
		t.Errorf("expected actor user-456, got %s", *l.ActorID)
	}
	if l.ActorType != "user" {
		t.Errorf("expected actor type user, got %s", l.ActorType)
	}
	if l.Action != "create" {
		t.Errorf("expected action create, got %s", l.Action)
	}
	if l.ResourceType != "wallet" {
		t.Errorf("expected resource type wallet, got %s", l.ResourceType)
	}
}

func TestAuditMiddleware_SkipsGET(t *testing.T) {
	repo := &mockAuditRepo{}
	r := newTestRouter(repo)
	r.GET("/api/v1/raffles/r-1", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/raffles/r-1", nil)
	r.ServeHTTP(w, req)

	if len(repo.logs) != 0 {
		t.Errorf("expected 0 audit logs for GET, got %d", len(repo.logs))
	}
}

func TestAuditMiddleware_SkipsFailedRequests(t *testing.T) {
	repo := &mockAuditRepo{}
	r := newTestRouter(repo)
	r.POST("/api/v1/tickets/buy", func(c *gin.Context) { c.Status(http.StatusBadRequest) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tickets/buy", nil)
	r.ServeHTTP(w, req)

	if len(repo.logs) != 0 {
		t.Errorf("expected 0 audit logs for failed request, got %d", len(repo.logs))
	}
}

func TestAuditMiddleware_AnonymousActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockAuditRepo{}
	svc := application.NewAuditService(repo)
	mw := NewAuditMiddleware(svc)
	r := gin.New()
	r.Use(mw.Middleware())
	r.POST("/api/v1/auth/register", func(c *gin.Context) { c.Status(http.StatusCreated) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", nil)
	r.ServeHTTP(w, req)

	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(repo.logs))
	}
	if repo.logs[0].ActorType != "anonymous" {
		t.Errorf("expected anonymous actor, got %s", repo.logs[0].ActorType)
	}
}

func TestAuditMiddleware_AdminActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockAuditRepo{}
	svc := application.NewAuditService(repo)
	mw := NewAuditMiddleware(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Set("role", "admin")
		c.Next()
	})
	r.Use(mw.Middleware())
	r.POST("/api/v1/raffles/r-1", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/raffles/r-1", nil)
	r.ServeHTTP(w, req)

	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(repo.logs))
	}
	if repo.logs[0].ActorType != "admin" {
		t.Errorf("expected admin actor type, got %s", repo.logs[0].ActorType)
	}
}
