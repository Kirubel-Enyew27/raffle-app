package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

// RegisterGoogleAuthRoutes mounts the Google OAuth endpoints next to the
// existing auth routes. `limiter` is optional; when supplied it throttles the
// entry points that create server-side state.
func RegisterGoogleAuthRoutes(r *gin.RouterGroup, handler *GoogleAuthHandler, limiter gin.HandlerFunc) {
	auth := r.Group("/auth")
	{
		auth.GET("/providers", handler.Providers)
		auth.GET("/google/callback", handler.Callback)
		// Account linking is only available to a signed-in user.
		auth.GET("/google/link", middleware.AuthMiddleware(), handler.LinkStatus)
		if limiter != nil {
			auth.GET("/google", limiter, handler.Start)
			auth.POST("/google/exchange", limiter, handler.Exchange)
			auth.POST("/google/link", middleware.AuthMiddleware(), limiter, handler.StartLink)
		} else {
			auth.GET("/google", handler.Start)
			auth.POST("/google/exchange", handler.Exchange)
			auth.POST("/google/link", middleware.AuthMiddleware(), handler.StartLink)
		}
	}
}

func RegisterIdentityRoutes(r *gin.RouterGroup, handler *IdentityHandler) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
		auth.POST("/refresh", handler.Refresh)
		auth.POST("/logout", handler.Logout)
		auth.PUT("/profile", middleware.AuthMiddleware(), handler.UpdateProfile)
		auth.POST("/avatar", middleware.AuthMiddleware(), handler.UploadAvatar)
		auth.POST("/change-password", middleware.AuthMiddleware(), handler.ChangePassword)
	}
}
