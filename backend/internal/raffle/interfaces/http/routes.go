package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterRaffleRoutes(r *gin.RouterGroup, handler *RaffleHandler) {
	raffles := r.Group("/raffles")
	{
		raffles.GET("", handler.ListRaffles)
		raffles.GET("/:id", handler.GetRaffle)
		raffles.POST("", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handler.CreateRaffle)
		raffles.PUT("/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handler.UpdateRaffle)
		raffles.POST("/:id/close", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handler.CloseRaffle)
		raffles.POST("/:id/schedule-draw", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handler.ScheduleDrawDate)
	}
}
