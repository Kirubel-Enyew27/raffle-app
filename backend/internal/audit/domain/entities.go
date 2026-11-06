package domain

import (
	"time"
)

type AuditLog struct {
	ID           string    `json:"id"`
	ActorID      *string   `json:"actor_id"`
	ActorType    string    `json:"actor_type"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   *string   `json:"resource_id"`
	OldValue     *string   `json:"old_value"`
	NewValue     *string   `json:"new_value"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    *string   `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
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
