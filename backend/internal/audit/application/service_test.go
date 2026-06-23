package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/audit/domain"
)

type mockAuditRepo struct {
	logs []domain.AuditLog
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{logs: []domain.AuditLog{}}
}

func (m *mockAuditRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	log.ID = "audit-" + time.Now().Format("20060102150405")
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) FindByID(ctx context.Context, id string) (*domain.AuditLog, error) {
	for i := range m.logs {
		if m.logs[i].ID == id {
			return &m.logs[i], nil
		}
	}
	return nil, errors.New("audit log not found")
}

func (m *mockAuditRepo) FindByFilter(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLog, int, error) {
	var result []domain.AuditLog
	for _, log := range m.logs {
		match := true
		if filter.ActorID != nil && log.ActorID != nil && *log.ActorID != *filter.ActorID {
			match = false
		}
		if filter.Action != nil && log.Action != *filter.Action {
			match = false
		}
		if filter.ResourceType != nil && log.ResourceType != *filter.ResourceType {
			match = false
		}
		if match {
			result = append(result, log)
		}
	}
	return result, len(result), nil
}

func (m *mockAuditRepo) Count(ctx context.Context, filter domain.AuditLogFilter) (int, error) {
	logs, _, err := m.FindByFilter(ctx, filter)
	return len(logs), err
}

func (m *mockAuditRepo) DeleteOlderThan(ctx context.Context, cutoffDate time.Time) (int64, error) {
	var remaining []domain.AuditLog
	deleted := int64(0)
	for _, log := range m.logs {
		if log.CreatedAt.Before(cutoffDate) {
			deleted++
		} else {
			remaining = append(remaining, log)
		}
	}
	m.logs = remaining
	return deleted, nil
}

func TestRecordAuditLog(t *testing.T) {
	repo := newMockAuditRepo()
	svc := NewAuditService(repo)

	actorID := "user-1"
	err := svc.Record(
		context.Background(),
		&actorID, "user", "login", "auth", nil,
		"192.168.1.1", nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.logs))
	}
	log := repo.logs[0]
	if log.Action != "login" {
		t.Errorf("expected action 'login', got %s", log.Action)
	}
	if log.ResourceType != "auth" {
		t.Errorf("expected resource_type 'auth', got %s", log.ResourceType)
	}
	if *log.ActorID != "user-1" {
		t.Errorf("expected actor_id 'user-1', got %s", *log.ActorID)
	}
}

func TestRecordAuditLogWithValues(t *testing.T) {
	repo := newMockAuditRepo()
	svc := NewAuditService(repo)

	oldVal := "scheduled"
	newVal := "active"
	resourceID := "raffle-123"

	err := svc.Record(
		context.Background(),
		&[]string{"admin-1"}[0], "admin", "update",
		"raffle", &resourceID, "10.0.0.1",
		&oldVal, &newVal,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.logs))
	}
	log := repo.logs[0]
	if *log.OldValue != "scheduled" {
		t.Errorf("expected old_value 'scheduled', got %s", *log.OldValue)
	}
	if *log.NewValue != "active" {
		t.Errorf("expected new_value 'active', got %s", *log.NewValue)
	}
}

func TestGetAuditLogsByFilter(t *testing.T) {
	repo := newMockAuditRepo()
	svc := NewAuditService(repo)

	svc.Record(context.Background(), &[]string{"user-1"}[0], "user", "login", "auth", nil, "192.168.1.1", nil, nil)
	svc.Record(context.Background(), &[]string{"user-1"}[0], "user", "purchase", "ticket", nil, "192.168.1.1", nil, nil)
	svc.Record(context.Background(), &[]string{"admin-1"}[0], "admin", "update", "raffle", nil, "10.0.0.1", nil, nil)

	logs, total, err := svc.GetAuditLogs(context.Background(), domain.AuditLogFilter{
		ActorID: &[]string{"user-1"}[0],
		Limit:   10,
		Offset:  0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestCleanupOldLogs(t *testing.T) {
	repo := newMockAuditRepo()
	svc := NewAuditService(repo)

	svc.Record(context.Background(), nil, "system", "cleanup", "system", nil, "127.0.0.1", nil, nil)
	svc.Record(context.Background(), nil, "system", "cleanup", "system", nil, "127.0.0.1", nil, nil)

	// Make the first log 2 days old so it is before the 1-day retention cutoff
	repo.logs[0].CreatedAt = time.Now().AddDate(0, 0, -2)
	cutoff := time.Now().AddDate(0, 0, -1)

	deleted, err := svc.CleanupOldLogs(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted log, got %d", deleted)
	}
	if len(repo.logs) != 1 {
		t.Errorf("expected 1 remaining log, got %d", len(repo.logs))
	}
	if repo.logs[0].CreatedAt.Before(cutoff) {
		t.Error("remaining log should be after cutoff")
	}
}

func TestGetAuditLogByID(t *testing.T) {
	repo := newMockAuditRepo()
	svc := NewAuditService(repo)

	svc.Record(context.Background(), &[]string{"user-1"}[0], "user", "login", "auth", nil, "192.168.1.1", nil, nil)

	logs, _, _ := svc.GetAuditLogs(context.Background(), domain.AuditLogFilter{ActorID: &[]string{"user-1"}[0]})
	if len(logs) == 0 {
		t.Fatal("expected at least one log")
	}

	found, err := svc.GetAuditLogByID(context.Background(), logs[0].ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.Action != "login" {
		t.Errorf("expected action 'login', got %s", found.Action)
	}
}
