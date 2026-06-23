package application

import (
	"context"
	"time"

	"github.com/raffle-app/backend/internal/audit/domain"
	appcontext "github.com/raffle-app/backend/pkg/context"
)

type AuditService struct {
	auditRepo domain.AuditRepository
}

func NewAuditService(auditRepo domain.AuditRepository) *AuditService {
	return &AuditService{auditRepo: auditRepo}
}

func (s *AuditService) Record(ctx context.Context, actorID *string, actorType, action, resourceType string, resourceID *string, ipAddress string, oldValue, newValue *string) error {
	if ipAddress == "" {
		ipAddress = appcontext.GetIPAddress(ctx)
	}
	var userAgent *string
	ua := appcontext.GetUserAgent(ctx)
	if ua != "" {
		userAgent = &ua
	}

	log := &domain.AuditLog{
		ActorID:       actorID,
		ActorType:     actorType,
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		OldValue:      oldValue,
		NewValue:      newValue,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		CreatedAt:     time.Now(),
	}
	return s.auditRepo.Create(ctx, log)
}

func (s *AuditService) GetAuditLogs(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLog, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.auditRepo.FindByFilter(ctx, filter)
}

func (s *AuditService) GetAuditLogByID(ctx context.Context, id string) (*domain.AuditLog, error) {
	return s.auditRepo.FindByID(ctx, id)
}

func (s *AuditService) Count(ctx context.Context, filter domain.AuditLogFilter) (int, error) {
	return s.auditRepo.Count(ctx, filter)
}

func (s *AuditService) CleanupOldLogs(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return s.auditRepo.DeleteOlderThan(ctx, cutoff)
}
