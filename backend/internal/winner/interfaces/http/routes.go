package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterWinnerRoutes(r *gin.RouterGroup, handler *WinnerHandler) {
	winners := r.Group("/winners")
	{
		winners.GET("/raffle/:raffle_id", middleware.AuthMiddleware(), handler.GetWinnersByRaffle)
		winners.GET("/:id", middleware.AuthMiddleware(), handler.GetWinnerDetail)
		winners.POST("/:id/paid", middleware.AuthMiddleware(), handler.MarkPrizePaid)
	}
}
