package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterP2PRoutes(r *gin.RouterGroup, handler *P2PHandler) {
	p2p := r.Group("/p2p")
	{
		// Public game list and leaderboard
		p2p.GET("/games", handler.ListGames)
		p2p.GET("/games/:slug", handler.GetGame)
		p2p.GET("/leaderboard", middleware.OptionalAuthMiddleware(), handler.GetLeaderboard)

		// Protected matchmaking and match actions
		auth := p2p.Group("")
		auth.Use(middleware.AuthMiddleware())
		{
			auth.GET("/matches/lobby", handler.ListLobby)
			auth.GET("/matches/my", handler.ListUserMatches)
			auth.POST("/matches", handler.CreateMatch)
			auth.POST("/matches/join-code", handler.JoinMatchByCode)
			auth.POST("/matches/:id/join", handler.JoinMatch)
			auth.POST("/matches/:id/ready", handler.ReadyUp)
			auth.POST("/matches/:id/move", handler.SubmitMove)
			auth.POST("/matches/:id/resign", handler.ResignMatch)
			auth.POST("/matches/:id/rematch", handler.Rematch)
			auth.GET("/matches/:id/state", handler.GetMatchState)
		}
	}
}
