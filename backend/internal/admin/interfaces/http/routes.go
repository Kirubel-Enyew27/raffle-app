package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterAdminRoutes(r *gin.RouterGroup, handler *AdminHandler) {
	admin := r.Group("/admin")
	admin.Use(middleware.AuthMiddleware(), middleware.AdminMiddleware())
	{
		admin.GET("/overview", handler.Overview)
		admin.GET("/users", handler.Users)
		admin.GET("/users/:id", handler.UserDetail)
		admin.GET("/matches", handler.Matches)
		admin.GET("/games", handler.Games)
		admin.GET("/live", handler.Live)
		admin.GET("/deposits", handler.Deposits)
		admin.GET("/withdrawals", handler.Withdrawals)
	}
}
