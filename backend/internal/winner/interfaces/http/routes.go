package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterWinnerRoutes(r *gin.RouterGroup, handler *WinnerHandler) {
	winners := r.Group("/winners")
	{
		winners.GET("", middleware.AuthMiddleware(), handler.ListWinners)
		winners.GET("/raffle/:raffle_id", middleware.AuthMiddleware(), handler.GetWinnersByRaffle)
		winners.GET("/:id", middleware.AuthMiddleware(), handler.GetWinnerDetail)
		winners.GET("/:id/ticket", middleware.AuthMiddleware(), handler.GetWinningTicket)
		winners.GET("/:id/verification", middleware.AuthMiddleware(), handler.GetDrawVerification)
		winners.POST("/:id/paid", middleware.AuthMiddleware(), handler.MarkPrizePaid)
	}
}
