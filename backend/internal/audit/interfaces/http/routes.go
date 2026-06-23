package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterAuditRoutes(r *gin.RouterGroup, handler *AuditHandler) {
	audit := r.Group("/audit")
	{
		audit.GET("/logs", middleware.AuthMiddleware(), handler.GetAuditLogs)
		audit.GET("/logs/:id", middleware.AuthMiddleware(), handler.GetAuditLog)
	}
}
