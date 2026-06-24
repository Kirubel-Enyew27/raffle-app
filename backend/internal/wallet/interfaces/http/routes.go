package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterWalletRoutes(r *gin.RouterGroup, handler *WalletHandler) {
	wallets := r.Group("/wallets", middleware.AuthMiddleware())
	{
		wallets.GET("/balance", handler.GetBalance)
		wallets.GET("/transactions", handler.GetTransactions)
		wallets.POST("/deposit", handler.Deposit)
		wallets.POST("/withdraw", handler.Withdraw)
	}
}
