package http

import "github.com/gin-gonic/gin"

func RegisterWalletRoutes(r *gin.RouterGroup, handler *WalletHandler) {
	wallets := r.Group("/wallets")
	{
		wallets.GET("/balance", handler.GetBalance)
		wallets.GET("/transactions", handler.GetTransactions)
		wallets.POST("/deposit", handler.Deposit)
		wallets.POST("/withdraw", handler.Withdraw)
	}
}
