package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterDrawRoutes(r *gin.RouterGroup, handler *DrawHandler) {
	draws := r.Group("/draw")
	{
		draws.POST("/commit/:raffle_id", middleware.AuthMiddleware(), handler.CommitDrawSeed)
		draws.POST("/execute/:raffle_id", middleware.AuthMiddleware(), handler.ExecuteDraw)
		draws.GET("/:raffle_id/result", handler.GetDrawResult)
		draws.POST("/verify", handler.VerifyDraw)
	}
}
