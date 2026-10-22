package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

// RegisterCrashRoutes mounts the crash game API. Every endpoint requires an
// authenticated user; the game never trusts client-supplied balances or
// multipliers.
func RegisterCrashRoutes(r *gin.RouterGroup, handler *CrashHandler) {
	crash := r.Group("/crash", middleware.AuthMiddleware())
	{
		// Player endpoints.
		crash.GET("/state", handler.GetState)
		crash.GET("/history", handler.GetHistory)
		crash.GET("/rounds/:id/bets", handler.GetRoundBets)
		crash.GET("/rounds/:id/verify", handler.VerifyRound)
		crash.GET("/my-bets", handler.GetMyBets)
		crash.POST("/bets", handler.PlaceBet)
		crash.POST("/bets/cashout", handler.CashOut)

		// Admin-only settings endpoints (guarded by AdminMiddleware).
		admin := crash.Group("/admin", middleware.AdminMiddleware())
		{
			admin.GET("/settings", handler.GetSettings)
			admin.PUT("/settings/:key", handler.UpdateSetting)
		}
	}
}
