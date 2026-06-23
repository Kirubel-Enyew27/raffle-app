package domain

import (
	"context"
	"time"
)

type AuditRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	FindByID(ctx context.Context, id string) (*AuditLog, error)
	FindByFilter(ctx context.Context, filter AuditLogFilter) ([]AuditLog, int, error)
	Count(ctx context.Context, filter AuditLogFilter) (int, error)
	DeleteOlderThan(ctx context.Context, cutoffDate time.Time) (int64, error)
}
