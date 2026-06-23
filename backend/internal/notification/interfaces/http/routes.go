package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/notifications", middleware.AuthMiddleware())
	{
		g.GET("",          h.List)     // GET  /notifications?limit=&offset=
		g.GET("/unread",   h.Unread)   // GET  /notifications/unread
		g.POST("/:id/read", h.MarkRead) // POST /notifications/:id/read
	}
}
