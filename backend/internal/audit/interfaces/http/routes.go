package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterAuditRoutes(r *gin.RouterGroup, handler *AuditHandler) {
	audit := r.Group("/audit")
	{
		audit.GET("/logs", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handler.GetAuditLogs)
		audit.GET("/logs/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handler.GetAuditLog)
	}
}
