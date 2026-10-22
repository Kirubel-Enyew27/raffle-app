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
		wallets.POST("/deposits/intent", handler.CreateDepositIntent)
		wallets.GET("/deposits", handler.ListUserDeposits)
		wallets.GET("/deposits/:id", handler.GetDeposit)
		wallets.POST("/deposits/:id/verify", handler.VerifyDeposit)
		wallets.POST("/deposit/verify", handler.VerifyAndDepositReceipt)
		wallets.POST("/deposit/extract", handler.ExtractReceipt)
		wallets.POST("/withdraw", handler.Withdraw)
	}

	// Admin routes for withdrawal management
	admin := r.Group("/admin/wallets", middleware.AuthMiddleware())
	{
		admin.GET("/pending-withdrawals", handler.ListPendingWithdrawals)
		admin.POST("/approve-withdrawal/:id", handler.ApproveWithdrawal)
		admin.POST("/reject-withdrawal/:id", handler.RejectWithdrawal)
	}
}
