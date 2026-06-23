package context

import (
	"context"
)

type contextKey string

const (
	IPAddressKey contextKey = "ip_address"
	UserAgentKey contextKey = "user_agent"
	UserIDKey    contextKey = "user_id"
	UserRoleKey  contextKey = "user_role"
)

func WithAuditContext(ctx context.Context, ipAddress, userAgent string) context.Context {
	ctx = context.WithValue(ctx, IPAddressKey, ipAddress)
	return context.WithValue(ctx, UserAgentKey, userAgent)
}

func WithUserContext(ctx context.Context, userID, role string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return context.WithValue(ctx, UserRoleKey, role)
}

func GetIPAddress(ctx context.Context) string {
	if val, ok := ctx.Value(IPAddressKey).(string); ok {
		return val
	}
	return ""
}

func GetUserAgent(ctx context.Context) string {
	if val, ok := ctx.Value(UserAgentKey).(string); ok {
		return val
	}
	return ""
}

func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	return ""
}

func GetUserRole(ctx context.Context) string {
	if val, ok := ctx.Value(UserRoleKey).(string); ok {
		return val
	}
	return ""
}
