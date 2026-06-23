package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterTicketRoutes(r *gin.RouterGroup, handler *TicketHandler) {
	tickets := r.Group("/tickets")
	{
		tickets.POST("/purchase", middleware.AuthMiddleware(), handler.PurchaseTickets)
	}
}
