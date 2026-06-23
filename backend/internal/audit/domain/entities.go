package domain

import (
	"time"
)

type AuditLog struct {
	ID            string
	ActorID       *string
	ActorType     string
	Action        string
	ResourceType  string
	ResourceID    *string
	OldValue      *string
	NewValue      *string
	IPAddress     string
	UserAgent     *string
	CreatedAt     time.Time
}

type AuditLogFilter struct {
	ActorID      *string
	ActorType    *string
	Action       *string
	ResourceType *string
	ResourceID   *string
	StartDate    *time.Time
	EndDate      *time.Time
	Limit        int
	Offset       int
}
