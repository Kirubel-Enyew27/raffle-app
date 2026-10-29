package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

// RegisterP2PRoutes registers all P2P routes.
// rl may be nil — when nil, rate limiting is skipped (e.g. in tests).
func RegisterP2PRoutes(r *gin.RouterGroup, handler *P2PHandler, rl ...*P2PRateLimiter) {
	var rateLimiter *P2PRateLimiter
	if len(rl) > 0 {
		rateLimiter = rl[0]
	}

	p2p := r.Group("/p2p")
	{
		// Public game list, leaderboard and invite preview
		p2p.GET("/games", handler.ListGames)
		p2p.GET("/games/:slug", handler.GetGame)
		p2p.GET("/leaderboard", middleware.OptionalAuthMiddleware(), handler.GetLeaderboard)
		p2p.GET("/invites/:code", handler.GetInvite)

		// Protected matchmaking and match actions
		auth := p2p.Group("")
		auth.Use(middleware.AuthMiddleware())
		{
			auth.GET("/matches/lobby", handler.ListLobby)
			auth.GET("/matches/my", handler.ListUserMatches)

			// Match creation: limited to 10 per 60s per user
			if rateLimiter != nil {
				auth.POST("/matches", rateLimiter.CreateRateLimit(), handler.CreateMatch)
			} else {
				auth.POST("/matches", handler.CreateMatch)
			}

			// Join actions: limited to 20 per 60s per user
			if rateLimiter != nil {
				auth.POST("/matches/join-code", rateLimiter.JoinRateLimit(), handler.JoinMatchByCode)
				auth.POST("/matches/:id/join", rateLimiter.JoinRateLimit(), handler.JoinMatch)
				auth.POST("/matches/:id/ready", rateLimiter.JoinRateLimit(), handler.ReadyUp)
			} else {
				auth.POST("/matches/join-code", handler.JoinMatchByCode)
				auth.POST("/matches/:id/join", handler.JoinMatch)
				auth.POST("/matches/:id/ready", handler.ReadyUp)
			}

			auth.POST("/matches/:id/cancel", handler.CancelMatch)

			// Move submission: limited to 30 per 60s per user
			if rateLimiter != nil {
				auth.POST("/matches/:id/move", rateLimiter.MoveRateLimit(), handler.SubmitMove)
			} else {
				auth.POST("/matches/:id/move", handler.SubmitMove)
			}

			auth.POST("/matches/:id/resign", handler.ResignMatch)
			auth.POST("/matches/:id/rematch", handler.Rematch)
			auth.GET("/matches/:id/state", handler.GetMatchState)
			auth.GET("/matches/:id", handler.GetMatchState)
		}
	}
}
