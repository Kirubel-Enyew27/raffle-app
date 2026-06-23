package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/audit/application"
)

type AuditMiddleware struct {
	auditService *application.AuditService
}

func NewAuditMiddleware(auditService *application.AuditService) *AuditMiddleware {
	return &AuditMiddleware{auditService: auditService}
}

// Middleware records an audit log entry after each request completes.
// It only logs mutating methods (POST/PUT/PATCH/DELETE) and skips
// read-only GET requests to avoid audit log noise.
func (m *AuditMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // execute handler first

		method := c.Request.Method
		if method == "GET" {
			return
		}

		status := c.Writer.Status()
		// Only audit successful mutations (2xx).
		if status < 200 || status >= 300 {
			return
		}

		actorID := c.GetString("user_id")
		actorType := "anonymous"
		if actorID != "" {
			actorType = "user"
			if c.GetString("role") == "admin" {
				actorType = "admin"
			}
		}

		resourceType := detectResourceType(c.Request.URL.Path)
		resourceID := extractResourceID(c.FullPath()) // use param pattern, not raw path
		action := detectAction(method, resourceType)

		detail := fmt.Sprintf("%s %s → %d", method, c.Request.URL.Path, status)
		_ = m.auditService.Record(
			c.Request.Context(),
			strPtr(actorID), actorType,
			action, resourceType, resourceID,
			c.ClientIP(), nil, &detail,
		)
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func detectResourceType(path string) string {
	switch {
	case strings.Contains(path, "/auth/"):
		return "auth"
	case strings.Contains(path, "/wallets/"):
		return "wallet"
	case strings.Contains(path, "/raffles/"):
		return "raffle"
	case strings.Contains(path, "/tickets/"):
		return "ticket"
	case strings.Contains(path, "/draw/"):
		return "draw"
	case strings.Contains(path, "/winners/"):
		return "winner"
	case strings.Contains(path, "/users/"):
		return "user"
	default:
		return "system"
	}
}

func detectAction(method, resourceType string) string {
	switch method {
	case "POST":
		// Distinguish common sub-resource actions by resource type.
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return "unknown"
	}
}

// extractResourceID pulls the last path segment from a Gin route pattern
// (e.g. "/api/v1/raffles/:id" → ":id").  We log the pattern rather than the
// raw value so callers can correlate logs with the actual param value that
// is already present in new_value.
func extractResourceID(fullPath string) *string {
	parts := strings.Split(strings.TrimRight(fullPath, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && !strings.HasSuffix(parts[i], "paid") {
			id := parts[i]
			return &id
		}
	}
	return nil
}
